# Detalhamento técnico

Respostas aos itens pedidos no documento do desafio.

> **Rascunho** — reescrever com as minhas palavras antes de gravar o vídeo.

---

## 1. Quais ciclos de vida do Angular foram utilizados

Dois, em todos os três componentes de tela.

**`ngOnInit`** — dispara a carga inicial de dados. Não uso o construtor para isso porque no construtor as dependências injetadas e os inputs ainda não estão necessariamente prontos; o `ngOnInit` roda depois que o Angular terminou de montar o componente.

- `Produtos.ngOnInit` chama `carregar()`
- `Notas.ngOnInit` chama `carregar()`
- `NotaDetalhe.ngOnInit` carrega a lista de produtos e se inscreve no `paramMap` da rota

**`ngOnDestroy`** — encerra as inscrições abertas. Cada componente tem um `Subject` chamado `destruido$`; no `ngOnDestroy` ele emite e completa, e todo Observable da classe passa por `takeUntil(this.destruido$)`.

Sem isso, sair da tela no meio de uma requisição deixaria a inscrição viva, tentando escrever num componente que já não existe. É o vazamento de memória mais comum em Angular.

## 2. Se foi feito uso da biblioteca RxJS e como

Sim. O `HttpClient` do Angular devolve `Observable` em vez de `Promise`, então todo acesso à API já nasce reativo. Os operadores usados:

| Operador | Onde | Para quê |
|---|---|---|
| `takeUntil` | todos os componentes | cancelar a inscrição quando o componente é destruído |
| `finalize` | carregamentos e impressão | desligar o indicador de progresso **tanto no sucesso quanto no erro** — sem ele, uma falha deixaria o spinner girando para sempre |
| `switchMap` | `NotaDetalhe` | trocar o id da rota pela busca da nota, cancelando a busca anterior se o id mudar |
| `map` | `NotaDetalhe` | extrair o `id` do `paramMap` e convertê-lo para número |
| `Subject` | todos os componentes | o gatilho manual que alimenta o `takeUntil` |

O uso mais interessante é o `switchMap` no detalhe da nota:

```ts
this.rota.paramMap.pipe(
  map((parametros) => Number(parametros.get('id'))),
  switchMap((id) => this.service.buscar(id)),
  takeUntil(this.destruido$),
)
```

O `paramMap` é um Observable e não um valor porque, ao navegar de `/notas/1` para `/notas/2`, o Angular **reaproveita o componente** — o `ngOnInit` não roda de novo, só o `paramMap` emite. E `switchMap` (em vez de `mergeMap`) garante que, se dois ids chegarem rápido, a resposta antiga é descartada em vez de sobrescrever a nova.

Também vale a distinção que motivou a escolha: os services **nunca** chamam `.subscribe()`. Eles descrevem a requisição e devolvem o Observable; quem decide executar — e cancelar — é o componente. Um Observable só dispara quando alguém se inscreve, diferente de uma Promise, que já sai executando ao ser criada.

## 3. Quais outras bibliotecas foram utilizadas e para qual finalidade

**Frontend:**

| Biblioteca | Finalidade |
|---|---|
| `@angular/router` | rotas e navegação, com carregamento sob demanda (`loadComponent`) |
| `@angular/forms` (Reactive Forms) | os formulários de cadastro de produto e de inclusão de item, com validação declarativa |
| `@angular/common/http` | cliente HTTP, configurado com `withFetch()` |
| RxJS | já detalhado acima |

Nenhuma dependência de terceiros além do Angular e do Material. Foi decisão consciente: menos superfície para justificar e menos risco de incompatibilidade.

**Backend:** `gin-gonic/gin` (HTTP), `gorm.io/gorm` + `gorm.io/driver/postgres` (acesso a dados). Nada além disso — inclusive a geração da chave de idempotência usa `crypto/rand` da biblioteca padrão, em vez de uma dependência de UUID.

## 4. Para componentes visuais, quais bibliotecas foram utilizadas

**Angular Material** (`@angular/material`), tema Azure Blue.

Componentes usados: `MatToolbar`, `MatCard`, `MatTable`, `MatFormField`, `MatInput`, `MatSelect`, `MatButton`, `MatIcon`, `MatProgressBar`, `MatProgressSpinner`, `MatSnackBar`.

