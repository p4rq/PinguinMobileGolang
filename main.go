package main

import (
	"PinguinMobile/config"
	"PinguinMobile/controllers"
	"PinguinMobile/repositories/impl"
	"PinguinMobile/routes"
	"PinguinMobile/services"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables")
	}

	// Initialize database and Firebase
	config.InitDatabase()
	config.InitFirebase()

	// Initialize repositories
	parentRepo := impl.NewParentRepository(config.DB)
	childRepo := impl.NewChildRepository(config.DB)

	// Initialize services
	authService := services.NewAuthService(config.DB, config.FirebaseAuth)
	childService := services.NewChildService(childRepo, parentRepo, config.FirebaseAuth)

	// Set services in controllers
	controllers.SetAuthService(authService)
	controllers.SetChildService(childService)

	// Initialize Gin router
	r := gin.Default()

	// Register routes
	routes.RegisterRoutes(r)

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	r.Run(":" + port)
}
