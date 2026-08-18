// Package cliente concentra a comunicação HTTP com o serviço de estoque.
//
// Isolar isso num pacote é deliberado: quando perguntarem onde a falha do outro
// serviço é tratada, a resposta é uma pasta — e não `if err` espalhado pelos
// handlers. É também onde o disjuntor vai entrar.
package cliente

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	// ErrIndisponivel: o estoque não respondeu — caiu, estourou o timeout ou
	// devolveu 5xx. Vira 503 para o usuário, com opção de tentar de novo.
	ErrIndisponivel = errors.New("estoque indisponível")

	// ErrProdutoNaoEncontrado: o estoque respondeu, e a resposta foi "não
	// existe". Isso é o sistema funcionando, não quebrando — vira 404.
	ErrProdutoNaoEncontrado = errors.New("produto não encontrado no estoque")
)

type Produto struct {
	ID        uint   `json:"id"`
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao"`
	Saldo     int    `json:"saldo"`
}

type Estoque struct {
	baseURL string
	http    *http.Client
}

func NovoEstoque(baseURL string, timeout time.Duration) *Estoque {
	return &Estoque{
		baseURL: baseURL,
		// O timeout do http.Client é a rede de segurança final. O context de
		// cada chamada pode apertar mais que isso, nunca afrouxar.
		http: &http.Client{Timeout: timeout},
	}
}

func (e *Estoque) BuscarProduto(ctx context.Context, produtoID uint) (*Produto, error) {
	url := fmt.Sprintf("%s/produtos/%d", e.baseURL, produtoID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("cliente: requisição inválida: %w", err)
	}

	resp, err := e.http.Do(req)
	if err != nil {
		// Conexão recusada, DNS falhando, timeout: o estoque não está
		// alcançável. Não distinguimos os casos porque a ação do usuário é a
		// mesma nos três — tentar de novo mais tarde.
		return nil, fmt.Errorf("%w: %v", ErrIndisponivel, err)
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