Escolhi Material por ser a biblioteca oficial do Angular — sem risco de descompasso de versão — e porque o `MatProgressSpinner` e o `MatSnackBar` resolvem diretamente dois requisitos do enunciado: o indicador de processamento durante a impressão e o retorno visível de erro ao usuário.

Estilos próprios ficaram limitados a layout (espaçamento, flexbox) e às etiquetas de status.

## 5. Como foi realizado o gerenciamento de dependências no Golang

**Go Modules**, com um módulo independente por serviço — `estoque/go.mod` e `faturamento/go.mod`. Não é um módulo com duas pastas: são dois programas separados que por acaso moram no mesmo repositório, e a fronteira entre eles aparece já na estrutura de arquivos.

- `go mod init <caminho-do-repo>/<serviço>` criou cada módulo
- `go.mod` declara as dependências diretas e a versão do Go
- `go.sum` guarda o **hash criptográfico** de cada dependência da árvore inteira — não é uma lista de versões, é prova de integridade: se o conteúdo de uma versão já publicada mudar, o build falha
- `go mod tidy` sincroniza o `go.mod` com o que o código realmente importa

As dependências marcadas `// indirect` são as que nenhum arquivo `.go` do módulo importa diretamente — vieram arrastadas pela árvore do Gin e do GORM.

Vale citar que o Go usa *minimal version selection*: escolhe a **menor** versão que satisfaz todos os requisitos, não a mais recente. Isso torna os builds reprodutíveis sem precisar de lockfile separado.

Cada serviço também usa `//go:embed` para embutir os arquivos `.sql` de migration dentro do binário — por isso o Dockerfile copia apenas o executável.

## 6. Quais frameworks foram utilizados no Golang

**Gin** para HTTP: roteamento, agrupamento de rotas, middleware e serialização JSON. Usei middleware para CORS e o `gin.Context` para propagar o `context` da requisição até as chamadas de rede e de banco.

**GORM** como ORM sobre o PostgreSQL, com o driver `pgx`. Duas configurações importam:

- `TranslateError: true` — sem isso o GORM devolve a string crua do Postgres, e não há como distinguir uma violação de UNIQUE de qualquer outro erro sem comparar texto. Com ele, `errors.Is(err, gorm.ErrDuplicatedKey)` funciona.
- O pool de conexões é configurado explicitamente (`SetMaxOpenConns(25)`, `SetMaxIdleConns(25)`, `SetConnMaxLifetime(5min)`). O padrão do Go é 2 conexões ociosas e nenhum teto de abertas — com carga concorrente isso derruba e reabre conexão o tempo todo, e sem teto dá para estourar o limite de clientes do Postgres.

As migrations **não** usam `AutoMigrate`: são SQL escrito à mão, porque preciso de `CHECK (saldo >= 0)` e de uma `SEQUENCE` para a numeração das notas, que o AutoMigrate não expressa bem. E deixa o schema visível para quem for ler o repositório.

## 7. Como foram tratados os erros e exceções no backend

Go não tem exceção: erro é valor de retorno, e a função decide o que fazer com ele. A estrutura tem quatro camadas.

**Erros de domínio como sentinelas.** Cada service declara os seus (`ErrNaoEncontrado`, `ErrNotaNaoAberta`, `ErrSaldoInsuficiente`, `ErrCodigoEmUso`). O service **não conhece HTTP** — ele nunca decide um 404. Se amanhã o mesmo service atender uma fila em vez de uma API, nada muda.

**Um único ponto de tradução para HTTP.** A função `responderErroDeServico` é o único lugar que mapeia erro de domínio em código de status. Concentrar isso evita que cada handler escolha um status diferente para o mesmo problema — e mantém a complexidade ciclomática dos handlers em 2 a 4 em vez de 7.

**Envelopamento com `%w`.** Erros de infraestrutura são embrulhados com contexto (`fmt.Errorf("service: falha ao buscar nota %d: %w", id, err)`), preservando a cadeia para o `errors.Is` e o `errors.As` funcionarem lá em cima.

**Erro interno nunca vaza detalhe.** No `default` do switch, o erro real vai para o log e o cliente recebe `{"codigo":"ERRO_INTERNO","mensagem":"Erro interno no servidor"}`. Mensagem de erro de banco carrega nome de tabela, coluna e trecho de query.

