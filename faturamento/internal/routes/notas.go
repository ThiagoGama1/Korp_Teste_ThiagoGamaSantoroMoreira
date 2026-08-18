package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/cliente"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/handler"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/service"
)

// RegistrarNotas monta a cadeia cliente -> service -> handler -> rotas.
// Tudo entra por parâmetro, sem variável global: é injeção de dependência na
// mão, e é o que permitiria trocar o cliente do estoque por um falso num teste.
func RegistrarNotas(r *gin.Engine, db *gorm.DB, estoque *cliente.Estoque) {
	notaHandler := handler.NovaNota(service.NovaNota(db, estoque))

	notas := r.Group("/notas")
	{
		notas.GET("", notaHandler.Listar)
		notas.POST("", notaHandler.Criar)
		notas.GET("/:id", notaHandler.Buscar)
		notas.POST("/:id/itens", notaHandler.AdicionarItem)
		notas.DELETE("/:id/itens/:item_id", notaHandler.RemoverItem)
	}
}
