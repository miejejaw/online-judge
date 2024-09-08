package routes

import (
	"github.com/gin-gonic/gin"
)

func SetupRoutes(router *gin.RouterGroup) {

	// run routes
	runRoutes := router.Group("/run")
	SetupRunRoutes(runRoutes)
}

//package main
//
//import (
//	"github.com/gin-gonic/gin"
//	"github.com/joho/godotenv"
//	"log"
//	"online-judge/internal/routes"
//)
//
//func main() {
//
//	// Load environment variables from .env file
//	if err := godotenv.Load(); err != nil {
//		log.Fatalf("Error loading .env file")
//	}
//
//	router := gin.Default()
//	api := router.Group("/api")
//	routes.SetupRoutes(api)
//
//	// Start the server
//	if err := router.Run(":8080"); err != nil {
//		log.Fatalf("Server failed to start: %v", err)
//	}
//}
