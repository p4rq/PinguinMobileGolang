package controllers

import (
	"PinguinMobile/models"
)

// ChatServiceInterface определяет методы для работы с сообщениями чата
type ChatServiceInterface interface {
	SaveMessage(message *models.ChatMessage) error
	GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error)
}
