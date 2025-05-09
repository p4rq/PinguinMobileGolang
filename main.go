package main

import (
	"PinguinMobile/config"
	"PinguinMobile/controllers"
	"PinguinMobile/models"
	"PinguinMobile/repositories/impl"
	"PinguinMobile/routes"
	"PinguinMobile/services"
	"PinguinMobile/websocket"
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

	// Migrate the schema
	config.DB.AutoMigrate(&models.ChatMessage{})

	// Initialize repositories
	parentRepo := impl.NewParentRepository(config.DB)
	childRepo := impl.NewChildRepository(config.DB)
	chatRepo := impl.NewChatRepository(config.DB)

	// Initialize services
	authService := services.NewAuthService(parentRepo, childRepo, config.FirebaseAuth)
	childService := services.NewChildService(childRepo, parentRepo, config.FirebaseAuth)
	parentService := services.NewParentService(parentRepo, childRepo)
	chatService := services.NewChatService(chatRepo, parentRepo, childRepo)

	// Set services in controllers
	controllers.SetAuthService(authService)
	controllers.SetChildService(childService)
	controllers.SetParentService(parentService)
	controllers.SetChatService(chatService)
	// controllers.InitWebsocket(chatService)

	// Инициализация WebSocket Hub
	webSocketHub := websocket.NewHub()
	controllers.SetWebSocketHub(webSocketHub)

	// Initialize Gin router
	r := gin.Default()

	// Register routes
	routes.RegisterRoutes(r)

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000" // Порт по умолчанию
	}

	log.Printf("Starting server on port %s...", port)
	r.Run(":" + port)
}
