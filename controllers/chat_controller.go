package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

var chatService *services.ChatService

func SetChatService(service *services.ChatService) {
	chatService = service
}

// SendTextMessage отправляет новое текстовое сообщение
func SendTextMessage(c *gin.Context) {
	fmt.Println("Получен запрос на отправку текстового сообщения")
	fmt.Println("Authorization Header:", c.GetHeader("Authorization"))

	var input struct {
		ParentID    string `json:"parent_id" binding:"required"`
		Message     string `json:"message" binding:"required"`
		Channel     string `json:"channel"`
		IsPrivate   bool   `json:"is_private"`
		RecipientID string `json:"recipient_id"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем ID пользователя из токена
	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Определяем тип отправителя (родитель или ребенок)
	userType, _ := c.Get("user_type")
	isParent := userType == "parent"

	message, err := chatService.SendMessage(
		userID.(string),
		input.ParentID,
		input.Message,
		input.Channel,
		input.IsPrivate,
		input.RecipientID,
		isParent)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": message})
}

// SendMediaMessage отправляет сообщение с медиа-файлом
func SendMediaMessage(c *gin.Context) {
	parentID := c.PostForm("parent_id")
	if parentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parent_id is required"})
		return
	}

	message := c.PostForm("message") // Может быть пустым
	channel := c.PostForm("channel") // Может быть пустым

	isPrivateStr := c.PostForm("is_private")
	isPrivate := isPrivateStr == "true"

	recipientID := c.PostForm("recipient_id") // Может быть пустым
	messageType := c.PostForm("message_type")

	if messageType != models.MessageTypeImage &&
		messageType != models.MessageTypeFile &&
		messageType != models.MessageTypeVideo &&
		messageType != models.MessageTypeAudio {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message_type"})
		return
	}

	// Получаем файл
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// Получаем ID пользователя из токена
	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Определяем тип отправителя (родитель или ребенок)
	userType, _ := c.Get("user_type")
	isParent := userType == "parent"

	// Fix: Use a different variable name to store the result
	chatMessage, err := chatService.SendMediaMessage(
		userID.(string),
		parentID,
		message,
		messageType,
		channel,
		isPrivate,
		recipientID,
		isParent,
		file)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": chatMessage})
}

// GetFamilyMessages получает сообщения семьи с пагинацией
func GetFamilyMessages(c *gin.Context) {
	parentID := c.Param("parent_id")
	channel := c.Query("channel") // Может быть пустым для всех каналов

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := chatService.GetFamilyMessages(userID.(string), parentID, channel, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": messages})
}

// GetPrivateMessages получает личные сообщения между двумя пользователями
func GetPrivateMessages(c *gin.Context) {
	parentID := c.Param("parent_id")
	otherUserID := c.Param("user_id")

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := chatService.GetPrivateMessages(userID.(string), parentID, otherUserID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": messages})
}

// MarkAsRead отмечает сообщения как прочитанные
func MarkAsRead(c *gin.Context) {
	var input struct {
		MessageIDs []uint `json:"message_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	err := chatService.MarkMessagesAsRead(input.MessageIDs, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteMessage удаляет сообщение
func DeleteMessage(c *gin.Context) {
	messageIDStr := c.Param("message_id")
	messageID, err := strconv.Atoi(messageIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userType, _ := c.Get("user_type")
	isParent := userType == "parent"

	err = chatService.DeleteMessage(uint(messageID), userID.(string), isParent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ModerateMessage позволяет родителю модерировать сообщение
func ModerateMessage(c *gin.Context) {
	var input struct {
		MessageID uint `json:"message_id" binding:"required"`
		IsHidden  bool `json:"is_hidden"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	userType, _ := c.Get("user_type")
	isParent := userType == "parent"

	if !isParent {
		c.JSON(http.StatusForbidden, gin.H{"error": "only parents can moderate messages"})
		return
	}

	err := chatService.ModerateMessage(input.MessageID, userID.(string), input.IsHidden)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetUnreadCount получает количество непрочитанных сообщений
func GetUnreadCount(c *gin.Context) {
	parentID := c.Param("parent_id")
	channel := c.Query("channel") // Может быть пустым для всех каналов

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	count, err := chatService.GetUnreadCount(parentID, userID.(string), channel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}

// GetUnreadPrivateCount получает количество непрочитанных личных сообщений
func GetUnreadPrivateCount(c *gin.Context) {
	parentID := c.Param("parent_id")

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	count, err := chatService.GetUnreadPrivateCount(parentID, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_private_count": count})
}

// GetChannelsList получает список активных каналов в чате семьи
func GetChannelsList(c *gin.Context) {
	parentID := c.Param("parent_id")

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	channels, err := chatService.GetChannelsList(parentID, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"channels": channels})
}
