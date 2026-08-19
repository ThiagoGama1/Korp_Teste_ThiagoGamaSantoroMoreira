package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/service"
)

const cabecalhoIdempotencia = "Idempotency-Key"

type Baixa struct {
	svc *service.Baixa
}

func NovaBaixa(svc *service.Baixa) *Baixa {
	return &Baixa{svc: svc}
}

type requisicaoBaixa struct {
	Itens []service.ItemBaixa `json:"itens"`
}

// Executar debita o saldo dos produtos de uma nota.
//
// A chave vem no header e não no corpo por convenção de mercado — Stripe e
// PayPal usam o mesmo nome. Separar "como chamar" de "o que pedir" também
// permite tratá-la em middleware no futuro, sem mexer no payload.
func (h *Baixa) Executar(c *gin.Context) {
	chave := c.GetHeader(cabecalhoIdempotencia)

	var req requisicaoBaixa
	if err := c.ShouldBindJSON(&req); err != nil {
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", "Corpo da requisição inválido")
		return
	}

	corpo, err := h.svc.Executar(c.Request.Context(), chave, req.Itens)

	// O corpo já vem serializado do service, inclusive no caso de recusa. Ele é
	// devolvido como está para que uma repetição da mesma chave receba resposta
	// idêntica byte a byte.
	if errors.Is(err, service.ErrSaldoInsuficiente) {
		c.Data(http.StatusConflict, "application/json; charset=utf-8", corpo)
		return
	}
	if errors.Is(err, service.ErrDadosInvalidos) {
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", err.Error())
		return
	}
	if err != nil {
		log.Printf("estoque: falha na baixa: %v", err)
		responderErro(c, http.StatusInternalServerError, "ERRO_INTERNO", "Erro interno no servidor")
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", corpo)
}
