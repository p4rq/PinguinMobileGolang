package controllers

import (
	"PinguinMobile/services"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TestPushNotification отправляет тестовое push-уведомление указанному пользователю
func TestPushNotification(c *gin.Context) {
	// Получаем данные текущего пользователя из JWT
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	currentUserUID := claims.(map[string]interface{})["uid"].(string)
	// Структура запроса
	var request struct {
		RecipientUID string            `json:"recipient_uid" binding:"required"`
		Title        string            `json:"title" binding:"required"`
		Body         string            `json:"body" binding:"required"`
		Data         map[string]string `json:"data"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем сервисы
	parentService := c.MustGet("parentService").(*services.ParentService)
	notifyService := c.MustGet("notificationService").(*services.NotificationService)

	// Логируем действие
	log.Printf("[PUSH-TEST] User %s is sending test notification to %s",
		currentUserUID, request.RecipientUID)

	// Проверяем, существует ли получатель как родитель
	parent, err := parentService.ReadParent(request.RecipientUID)
	if err == nil {
		// Это родитель
		if parent.DeviceToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Parent exists but has no device token. Login from a mobile device first.",
			})
			return
		}

		// Отправляем уведомление
		err = notifyService.SendNotification(
			parent.DeviceToken,
			request.Title,
			request.Body,
			request.Data,
			parent.Lang,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to send notification: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Push notification sent to parent",
			"details": gin.H{
				"recipient_type": "parent",
				"recipient_name": parent.Name,
				"device_token":   maskToken(parent.DeviceToken),
				"title":          request.Title,
				"body":           request.Body,
				"lang":           parent.Lang,
			},
		})
		return
	}

	// Проверяем, существует ли получатель как ребенок
	child, err := parentService.ReadChild(request.RecipientUID)
	if err == nil {
		// Это ребенок
		if child.DeviceToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Child exists but has no device token. Login from a mobile device first.",
			})
			return
		}

		// Отправляем уведомление
		err = notifyService.SendNotification(
			child.DeviceToken,
			request.Title,
			request.Body,
			request.Data,
			child.Lang,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to send notification: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Push notification sent to child",
			"details": gin.H{
				"recipient_type": "child",
				"recipient_name": child.Name,
				"device_token":   maskToken(child.DeviceToken),
				"title":          request.Title,
				"body":           request.Body,
				"lang":           child.Lang,
			},
		})
		return
	}

	// Если не найден ни родитель, ни ребенок
	c.JSON(http.StatusNotFound, gin.H{
		"error": "Recipient not found. Please check the firebase_uid.",
	})
}

// maskToken скрывает большую часть токена для безопасности в логах
func maskToken(token string) string {
	if len(token) <= 10 {
		return "***" // Слишком короткий токен
	}
	return token[:5] + "..." + token[len(token)-5:]
}
