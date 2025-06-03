package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// DebugPushNotification отправляет тестовое push-уведомление
func DebugPushNotification(c *gin.Context) {
	var request struct {
		DeviceToken string            `json:"device_token" binding:"required"`
		Title       string            `json:"title" binding:"required"`
		Body        string            `json:"body" binding:"required"`
		Data        map[string]string `json:"data"`
		Lang        string            `json:"lang"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Устанавливаем язык по умолчанию, если не указан
	if request.Lang == "" {
		request.Lang = "ru"
	}

	// Создаем контекст
	ctx := context.Background()

	// ====== ВАЖНОЕ ИСПРАВЛЕНИЕ: ДОБАВЛЯЕМ ЯВНУЮ КОНФИГУРАЦИЮ FIREBASE ======
	// Проверяем переменную окружения PROJECT_ID или берем значение по умолчанию
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		projectID = "pinguin-46f73" // Укажите здесь ID вашего проекта Firebase
	}

	// Создаем конфигурацию Firebase с явным указанием projectID
	config := &firebase.Config{
		ProjectID: projectID,
	}

	// Создаем опции Firebase
	var opts []option.ClientOption

	// Проверяем наличие файла учетных данных службы или переменной окружения
	credentialsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if credentialsPath != "" {
		log.Printf("[DEBUG] Using Firebase credentials from file: %s", credentialsPath)
		opts = append(opts, option.WithCredentialsFile(credentialsPath))
	} else {
		// Используем JSON строку из переменной окружения
		credentialsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
		if credentialsJSON != "" {
			log.Printf("[DEBUG] Using Firebase credentials from environment JSON")
			opts = append(opts, option.WithCredentialsJSON([]byte(credentialsJSON)))
		} else {
			// Для локального тестирования используем файл из директории проекта
			defaultPath := "pinguin-46f73-firebase-adminsdk-fbsvc-8610ba7d3e.json"
			if _, err := os.Stat(defaultPath); err == nil {
				log.Printf("[DEBUG] Using Firebase credentials from default path: %s", defaultPath)
				opts = append(opts, option.WithCredentialsFile(defaultPath))
			} else {
				// Если не нашли файл, возвращаем ошибку
				errMsg := "Firebase service account credentials not found. Please set GOOGLE_APPLICATION_CREDENTIALS or FIREBASE_CREDENTIALS_JSON"
				log.Printf("[ERROR] %s", errMsg)
				c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
				return
			}
		}
	}

	// Инициализируем Firebase с явной конфигурацией
	app, err := firebase.NewApp(ctx, config, opts...)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to initialize Firebase app: %v", err)
		log.Printf("[ERROR] %s", errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	log.Printf("[DEBUG] Firebase app initialized successfully")

	// Инициализируем FCM клиент
	fcmClient, err := app.Messaging(ctx)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to initialize FCM client: %v", err)
		log.Printf("[ERROR] %s", errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	log.Printf("[DEBUG] FCM client initialized successfully")

	// Создаем и отправляем сообщение
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: request.Title,
			Body:  request.Body,
		},
		Data:  request.Data,
		Token: request.DeviceToken,
	}

	log.Printf("[DEBUG] Sending notification to token: %s...",
		request.DeviceToken[:10])

	// Отправляем сообщение
	response, err := fcmClient.Send(ctx, message)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to send FCM message: %v", err)
		log.Printf("[ERROR] %s", errMsg)
		c.JSON(http.StatusInternalServerError, gin.H{"error": errMsg})
		return
	}

	log.Printf("[SUCCESS] Message sent successfully: %s", response)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Push notification sent successfully",
		"details": gin.H{
			"device_token": request.DeviceToken[:10] + "...",
			"response_id":  response,
			"title":        request.Title,
			"body":         request.Body,
			"project_id":   projectID,
		},
	})
}
