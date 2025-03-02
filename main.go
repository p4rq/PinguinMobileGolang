package main

import (
	"PinguinMobile/config"
	"PinguinMobile/controllers"
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

	config.InitDatabase()
	config.InitFirebase()

	// Pass the DB and FirebaseAuth instances to the controllers
	controllers.SetDB(config.DB)
	controllers.SetFirebaseAuth(config.FirebaseAuth)

	// Initialize services
	authService := services.NewAuthService(config.DB, config.FirebaseAuth)
	controllers.SetAuthService(authService)

	childService := services.NewChildService(config.DB, config.FirebaseAuth)
	controllers.SetChildService(childService)

	r := gin.Default()

	// Register routes
	routes.RegisterRoutes(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	r.Run(":" + port)
}
