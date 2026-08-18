package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Health responde se o serviço consegue atender. Não basta devolver 200 fixo:
// um health check que não verifica nada mente para o Docker, que mantém no ar
// um container quebrado.
//
// Checa apenas o próprio banco. O estoque não depende de mais ninguém — e no
// faturamento essa distinção vai importar ainda mais: o estoque estar fora não
// deixa o faturamento doente, então ele também não deve reportar 503 por isso.
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
			"servico": "estoque",
			"status":  "ok",
			"banco":   "ok",
		})
	}
}
