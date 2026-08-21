# Sistema de emissão de Notas Fiscais

Teste prático — Korp ERP (Viasoft). Angular no frontend, Go no backend, arquitetura de microsserviços.

Uma nota fiscal nasce **ABERTA**, recebe produtos com quantidade, e ao ser impressa vira **FECHADA** e debita o saldo dos produtos no estoque. Os dois lados dessa operação vivem em serviços diferentes, com bancos diferentes.

## Como rodar

```bash
git clone https://github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira.git
cd Korp_Teste_ThiagoGamaSantoroMoreira
docker compose up --build
```

Só precisa de Docker. O banco, as migrations e o build do frontend acontecem dentro dos containers.

| Serviço | Endereço | O que é |
|---|---|---|
| Frontend | http://localhost:4200 | Angular + Angular Material |
| Faturamento | http://localhost:7081 | notas fiscais |
| Estoque | http://localhost:7080 | produtos e saldos |
| PostgreSQL | localhost:5442 | dois bancos, um por serviço |

Para recomeçar do zero: `docker compose down -v && docker compose up`.

## O que dá para fazer na tela

1. **Produtos** — cadastrar produto com código, descrição e saldo.
2. **Notas** — criar nota, que nasce com número sequencial e status `ABERTA`.
3. Dentro da nota — adicionar produtos com quantidade, remover itens.
4. **Imprimir** — fecha a nota e debita o saldo. Saldo 10, nota usa 2, vira 8.

Nota fechada não pode mais ser alterada, reimpressa nem apagada.

### Vendo o cenário de falha

```bash
docker compose stop estoque     # derruba um dos serviços
```

Tente imprimir uma nota: a tela mostra `503` com mensagem em português e um botão **Tentar novamente**. O faturamento continua de pé — lista e cria notas normalmente, só a operação que depende do estoque falha.

```bash
docker compose start estoque    # espere ~10s pelo DNS interno do Docker
```

O mesmo botão agora imprime. A nota nunca saiu de `ABERTA` e nada foi debitado no meio do caminho.

## Estrutura

```
estoque/        serviço de produtos e saldos (Go)
faturamento/    serviço de notas fiscais (Go)
web/            frontend (Angular)
infra/          init do PostgreSQL
```

Cada serviço Go segue `cmd/api` → `routes` → `handler` → `service` → `model`, com o `*gorm.DB` entrando por parâmetro. Migrations em SQL, embutidas no binário com `//go:embed`.

## Testes

Os testes de integração precisam do compose no ar.

```bash
cd estoque
TEST_DATABASE_URL="postgres://korp:korp@localhost:5442/estoque_db?sslmode=disable" go test ./... -v

cd ../faturamento
TEST_DATABASE_URL="postgres://korp:korp@localhost:5442/faturamento_db?sslmode=disable" \
TEST_ESTOQUE_URL="http://localhost:7080" go test ./... -v
```

Os testes do disjuntor são unitários e rodam sem nada no ar: `go test ./internal/cliente/...`

## Documentação

| Arquivo | O que tem |
|---|---|
| [DETALHAMENTO.md](DETALHAMENTO.md) | as respostas às oito perguntas da especificação, os requisitos obrigatórios e os opcionais |
| [ARQUITETURA.md](ARQUITETURA.md) | o contrato entre os serviços: schema, endpoints e formato de erro |

## Stack

Go 1.26 · Gin · GORM · PostgreSQL 16 · Angular 22 (zoneless) · Angular Material · RxJS · Docker Compose
