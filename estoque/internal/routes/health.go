package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/estoque/internal/handler"
)

// RegistrarHealth expõe o endpoint que o Docker usa para saber se o serviço
// está saudável. Fica fora de qualquer grupo de rotas de propósito: health não
// deve depender de prefixo, versionamento ou autenticação para ser alcançável.
func RegistrarHealth(r *gin.Engine, db *gorm.DB) {
	r.GET("/health", handler.Health(db))
}
