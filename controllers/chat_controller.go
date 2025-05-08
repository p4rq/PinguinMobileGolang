package controllers

import (
	"PinguinMobile/services"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

var chatService *services.ChatService

func SetChatService(service *services.ChatService) {
	chatService = service
}

// SendMessage отправляет новое сообщение
func SendMessage(c *gin.Context) {
	var input struct {
		ParentID    string `json:"parent_id" binding:"required"` // firebase_uid родителя (владельца семьи)
		Message     string `json:"message" binding:"required"`
		MessageType string `json:"message_type"`
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

	if input.MessageType == "" {
		input.MessageType = "text"
	}

	message, err := chatService.SendMessage(userID.(string), input.ParentID, input.Message, input.MessageType, isParent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": message})
}

// GetFamilyMessages получает сообщения семьи с пагинацией
func GetFamilyMessages(c *gin.Context) {
	parentID := c.Param("parent_id")

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	messages, err := chatService.GetFamilyMessages(userID.(string), parentID, limit, offset)
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

	err = chatService.DeleteMessage(uint(messageID), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetUnreadCount получает количество непрочитанных сообщений
func GetUnreadCount(c *gin.Context) {
	parentID := c.Param("parent_id")

	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	count, err := chatService.GetUnreadCount(parentID, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"unread_count": count})
}