Todos os erros saem no mesmo formato, nos dois serviços:

```json
{ "erro": { "codigo": "SALDO_INSUFICIENTE", "mensagem": "...", "detalhes": [...] } }
```

### A distinção que mais importa: 409 versus 503

Um `409` é o sistema **funcionando** e dizendo não — saldo insuficiente, nota já fechada. O usuário precisa mudar alguma coisa; tentar de novo não adianta.

Um `503` é o sistema **quebrado** — o estoque não respondeu. O usuário não precisa mudar nada, só tentar de novo.

São mensagens diferentes na tela e ações diferentes: o 503 mostra um botão "Tentar novamente", o 409 não. Essa separação é o que dá sentido ao requisito de "feedback apropriado ao usuário".

### Erro tipado com detalhes

Quando o estoque recusa por saldo, o faturamento precisa repassar **quais** produtos faltaram. Para isso existe um erro concreto:

```go
type FalhaDeSaldo struct { Detalhes any }
func (f *FalhaDeSaldo) Error() string { return ErrSaldoInsuficiente.Error() }
func (f *FalhaDeSaldo) Unwrap() error { return ErrSaldoInsuficiente }
```

O `Unwrap` faz `errors.Is(err, ErrSaldoInsuficiente)` continuar funcionando, e `errors.As` recupera os detalhes. Sem isso a tela só poderia dizer "não deu".

## 8. Caso a implementação utilize C#, indicar se foi utilizado LINQ

Não se aplica — a implementação é em Go.

---

# Arquitetura e requisitos

## Microsserviços (requisito obrigatório 1)

Dois serviços Go independentes, com **databases separados** no mesmo PostgreSQL:

- **Estoque** (porta 7080, `estoque_db`) — produtos e saldos
- **Faturamento** (porta 7081, `faturamento_db`) — notas fiscais

Cada serviço só recebe a credencial do seu banco. **Nenhum JOIN entre eles é possível** — que é a regra que importa. A dependência é de mão única: o faturamento chama o estoque, nunca o contrário.

Consequência prática: o item da nota guarda uma **cópia** do código e da descrição do produto, tirada no momento em que ele foi incluído. Parece duplicação, mas é obrigatório sem JOIN — e é o comportamento correto no domínio, porque uma nota fiscal registra o que foi vendido *naquele momento*, mesmo que o cadastro do produto mude depois.

## Tratamento de falhas (requisito obrigatório 2)

**Cenário demonstrável:** `docker compose stop estoque`, e então tentar imprimir uma nota.

O que acontece:

1. O faturamento tenta chamar o estoque com timeout de 3 segundos
2. A conexão é recusada → o erro é classificado como `ErrIndisponivel`
3. A API devolve `503 ESTOQUE_INDISPONIVEL` com mensagem em português
4. A tela mostra a mensagem **e um botão "Tentar novamente"**
5. A nota permanece `ABERTA` — nada foi debitado
6. `docker compose start estoque` e o mesmo botão funciona

**Disjuntor (circuit breaker).** Depois de 5 falhas consecutivas, o cliente do estoque para de tentar por 10 segundos e responde na hora, sem esperar o timeout. Passado esse tempo, libera **uma** tentativa para sondar se o serviço voltou: se responder, fecha o disjuntor; se falhar, volta a bloquear.

Sem ele, cada usuário esperaria os 3 segundos completos para receber erro, e um serviço já sobrecarregado continuaria recebendo carga enquanto tenta se recuperar.

Três estados, protegidos por mutex porque o disjuntor é compartilhado entre todas as requisições simultâneas:

```
fechado  → tudo passa
aberto   → nada passa, até o tempo de espera vencer
meio-aberto → uma tentativa passa, para sondar
```

**Desligamento gracioso.** Os dois serviços interceptam `SIGTERM` (que é o que o `docker compose stop` envia) e param de aceitar conexões novas, aguardando até 10 segundos as requisições em andamento terminarem. O log mostra `desligando...` e depois `encerrado` — a queda é controlada, não um processo que some no meio de uma operação.

## Banco de dados real (requisito obrigatório 3)

PostgreSQL 16 em container, com volume persistente. Migrations em SQL versionado, embutidas no binário com `//go:embed`.

