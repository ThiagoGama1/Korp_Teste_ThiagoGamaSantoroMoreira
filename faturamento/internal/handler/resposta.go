package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/cliente"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/service"
)

// Mesmo formato de erro do serviço de estoque, definido no ARQUITETURA.md.
// O Angular tem um caminho único para ler erro dos dois serviços.
type corpoErro struct {
	Erro detalheErro `json:"erro"`
}

type detalheErro struct {
	Codigo   string `json:"codigo"`
	Mensagem string `json:"mensagem"`
	Detalhes any    `json:"detalhes,omitempty"`
}

func responderErro(c *gin.Context, status int, codigo, mensagem string) {
	c.JSON(status, corpoErro{Erro: detalheErro{Codigo: codigo, Mensagem: mensagem}})
}

// responderErroDeServico é o único lugar que traduz erro de domínio em HTTP.
//
// A distinção que mais importa aqui é entre 409 e 503: 409 é o sistema
// funcionando e dizendo não (o usuário precisa mudar alguma coisa); 503 é o
// sistema quebrado (o usuário só precisa tentar de novo). São mensagens
// diferentes na tela, e essa separação é metade do requisito de tratamento de
// falhas.
func responderErroDeServico(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNaoEncontrado):
		responderErro(c, http.StatusNotFound, "NAO_ENCONTRADO", "Registro não encontrado")
	case errors.Is(err, cliente.ErrProdutoNaoEncontrado):
		responderErro(c, http.StatusNotFound, "PRODUTO_NAO_ENCONTRADO", "Produto não encontrado no estoque")
	case errors.Is(err, service.ErrDadosInvalidos):
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", err.Error())
	case errors.Is(err, service.ErrNotaNaoAberta):
		responderErro(c, http.StatusConflict, "NOTA_NAO_ABERTA", "A nota já foi fechada e não pode mais ser alterada")
	case errors.Is(err, service.ErrBaixaPendente):
		responderErro(c, http.StatusConflict, "BAIXA_PENDENTE",
			"O estoque desta nota já foi baixado. Conclua a impressão antes de alterar os itens.")
	case errors.Is(err, service.ErrNotaVazia):
		responderErro(c, http.StatusConflict, "NOTA_VAZIA", "A nota não tem itens")
	case errors.Is(err, cliente.ErrSaldoInsuficiente):
		// errors.As recupera o erro concreto para repassar QUAIS produtos
		// faltaram. Sem isso a tela só poderia dizer "não deu".
		var falha *cliente.FalhaDeSaldo
		if errors.As(err, &falha) {
			c.JSON(http.StatusConflict, corpoErro{Erro: detalheErro{
				Codigo:   "SALDO_INSUFICIENTE",
				Mensagem: "Não há saldo suficiente para imprimir esta nota",
				Detalhes: falha.Detalhes,
			}})
			return
		}
		responderErro(c, http.StatusConflict, "SALDO_INSUFICIENTE", "Não há saldo suficiente para imprimir esta nota")
	case errors.Is(err, cliente.ErrIndisponivel):
		// Log em nível separado: 503 aqui significa que o outro serviço caiu, e
		// é o sinal que a gente quer ver no terminal durante a demonstração.
		log.Printf("faturamento: estoque indisponível: %v", err)
		responderErro(c, http.StatusServiceUnavailable, "ESTOQUE_INDISPONIVEL", "O serviço de estoque não está respondendo. Tente novamente em instantes.")
	default:
		log.Printf("faturamento: erro não tratado: %v", err)
		responderErro(c, http.StatusInternalServerError, "ERRO_INTERNO", "Erro interno no servidor")
	}
}
