# Arquitetura — Sistema de Emissão de Notas Fiscais

Contrato entre os serviços. Escrito antes do código, de propósito: os dois serviços são
implementados em momentos diferentes e precisam concordar sem depender de memória.

## Visão geral

| Componente | Stack | Porta |
|---|---|---|
| `web` | Angular + Angular Material | 4200 |
| `faturamento` | Go + Gin + GORM | 7081 |
| `estoque` | Go + Gin + GORM | 7080 |
| `postgres` | PostgreSQL 16 | 5442 no host, 5432 na rede do Docker |

O Angular fala com os dois serviços. O `faturamento` fala com o `estoque` por HTTP.
O `estoque` nunca chama o `faturamento` — a dependência é de mão única.

```
  Angular ──┬──────────────► estoque    (produtos)
            │                   ▲
            └──► faturamento ───┘        (notas; chama o estoque na impressão)
```

## Bancos

Um container Postgres com **dois databases**: `estoque_db` e `faturamento_db`.
Cada serviço só recebe a credencial do seu. **Nenhum JOIN entre os dois é possível** —
que é a regra que importa num microsserviço. Um container só porque em 5 dias o
isolamento físico não paga o custo de RAM.

Consequência prática: o Faturamento **não pode** fazer join para descobrir a descrição de
um produto. Por isso ele guarda um *snapshot* de `codigo` e `descricao` no item da nota,
copiado no momento em que o produto é adicionado. Isso também está correto no domínio —
uma nota fiscal registra o que foi vendido *naquele momento*, mesmo que o cadastro do
produto mude depois.

### `estoque_db`

```
produtos
  id            bigserial PK
  codigo        text UNIQUE NOT NULL
  descricao     text NOT NULL
  saldo         integer NOT NULL CHECK (saldo >= 0)
  created_at, updated_at

baixas                        -- registro de idempotência
  id            bigserial PK
  chave         text UNIQUE NOT NULL     -- valor do header Idempotency-Key
  status        text NOT NULL            -- CONFIRMADA | RECUSADA
  resposta      jsonb NOT NULL           -- corpo devolvido na primeira vez
  created_at
```

### `faturamento_db`

```
notas
  id                bigserial PK
  numero            integer UNIQUE NOT NULL   -- vem de uma SEQUENCE do Postgres
  status            text NOT NULL             -- ABERTA | FECHADA
  idempotency_key   text                      -- gerada e salva ANTES de chamar o estoque
  baixa_confirmada  boolean NOT NULL DEFAULT false
  created_at, updated_at

nota_itens
  id                 bigserial PK
  nota_id            bigint FK → notas
  produto_id         bigint            -- id no estoque, sem FK (bancos separados)
  produto_codigo     text              -- snapshot
  produto_descricao  text              -- snapshot
  quantidade         integer NOT NULL CHECK (quantidade > 0)
```

## Formato de erro (idêntico nos dois serviços)

```json
{
  "erro": {
    "codigo": "SALDO_INSUFICIENTE",
    "mensagem": "Saldo insuficiente para 1 produto",
    "detalhes": [
      { "produto_codigo": "CAD-001", "solicitado": 5, "disponivel": 2 }
    ]
  }
}
```

| HTTP | `codigo` | Quando |
|---|---|---|
| 400 | `PAYLOAD_INVALIDO` | corpo malformado ou campo obrigatório ausente |
| 404 | `NAO_ENCONTRADO` | id inexistente |
| 409 | `SALDO_INSUFICIENTE` | pelo menos um item não tem saldo |
| 409 | `NOTA_NAO_ABERTA` | tentou imprimir nota já `FECHADA` |
| 409 | `NOTA_VAZIA` | tentou imprimir nota sem itens |
| 503 | `ESTOQUE_INDISPONIVEL` | o estoque não respondeu, deu timeout, ou o disjuntor está aberto |
| 500 | `ERRO_INTERNO` | qualquer outra coisa — nunca vaza detalhe interno |

**A distinção que importa:** `409` é o sistema funcionando e dizendo não. `503` é o
sistema quebrado. São mensagens diferentes na tela — a de 409 explica o que fazer
(reduzir a quantidade), a de 503 oferece "Tentar novamente". Essa separação é metade
do requisito obrigatório 2.

## Endpoints — `estoque` (7080)

| Método | Rota | Observação |
|---|---|---|
| GET | `/health` | |
| GET | `/produtos` | lista |
| POST | `/produtos` | `{ codigo, descricao, saldo }` |
| GET | `/produtos/:id` | |
| PUT | `/produtos/:id` | |
| DELETE | `/produtos/:id` | |
| POST | `/baixas` | **exige header `Idempotency-Key`** |

### `POST /baixas`

```
Idempotency-Key: nota-7-a3f9c2b1

{ "itens": [ { "produto_id": 1, "quantidade": 2 },
             { "produto_id": 4, "quantidade": 1 } ] }
```

