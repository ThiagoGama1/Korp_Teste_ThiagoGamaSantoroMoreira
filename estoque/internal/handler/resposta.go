package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/service"
)

// Formato de erro definido no ARQUITETURA.md. Os dois serviços respondem igual,
// então o Angular tem um único caminho para ler mensagem de erro.
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
// Concentrar aqui evita que cada handler escolha um status diferente para o
// mesmo problema.
//
// O default é deliberado: erro desconhecido virou 500 com mensagem genérica, e
// o detalhe real vai só para o log. Mensagem de erro de banco pode conter nome
// de tabela, coluna e trecho de query — informação que não deve chegar ao
// cliente.
func responderErroDeServico(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNaoEncontrado):
		responderErro(c, http.StatusNotFound, "NAO_ENCONTRADO", "Registro não encontrado")
	case errors.Is(err, service.ErrDadosInvalidos):
		responderErro(c, http.StatusBadRequest, "PAYLOAD_INVALIDO", err.Error())
	case errors.Is(err, service.ErrCodigoEmUso):
		responderErro(c, http.StatusConflict, "CODIGO_EM_USO", "Já existe um produto com este código")
	default:
		log.Printf("estoque: erro não tratado: %v", err)
		responderErro(c, http.StatusInternalServerError, "ERRO_INTERNO", "Erro interno no servidor")
	}
}
