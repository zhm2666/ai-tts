package routers

import (
	"ai-transform-backend/transform-web-api/controllers"
	"github.com/gin-gonic/gin"
)

func InitTransformRouters(g *gin.RouterGroup, controller *controllers.Transform) {
	v1 := g.Group("/v1")
	v1.POST("/translate", controller.Translate)
	v1.GET("/records", controller.GetRecords)
}
