package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS libera o navegador a chamar esta API de outra origem.
//
// O Angular roda em localhost:4200 e a API em outra porta — para o navegador
// isso é origem diferente, e sem estes cabeçalhos toda requisição é bloqueada
// antes mesmo de sair.
//
// Origem liberada para qualquer uma porque este é um projeto de avaliação que
// roda local. Num sistema real seria a lista de domínios do frontend.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// Idempotency-Key precisa estar aqui: cabeçalho fora da lista padrão é
		// recusado no preflight, e a impressão para de funcionar.
		c.Header("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key")

		// O navegador manda um OPTIONS antes de POST/PUT/DELETE para perguntar se
		// pode. Esse preflight é respondido aqui e não segue para as rotas.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
