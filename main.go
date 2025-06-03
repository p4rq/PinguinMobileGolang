package main

import (
	"PinguinMobile/config"
	"PinguinMobile/controllers"
	"PinguinMobile/models"
	"PinguinMobile/repositories/impl"
	"PinguinMobile/routes"
	"PinguinMobile/services"
	"PinguinMobile/websocket"
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

// ChatServiceAdapter адаптирует ChatService для соответствия интерфейсу ChatMessageService
type ChatServiceAdapter struct {
	chatService *services.ChatService
}

// SaveMessage реализует интерфейс ChatMessageService для сохранения сообщений
func (a *ChatServiceAdapter) SaveMessage(message *models.ChatMessage) error {

	return a.chatService.ChatRepo.SaveMessage(message)
}

// GetMessages реализует интерфейс ChatMessageService для получения сообщений
func (a *ChatServiceAdapter) GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error) {
	log.Printf("ChatServiceAdapter.GetMessages: parentID=%s, userID=%s", parentID, userID)

	// Если пользователь - родитель своей семьи, то у него всегда есть доступ
	if userID == parentID {
		log.Printf("User is parent of family, access granted")
		// Используем parentID как userID, чтобы пропустить проверку авторизации
		msgs, err := a.chatService.GetFamilyMessages(parentID, parentID, "", limit, 0)
		if err != nil {
			return nil, err
		}

		// Преобразуем []models.ChatMessage в []*models.ChatMessage
		result := make([]*models.ChatMessage, len(msgs))
		for i := range msgs {
			result[i] = &msgs[i]
		}
		return result, nil
	}

	// Для остальных пользователей проверяем принадлежность к семье
	msgs, err := a.chatService.GetFamilyMessages(userID, parentID, "", limit, 0)
	if err != nil {
		log.Printf("Error in GetFamilyMessages: %v", err)
		return nil, err
	}

	// Преобразуем []models.ChatMessage в []*models.ChatMessage
	result := make([]*models.ChatMessage, len(msgs))
	for i := range msgs {
		result[i] = &msgs[i]
	}

	return result, nil
}

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
	chatService := services.NewChatService(chatRepo, parentRepo, childRepo)

	// Set services in controllers
	controllers.SetAuthService(authService)
	controllers.SetChildService(childService)
	controllers.SetChatService(chatService)

	// Инициализируем Firebase app
	opt := option.WithCredentialsFile(os.Getenv("FIREBASE_CREDENTIALS_PATH"))
	firebaseApp, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("Error initializing Firebase: %v", err)
	}

	// Инициализируем сервисы
	translationService := services.NewTranslationService(config.DB)
	controllers.SetTranslationService(translationService)

	// Инициализируем сервис уведомлений
	notificationService, err := services.NewNotificationService(
		firebaseApp,
		translationService,
		parentRepo,
		childRepo,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize notification service: %v", err)
		notificationService = nil
	} else {
		log.Println("Notification service initialized successfully")
	}

	// Передаем сервис уведомлений другим сервисам
	parentService := services.NewParentService(parentRepo, childRepo, notificationService)
	controllers.SetParentService(parentService)

	// 1. Создаем адаптер для ChatService
	chatAdapter := &ChatServiceAdapter{
		chatService: chatService,
	}

	// 2. Инициализация WebSocket Hub с правильным количеством аргументов
	wsHub := websocket.NewHub(chatAdapter, notificationService, config.DB)
	go wsHub.Run()
	controllers.SetWebSocketHub(wsHub)

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
