package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ThiagoGama1/Korp_Teste_ThiagoGamaSantoroMoreira/faturamento/internal/handler"
)

func RegistrarHealth(r *gin.Engine, db *gorm.DB) {
	r.GET("/health", handler.Health(db))
}
