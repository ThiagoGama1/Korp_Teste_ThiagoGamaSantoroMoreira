package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Health checa apenas o próprio banco — de propósito.
//
// O estoque estar fora não deixa o faturamento doente: ele continua listando e
// criando notas, só a impressão para. Reportar 503 por causa do estoque faria o
// Docker reiniciar um serviço saudável, derrubando os dois em vez de um — e
// desmontando justamente a demonstração de falha.
func Health(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		sqlDB, err := db.DB()
		if err != nil {
			responderErro(c, http.StatusServiceUnavailable, "BANCO_INDISPONIVEL", "Sem conexão com o banco de dados")
			return
		}

		if err := sqlDB.PingContext(ctx); err != nil {
			responderErro(c, http.StatusServiceUnavailable, "BANCO_INDISPONIVEL", "O banco de dados não respondeu")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"servico": "faturamento",
			"status":  "ok",
			"banco":   "ok",
		})
	}
}
