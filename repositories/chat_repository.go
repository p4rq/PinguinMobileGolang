package repositories

import (
	"PinguinMobile/models"
)

type ChatRepository interface {
	SaveMessage(message *models.ChatMessage) error
	GetFamilyMessages(parentID string, limit, offset int) ([]models.ChatMessage, error)
	GetUnreadMessagesCount(parentID string, userID string) (int64, error)
	MarkAsRead(messageIDs []uint) error
	DeleteMessage(messageID uint) error
}
