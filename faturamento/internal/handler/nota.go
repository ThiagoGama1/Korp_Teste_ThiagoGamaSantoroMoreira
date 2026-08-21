package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/service"
)

type Nota struct {
	svc *service.Nota
}

func NovaNota(svc *service.Nota) *Nota {
	return &Nota{svc: svc}
}

// Sem tags `binding:"required"`: o validador do Gin trata o valor zero de int
// como ausente, e a mensagem que ele gera é técnica demais para chegar na tela.
// A validação fica no service, num lugar só.
type requisicaoItem struct {
	ProdutoID  uint `json:"produto_id"`
	Quantidade int  `json:"quantidade"`
}

func (h *Nota) Listar(c *gin.Context) {
	notas, err := h.svc.Listar()
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, notas)
}

func (h *Nota) Buscar(c *gin.Context) {
	id, ok := idDaRota(c, "id")
	if !ok {
		return
	}

	nota, err := h.svc.Buscar(id)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, nota)
}

func (h *Nota) Criar(c *gin.Context) {
	nota, err := h.svc.Criar()
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusCreated, nota)
}

func (h *Nota) AdicionarItem(c *gin.Context) {
	notaID, ok := idDaRota(c, "id")
	if !ok {
		return
	}

	var req requisicaoItem
	if err := c.ShouldBindJSON(&req); err != nil {
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", "Corpo da requisição inválido")
		return
	}

	// O context da requisição desce até a chamada HTTP ao estoque: se o usuário
	// fechar a aba, a chamada é cancelada em vez de seguir ocupando conexão.
	nota, err := h.svc.AdicionarItem(c.Request.Context(), notaID, req.ProdutoID, req.Quantidade)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, nota)
}

func (h *Nota) Remover(c *gin.Context) {
	id, ok := idDaRota(c, "id")
	if !ok {
		return
	}

	if err := h.svc.Remover(id); err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Nota) RemoverItem(c *gin.Context) {
	notaID, ok := idDaRota(c, "id")
	if !ok {
		return
	}
	itemID, ok := idDaRota(c, "item_id")
	if !ok {
		return
	}

	nota, err := h.svc.RemoverItem(notaID, itemID)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, nota)
}

// Imprimir fecha a nota e debita o estoque. É a única rota que atravessa a
// fronteira entre os dois serviços, então é a única que pode devolver 503.
func (h *Nota) Imprimir(c *gin.Context) {
	id, ok := idDaRota(c, "id")
	if !ok {
		return
	}

	// O context da requisição desce até a chamada HTTP ao estoque, carregando o
	// prazo e o cancelamento junto.
	nota, err := h.svc.Imprimir(c.Request.Context(), id)
	if err != nil {
		responderErroDeServico(c, err)
		return
	}
	c.JSON(http.StatusOK, nota)
}

// idDaRota lê um parâmetro numérico da URL. O segundo retorno diz se deu certo —
// quando não dá, a resposta de erro já foi escrita e o handler só precisa sair.
func idDaRota(c *gin.Context, nome string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(nome), 10, 64)
	if err != nil {
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", "O parâmetro "+nome+" deve ser um número inteiro positivo")
		return 0, false
	}
	return uint(id), true
}