Quatro respostas possíveis:

1. **200 — baixa efetuada.** Debitou todos os itens.
   ```json
   { "status": "CONFIRMADA", "itens": [ { "produto_id": 1, "saldo_anterior": 10, "saldo_atual": 8 } ] }
   ```
2. **200 — já processada.** A chave já existia: devolve o corpo guardado em `baixas.resposta`,
   byte a byte igual ao da primeira vez. **Não debita de novo.**
3. **409 — saldo insuficiente.** Nada foi debitado (a transação inteira volta atrás).
   Também é gravado em `baixas` com status `RECUSADA` — repetir a chave devolve o mesmo 409.
4. **503 / sem resposta.** O serviço caiu. Nada foi debitado.

Regras internas:
- Tudo dentro de **uma transação**.
- O débito é `UPDATE produtos SET saldo = saldo - $1 WHERE id = $2 AND saldo >= $1`.
  `RowsAffected == 0` significa saldo insuficiente. A verificação e a escrita são o mesmo
  comando — não existe brecha entre ler e gravar, então duas requisições simultâneas
  disputando o mesmo saldo não se atropelam.
- Itens ordenados por `produto_id` antes de debitar, para que duas requisições com os
  mesmos produtos travem sempre na mesma ordem (evita impasse entre transações).
- `singleflight` na chave: se duas requisições com a mesma chave chegarem ao mesmo tempo,
  só uma executa e as duas recebem o mesmo resultado.

## Endpoints — `faturamento` (7081)

| Método | Rota | Observação |
|---|---|---|
| GET | `/health` | |
| GET | `/notas` | lista |
| POST | `/notas` | cria vazia, `ABERTA`, número da sequence |
| GET | `/notas/:id` | com os itens |
| POST | `/notas/:id/itens` | `{ produto_id, quantidade }` — só se `ABERTA` |
| DELETE | `/notas/:id/itens/:item_id` | só se `ABERTA` |
| POST | `/notas/:id/imprimir` | o fluxo abaixo |

### `POST /notas/:id/imprimir`

```
1. Nota está ABERTA?            não → 409 NOTA_NAO_ABERTA
2. Nota tem itens?              não → 409 NOTA_VAZIA
3. Nota já tem idempotency_key? não → gera, SALVA e faz commit
                                       (antes de chamar o estoque — se gerar depois,
                                        cada retry vira uma chave nova e debita de novo)
4. POST estoque/baixas com essa chave, timeout de 3s
     ├─ 200 → marca baixa_confirmada = true, status = FECHADA → 200
     ├─ 409 → repassa o 409 com os detalhes; nota continua ABERTA
     └─ timeout / conexão recusada / disjuntor aberto → 503; nota continua ABERTA
```

**Por que isso é seguro sem transação distribuída:**

| Falha | O que acontece | Como se resolve |
|---|---|---|
| Estoque fora no passo 4 | Nada debitado, nota `ABERTA` | Usuário clica de novo quando voltar |
| Estoque debitou, faturamento morreu antes de fechar | Saldo baixado, nota `ABERTA` com a chave salva | Nova impressão manda a **mesma** chave → estoque devolve o resultado guardado sem redebitar → nota fecha |
| Clique duplo | Duas requisições, mesma chave | `singleflight` colapsa; o débito acontece uma vez |

Não há compensação e não há saga. A idempotência faz o trabalho: em vez de *desfazer* o
que ficou pela metade, a gente *termina* o que ficou pela metade.

### Resiliência no client do estoque

- Timeout de 3s por requisição, via `context.WithTimeout`, propagado até a query.
- **Disjuntor (circuit breaker):** após 5 falhas seguidas, para de tentar por 10s e
  responde 503 na hora. Depois dos 10s deixa passar uma tentativa para testar se voltou.
  Sem isso, cada usuário espera o timeout inteiro e o serviço caído continua sendo
  martelado.
- **Worker de reconciliação:** goroutine com `time.Ticker` a cada 30s procurando notas
  `ABERTA` com `idempotency_key` preenchida, reenviando a baixa e fechando as que
  confirmarem. É o que fecha sozinho a nota da linha 2 da tabela acima.

## Cenário de falha para o vídeo

```bash
docker compose stop estoque     # derruba
# clicar Imprimir  → tela mostra "Estoque indisponível" + botão Tentar novamente
docker compose start estoque    # sobe
# clicar Tentar novamente → imprime normalmente
```

## Decisões registradas

- **Go**, não C#: é a stack conhecida, e o Angular já consome o risco do prazo.
- **Idempotência no lugar de saga**: metade do código, cobre os mesmos três cenários.
- **Um Postgres, dois databases**: isolamento lógico é o que importa; físico não paga em 5 dias.
- **Header `Idempotency-Key`**: convenção de mercado (Stripe, PayPal), tratável em middleware.
- **Sem o opcional de IA**: caro em tempo e desconectado do resto do sistema.
