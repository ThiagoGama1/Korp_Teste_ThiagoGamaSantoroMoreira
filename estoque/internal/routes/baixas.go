package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/handler"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/service"
)

// RegistrarBaixas expoe a operacao que o faturamento chama ao imprimir uma nota.
// E o unico endpoint do estoque que altera saldo, e o unico que exige chave de
// idempotencia.
func RegistrarBaixas(r *gin.Engine, db *gorm.DB) {
	baixaHandler := handler.NovaBaixa(service.NovaBaixa(db))
	r.POST("/baixas", baixaHandler.Executar)
}
