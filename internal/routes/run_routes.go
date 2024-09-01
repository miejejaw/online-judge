package routes

import (
	"github.com/gin-gonic/gin"
	"online-judge/internal/controllers"
)

func SetupRunRoutes(router *gin.RouterGroup) {
	userController := controllers.NewRunController()

	userRoutes := router.Group("")
	{
		userRoutes.GET("/run-code", userController.RunCode)
	}
}
