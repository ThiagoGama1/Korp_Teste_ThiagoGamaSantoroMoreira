package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/handler"
	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/service"
)

// RegistrarProdutos monta a cadeia service -> handler -> rotas.
//
// Nada aqui usa variável global: o *gorm.DB entra por parâmetro e desce até o
// service. É injeção de dependência na mão, e é o que permite trocar o banco por
// outro numa eventual suíte de testes sem mexer no handler.
func RegistrarProdutos(r *gin.Engine, db *gorm.DB) {
	produtoHandler := handler.NovoProduto(service.NovoProduto(db))

	produtos := r.Group("/produtos")
	{
		produtos.GET("", produtoHandler.Listar)
		produtos.POST("", produtoHandler.Criar)
		produtos.GET("/:id", produtoHandler.Buscar)
		produtos.PUT("/:id", produtoHandler.Atualizar)
		produtos.DELETE("/:id", produtoHandler.Remover)
	}
}
