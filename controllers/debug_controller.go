package controllers

import (
	"PinguinMobile/config"
	"PinguinMobile/models"
	"PinguinMobile/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Используем общий сервис уведомлений
var debugNotificationService *services.NotificationService

// Сервис ребенка для получения данных
var debugChildService *services.ChildService

// Сервис родителя для получения данных
var debugParentService *services.ParentService

// Инициализируем сервисы для отладочного контроллера
func SetDebugServices(notifySrv *services.NotificationService,
	childSrv *services.ChildService,
	parentSrv *services.ParentService) {
	debugNotificationService = notifySrv
	debugChildService = childSrv
	debugParentService = parentSrv
}

// TestFCMNotification отправляет тестовое уведомление по произвольному токену устройства
func TestFCMNotification(c *gin.Context) {
	var request struct {
		Token string `json:"token" binding:"required"` // FCM токен устройства
		Title string `json:"title" binding:"required"` // Заголовок уведомления
		Body  string `json:"body" binding:"required"`  // Текст уведомления
		Lang  string `json:"lang"`                     // Язык (опционально)
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем, инициализирован ли сервис уведомлений
	if debugNotificationService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Сервис уведомлений не инициализирован. Добавьте вызов SetDebugServices в main.go",
		})
		return
	}

	// Добавим данные для уведомления
	data := map[string]string{
		"notification_type": "test_notification",
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	// Отправляем уведомление
	err := debugNotificationService.SendNotification(
		request.Token,
		request.Title,
		request.Body,
		data,
		request.Lang,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Уведомление успешно отправлено",
	})
}

// TestChildNotification отправляет уведомление ребенку по его Firebase UID
func TestChildNotification(c *gin.Context) {
	var request struct {
		FirebaseUID string `json:"firebase_uid" binding:"required"` // Firebase UID ребенка
		Title       string `json:"title" binding:"required"`        // Заголовок уведомления
		Body        string `json:"body" binding:"required"`         // Текст уведомления
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем, инициализирован ли сервис уведомлений
	if debugNotificationService == nil || debugChildService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Сервисы не инициализированы. Добавьте вызов SetDebugServices в main.go",
		})
		return
	}

	// Получаем данные ребенка
	child, err := debugChildService.ReadChild(request.FirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Ребенок не найден: " + err.Error(),
		})
		return
	}

	// Проверяем наличие токена устройства
	if child.DeviceToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "У ребенка отсутствует токен устройства",
		})
		return
	}

	// Добавим данные для уведомления
	data := map[string]string{
		"notification_type": "test_notification",
		"child_id":          "test",
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	// Отправляем уведомление
	err = debugNotificationService.SendNotification(
		child.DeviceToken,
		request.Title,
		request.Body,
		data,
		child.Lang,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Уведомление успешно отправлено ребенку",
		"token_preview": child.DeviceToken[:15] + "...",
		"token_length":  len(child.DeviceToken),
	})
}

// TestParentNotification отправляет уведомление родителю по его Firebase UID
func TestParentNotification(c *gin.Context) {
	var request struct {
		FirebaseUID string `json:"firebase_uid" binding:"required"` // Firebase UID родителя
		Title       string `json:"title" binding:"required"`        // Заголовок уведомления
		Body        string `json:"body" binding:"required"`         // Текст уведомления
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем, инициализирован ли сервис уведомлений
	if debugNotificationService == nil || debugParentService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Сервисы не инициализированы. Добавьте вызов SetDebugServices в main.go",
		})
		return
	}

	// Получаем данные родителя
	parent, err := debugParentService.ReadParent(request.FirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Родитель не найден: " + err.Error(),
		})
		return
	}

	// Проверяем наличие токена устройства
	if parent.DeviceToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "У родителя отсутствует токен устройства",
		})
		return
	}

	// Добавим данные для уведомления
	data := map[string]string{
		"notification_type": "test_notification",
		"parent_id":         "test",
		"timestamp":         time.Now().Format(time.RFC3339),
	}

	// Отправляем уведомление
	err = debugNotificationService.SendNotification(
		parent.DeviceToken,
		request.Title,
		request.Body,
		data,
		parent.Lang,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Уведомление успешно отправлено родителю",
		"token_preview": parent.DeviceToken[:15] + "...",
		"token_length":  len(parent.DeviceToken),
	})
}

// GetAllDeviceTokens получает список всех FCM токенов в системе
func GetAllDeviceTokens(c *gin.Context) {
	// Получаем токены родителей
	var parents []models.Parent
	if err := config.DB.Select("id, name, firebase_uid, device_token").Find(&parents).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Получаем токены детей
	var children []models.Child
	if err := config.DB.Select("id, name, firebase_uid, device_token").Find(&children).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Формируем результат
	parentTokens := make([]map[string]interface{}, 0)
	for _, p := range parents {
		if p.DeviceToken != "" {
			tokenPreview := p.DeviceToken
			if len(tokenPreview) > 20 {
				tokenPreview = tokenPreview[:20] + "..."
			}

			parentTokens = append(parentTokens, map[string]interface{}{
				"id":                   p.ID,
				"name":                 p.Name,
				"firebase_uid":         p.FirebaseUID,
				"device_token_preview": tokenPreview,
				"device_token_length":  len(p.DeviceToken),
				"device_token":         p.DeviceToken, // Полный токен для тестирования
			})
		}
	}

	childTokens := make([]map[string]interface{}, 0)
	for _, c := range children {
		if c.DeviceToken != "" {
			tokenPreview := c.DeviceToken
			if len(tokenPreview) > 20 {
				tokenPreview = tokenPreview[:20] + "..."
			}

			childTokens = append(childTokens, map[string]interface{}{
				"id":                   c.ID,
				"name":                 c.Name,
				"firebase_uid":         c.FirebaseUID,
				"device_token_preview": tokenPreview,
				"device_token_length":  len(c.DeviceToken),
				"device_token":         c.DeviceToken, // Полный токен для тестирования
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"parent_tokens": parentTokens,
		"child_tokens":  childTokens,
	})
}
