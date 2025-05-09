package repositories

import (
	"PinguinMobile/models"
)

type ChatRepository interface {
	SaveMessage(message *models.ChatMessage) error
	GetFamilyMessages(parentID string, channel string, limit, offset int) ([]models.ChatMessage, error)
	GetPrivateMessages(parentID string, user1ID, user2ID string, limit, offset int) ([]models.ChatMessage, error)
	GetUnreadMessagesCount(parentID string, userID string, channel string) (int64, error)
	GetUnreadPrivateCount(parentID string, recipientID string) (int64, error)
	MarkAsRead(messageIDs []uint) error
	DeleteMessage(messageID uint) error
	ModerateMessage(messageID uint, isHidden bool) error
	GetChannelsList(parentID string) ([]string, error)
}
