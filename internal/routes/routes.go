package routes

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.RouterGroup) {

	// run routes
	runRoutes := router.Group("/run")
	SetupRunRoutes(runRoutes)
}
