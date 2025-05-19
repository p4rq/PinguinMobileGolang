package impl

import (
	"PinguinMobile/models"
	"log"

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

func (r *ChatRepositoryImpl) GetFamilyMessages(parentID string, channel string, limit int, offset int) ([]models.ChatMessage, error) {
	log.Printf("DEBUG: GetFamilyMessages called with parentID=%s, channel=%s, limit=%d, offset=%d",
		parentID, channel, limit, offset)

	var messages []models.ChatMessage

	// Построение запроса
	query := r.DB.Where("parent_id = ?", parentID)

	// Если указан канал, добавляем условие
	if channel != "" {
		query = query.Where("channel = ?", channel)
	}

	// Выполнение запроса с сортировкой по времени
	result := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&messages)

	if result.Error != nil {
		log.Printf("ERROR: Failed to get messages: %v", result.Error)
		return nil, result.Error
	}

	log.Printf("DEBUG: Found %d messages for parentID=%s", len(messages), parentID)

	// Дополнительная отладка - вывод первого сообщения, если оно есть
	if len(messages) > 0 {
		log.Printf("DEBUG: First message: ID=%d, Text=%s, CreatedAt=%v",
			messages[0].ID, messages[0].Message, messages[0].CreatedAt)
	}

	return messages, nil
}

func (r *ChatRepositoryImpl) GetPrivateMessages(parentID string, user1ID, user2ID string, limit, offset int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	query := r.DB.Where("parent_id = ? AND is_private = ? AND ((sender_id = ? AND recipient_id = ?) OR (sender_id = ? AND recipient_id = ?)) AND is_hidden = ?",
		parentID, true, user1ID, user2ID, user2ID, user1ID, false)

	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&messages).Error
	return messages, err
}

func (r *ChatRepositoryImpl) GetUnreadMessagesCount(parentID string, userID string, channel string) (int64, error) {
	var count int64
	query := r.DB.Model(&models.ChatMessage{}).
		Where("parent_id = ? AND sender_id != ? AND is_private = ? AND is_read = ? AND is_hidden = ?",
			parentID, userID, false, false, false)

	if channel != "" {
		query = query.Where("channel = ?", channel)
	}

	err := query.Count(&count).Error
	return count, err
}

func (r *ChatRepositoryImpl) GetUnreadPrivateCount(parentID string, recipientID string) (int64, error) {
	var count int64
	err := r.DB.Model(&models.ChatMessage{}).
		Where("parent_id = ? AND recipient_id = ? AND is_private = ? AND is_read = ? AND is_hidden = ?",
			parentID, recipientID, true, false, false).
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

func (r *ChatRepositoryImpl) ModerateMessage(messageID uint, isHidden bool) error {
	return r.DB.Model(&models.ChatMessage{}).
		Where("id = ?", messageID).
		Updates(map[string]interface{}{
			"is_moderated": true,
			"is_hidden":    isHidden,
		}).Error
}

func (r *ChatRepositoryImpl) GetChannelsList(parentID string) ([]string, error) {
	var channels []string
	err := r.DB.Model(&models.ChatMessage{}).
		Where("parent_id = ? AND is_private = ? AND is_hidden = ?", parentID, false, false).
		Distinct("channel").
		Pluck("channel", &channels).Error
	return channels, err
}
