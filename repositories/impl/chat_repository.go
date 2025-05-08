package impl

import (
	"PinguinMobile/models"

	"gorm.io/gorm"
)

type ChatRepositoryImpl struct {
	DB *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepositoryImpl {
	return &ChatRepositoryImpl{DB: db}
}

func (r *ChatRepositoryImpl) SaveMessage(message *models.ChatMessage) error {
	return r.DB.Create(message).Error
}

func (r *ChatRepositoryImpl) GetFamilyMessages(parentID string, limit, offset int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	query := r.DB.Where("parent_id = ?", parentID).Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&messages).Error
	return messages, err
}

func (r *ChatRepositoryImpl) GetUnreadMessagesCount(parentID string, userID string) (int64, error) {
	var count int64
	err := r.DB.Model(&models.ChatMessage{}).
		Where("parent_id = ? AND sender_id != ? AND is_read = ?", parentID, userID, false).
		Count(&count).Error
	return count, err
}

func (r *ChatRepositoryImpl) MarkAsRead(messageIDs []uint) error {
	return r.DB.Model(&models.ChatMessage{}).
		Where("id IN ?", messageIDs).
		Update("is_read", true).Error
}

func (r *ChatRepositoryImpl) DeleteMessage(messageID uint) error {
	return r.DB.Delete(&models.ChatMessage{}, messageID).Error
}
