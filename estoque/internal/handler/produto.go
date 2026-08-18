package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/service"
)

type Produto struct {
	svc *service.Produto
}

func NovoProduto(svc *service.Produto) *Produto {
	return &Produto{svc: svc}
}

// requisicaoProduto é o corpo aceito na criação e na atualização.
//
// Sem tags `binding:"required"` de propósito: em Go o valor zero de int é 0, e o
// validador do Gin trata 0 como "ausente" — um produto com saldo 0, que é
// legítimo, seria recusado. A validação inteira fica no service, num lugar só.
type requisicaoProduto struct {
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao"`
	Saldo     int    `json:"saldo"`
}

func (h *Produto) Listar(c *gin.Context) {
	produtos, err := h.svc.Listar()
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, produtos)
}

func (h *Produto) Buscar(c *gin.Context) {
	id, ok := idDaRota(c)
	if !ok {
		return
	}

	produto, err := h.svc.Buscar(id)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, produto)
}

func (h *Produto) Criar(c *gin.Context) {
	dados, ok := corpoDaRequisicao(c)
	if !ok {
		return
	}

	produto, err := h.svc.Criar(dados)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusCreated, produto)
}

func (h *Produto) Atualizar(c *gin.Context) {
	id, ok := idDaRota(c)
	if !ok {
		return
	}
	dados, ok := corpoDaRequisicao(c)
	if !ok {
		return
	}

	produto, err := h.svc.Atualizar(id, dados)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, produto)
}

func (h *Produto) Remover(c *gin.Context) {
	id, ok := idDaRota(c)
	if !ok {
		return
	}

	if err := h.svc.Remover(id); err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// idDaRota devolve o id da URL. O segundo retorno diz se deu certo — quando não
// dá, a resposta de erro já foi escrita e o handler só precisa retornar.
func idDaRota(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", "O id deve ser um número inteiro positivo")
		return 0, false
	}
	return uint(id), true
}

func corpoDaRequisicao(c *gin.Context) (service.DadosProduto, bool) {
	var req requisicaoProduto
	if err := c.ShouldBindJSON(&req); err != nil {
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", "Corpo da requisição inválido")
		return service.DadosProduto{}, false
	}

	return service.DadosProduto{
		Codigo:    req.Codigo,
		Descricao: req.Descricao,
		Saldo:     req.Saldo,
	}, true
}
