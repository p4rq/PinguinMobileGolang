package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

// DebugAuth выводит информацию из контекста аутентификации
func DebugAuth(c *gin.Context) {
	// Собираем всю информацию из контекста
	firebaseUID, uidExists := c.Get("firebase_uid")
	userType, typeExists := c.Get("user_type")
	claims, claimsExist := c.Get("claims")

	c.JSON(http.StatusOK, gin.H{
		"firebase_uid_exists": uidExists,
		"firebase_uid":        firebaseUID,
		"user_type_exists":    typeExists,
		"user_type":           userType,
		"claims_exists":       claimsExist,
		"claims":              claims,
		"all_context_keys":    c.Keys,
	})
}

// DebugPushNotification отправляет тестовое push-уведомление с полной обработкой ошибок
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

	// Проверяем переменные окружения
	credentialsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	log.Printf("[DEBUG] Using Firebase credentials from: %s", credentialsPath)

	// Инициализируем Firebase App
	var app *firebase.App
	var err error

	// Создаем опции Firebase на основе доступных учетных данных
	var opts []option.ClientOption
	if credentialsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsPath))
	}

	// Создаем Firebase App
	app, err = firebase.NewApp(ctx, nil, opts...)
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

	log.Printf("[DEBUG] Sending notification to token: %s...", request.DeviceToken[:10])

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
		},
	})
}