O schema usa restrições reais em vez de confiar só na aplicação: `UNIQUE` no código do produto e na chave de idempotência, `CHECK (saldo >= 0)`, `CHECK (quantidade > 0)`, `CHECK (status IN ('ABERTA','FECHADA'))` e uma `SEQUENCE` para a numeração das notas.

---

# Requisitos opcionais

## Idempotência (opcional c)

O problema: imprimir uma nota exige fechar a nota (faturamento) e debitar o saldo (estoque). São dois bancos em dois serviços — **não existe transação cobrindo os dois**. Se o débito passar e o fechamento falhar, o estoque sai errado.

A solução escolhida foi idempotência em vez de saga com compensação. Em vez de *desfazer* o que ficou pela metade, o sistema *termina* o que ficou pela metade.

**Como funciona:**

1. O faturamento gera uma chave única e a **grava na nota antes de chamar o estoque**
2. Chama `POST /baixas` com o header `Idempotency-Key`
3. O estoque debita numa transação e grava a chave junto com o resultado
4. Chave repetida → devolve o resultado guardado, sem debitar de novo

O detalhe que faz tudo funcionar é o passo 1. Se a chave fosse gerada no momento da chamada, cada nova tentativa criaria uma chave diferente, o estoque trataria como pedidos distintos, e debitaria de novo — exatamente o bug que a idempotência existe para impedir. **A chave é da nota, não da tentativa.**

Isso cobre os três cenários de uma vez:

| Falha | O que acontece | Como se resolve |
|---|---|---|
| Estoque fora | nada debitado, nota `ABERTA` | usuário clica de novo |
| Estoque debitou, faturamento morreu antes de fechar | saldo baixado, nota `ABERTA` com a chave salva | nova impressão manda a **mesma** chave → estoque devolve o resultado guardado sem redebitar → nota fecha |
| Clique duplo | duas requisições, mesma chave | o débito acontece uma vez só |

## Concorrência (opcional a)

O débito não faz "ler o saldo, conferir, gravar" — são três momentos e cabe outra requisição no meio. Ele é um comando único:

```sql
UPDATE produtos SET saldo = saldo - $1 WHERE id = $2 AND saldo >= $1
```

A verificação e a escrita são atômicas. Duas requisições disputando o mesmo saldo 1 não se atropelam: uma altera a linha, a outra recebe `RowsAffected == 0`, que a aplicação lê como saldo insuficiente.

Os itens são **ordenados por `produto_id`** antes do débito. Sem isso, duas notas com os mesmos produtos em ordem inversa poderiam travar uma na linha que a outra segura — um deadlock que só aparece sob carga.

Isso vale para múltiplas instâncias do serviço, porque a garantia está no banco. Um `sync.Mutex` em memória não serviria: a trava viveria dentro de um processo, e duas réplicas teriam travas independentes protegendo o mesmo dado.

> Validado por teste automatizado: 100 goroutines disparadas com `sync.WaitGroup` contra um produto de saldo 1 resultam em exatamente 1 sucesso, 99 recusas e saldo final 0.

## Inteligência artificial (opcional b)

Não implementado. Foi um corte consciente de escopo: das três opções, era a que menos conversava com o eixo do desafio — concorrência, idempotência e recuperação de falha são faces do mesmo problema distribuído, e IA seria uma funcionalidade paralela sem ligação com ele.

---

# Como executar

```bash
cp .env.example .env
docker compose up --build
```

| Serviço | Endereço |
|---|---|
| Frontend | http://localhost:4200 |
| Estoque | http://localhost:7080 |
| Faturamento | http://localhost:7081 |

## Roteiro de demonstração

```bash
# 1. Fluxo normal
#    cadastrar produto → criar nota → adicionar item → imprimir
#    a nota vira FECHADA e o saldo do produto diminui

# 2. Bloqueio de reimpressão
#    o botão some e a tela avisa que a nota não pode mais ser alterada

# 3. Falha e recuperação
docker compose stop estoque
#    imprimir → "O serviço de estoque não está respondendo" + Tentar novamente
docker compose start estoque
#    Tentar novamente → imprime normalmente

# 4. Concorrência
cd estoque && go test ./... -run Concorrencia -v
```
