// Package cliente concentra a comunicação HTTP com o serviço de estoque.
//
// Isolar isso num pacote é deliberado: quando perguntarem onde a falha do outro
// serviço é tratada, a resposta é uma pasta — e não `if err` espalhado pelos
// handlers.
package cliente

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	// ErrIndisponivel: o estoque não respondeu — caiu, estourou o timeout, ou o
	// disjuntor está aberto. Vira 503 para o usuário, com opção de tentar de novo.
	ErrIndisponivel = errors.New("estoque indisponível")

	// ErrProdutoNaoEncontrado e ErrSaldoInsuficiente: o estoque respondeu, e a
	// resposta foi "não". Isso é o sistema funcionando, não quebrando.
	ErrProdutoNaoEncontrado = errors.New("produto não encontrado no estoque")
	ErrSaldoInsuficiente    = errors.New("saldo insuficiente")
)

// FalhaDeSaldo carrega quais produtos faltaram, para a tela poder dizer
// exatamente o que reduzir em vez de um "não deu" genérico.
//
// Unwrap faz errors.Is(err, ErrSaldoInsuficiente) funcionar, e errors.As
// recupera os detalhes. É o padrão de erro tipado do Go.
type FalhaDeSaldo struct {
	Detalhes any
}

func (f *FalhaDeSaldo) Error() string { return ErrSaldoInsuficiente.Error() }
func (f *FalhaDeSaldo) Unwrap() error { return ErrSaldoInsuficiente }

type Produto struct {
	ID        uint   `json:"id"`
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao"`
	Saldo     int    `json:"saldo"`
}

type ItemBaixa struct {
	ProdutoID  uint `json:"produto_id"`
	Quantidade int  `json:"quantidade"`
}

type ResultadoBaixa struct {
	Status string `json:"status"`
	Itens  []struct {
		ProdutoID     uint `json:"produto_id"`
		SaldoAnterior int  `json:"saldo_anterior"`
		SaldoAtual    int  `json:"saldo_atual"`
	} `json:"itens"`
}

type Estoque struct {
	baseURL   string
	http      *http.Client
	disjuntor *Disjuntor
}

func NovoEstoque(baseURL string, timeout time.Duration, disjuntor *Disjuntor) *Estoque {
	return &Estoque{
		baseURL: baseURL,
		// O timeout do http.Client é a rede de segurança final. O context de cada
		// chamada pode apertar mais que isso, nunca afrouxar.
		http:      &http.Client{Timeout: timeout},
		disjuntor: disjuntor,
	}
}

func (e *Estoque) BuscarProduto(ctx context.Context, produtoID uint) (*Produto, error) {
	url := fmt.Sprintf("%s/produtos/%d", e.baseURL, produtoID)

	resp, err := e.enviar(ctx, http.MethodGet, url, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrProdutoNaoEncontrado
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: respondeu %d", ErrIndisponivel, resp.StatusCode)
	}

	var produto Produto
	if err := json.NewDecoder(resp.Body).Decode(&produto); err != nil {
		return nil, fmt.Errorf("%w: resposta ilegível: %v", ErrIndisponivel, err)
	}
	return &produto, nil
}

// Baixar debita o saldo dos produtos. A chave vai no header Idempotency-Key: se
// a mesma chave chegar duas vezes, o estoque devolve o resultado da primeira sem
// debitar de novo. É o que torna seguro repetir esta chamada.
func (e *Estoque) Baixar(ctx context.Context, chave string, itens []ItemBaixa) (*ResultadoBaixa, error) {
	corpo, err := json.Marshal(map[string]any{"itens": itens})
	if err != nil {
		return nil, fmt.Errorf("cliente: falha ao montar o corpo da baixa: %w", err)
	}

	cabecalhos := map[string]string{
		"Content-Type":    "application/json",
		"Idempotency-Key": chave,
	}

	resp, err := e.enviar(ctx, http.MethodPost, e.baseURL+"/baixas", corpo, cabecalhos)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, lerFalhaDeSaldo(resp)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: respondeu %d", ErrIndisponivel, resp.StatusCode)
	}

	var resultado ResultadoBaixa
	if err := json.NewDecoder(resp.Body).Decode(&resultado); err != nil {
		return nil, fmt.Errorf("%w: resposta ilegível: %v", ErrIndisponivel, err)
	}
	return &resultado, nil
}

// enviar centraliza o disjuntor e a classificação de falha de rede. Toda chamada
// ao estoque passa por aqui, então não há como esquecer de registrar o resultado.
func (e *Estoque) enviar(ctx context.Context, metodo, url string, corpo []byte, cabecalhos map[string]string) (*http.Response, error) {
	if !e.disjuntor.Permitir() {
		// Falha instantânea: o serviço já se sabe fora, não faz sentido gastar
		// mais 3 segundos de timeout para confirmar.
		return nil, fmt.Errorf("%w: disjuntor aberto", ErrIndisponivel)
	}

	req, err := http.NewRequestWithContext(ctx, metodo, url, bytes.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("cliente: requisição inválida: %w", err)
	}
	for chave, valor := range cabecalhos {
		req.Header.Set(chave, valor)
	}

	resp, err := e.http.Do(req)
	if err != nil {
		// Cancelamento vindo de quem chamou (usuário fechou a aba, requisição
		// abortada) não é culpa do estoque, e não pode contar como falha: cinco
		// abas fechadas abririam o disjuntor contra um serviço saudável.
		//
		// Timeout é diferente e continua contando — ali o estoque realmente não
		// respondeu no prazo.
		if errors.Is(err, context.Canceled) {
			// Se esta chamada era a sondagem do estado meio-aberto, o disjuntor
			// precisa saber que ela não teve desfecho — senão fica preso nesse
			// estado, recusando tudo até o processo reiniciar. Com o disjuntor
			// fechado, o método não faz nada.
			e.disjuntor.AbortarSonda()
			return nil, fmt.Errorf("%w: requisição cancelada", ErrIndisponivel)
		}

		// Conexão recusada, DNS falhando, timeout: o estoque não está alcançável.
		// Não distinguimos os três porque a ação do usuário é a mesma nos três.
		e.disjuntor.RegistrarFalha()
		return nil, fmt.Errorf("%w: %v", ErrIndisponivel, err)
	}

	// 5xx conta como falha do serviço; 4xx não — um 409 é o estoque funcionando
	// e dizendo não, e abrir o disjuntor por causa disso derrubaria o sistema por
	// excesso de pedidos legítimos sem saldo.
	if resp.StatusCode >= http.StatusInternalServerError {
		e.disjuntor.RegistrarFalha()
		resp.Body.Close()
		return nil, fmt.Errorf("%w: respondeu %d", ErrIndisponivel, resp.StatusCode)
	}

	e.disjuntor.RegistrarSucesso()
	return resp, nil
}

func lerFalhaDeSaldo(resp *http.Response) error {
	var corpo struct {
		Erro struct {
			Detalhes any `json:"detalhes"`
		} `json:"erro"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		return ErrSaldoInsuficiente
	}
	return &FalhaDeSaldo{Detalhes: corpo.Erro.Detalhes}
}
