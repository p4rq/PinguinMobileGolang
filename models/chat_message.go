package models

import (
	"time"
)

// Константы для типов сообщений и каналов
const (
	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeFile  = "file"
	MessageTypeVideo = "video"
	MessageTypeAudio = "audio"

	ChannelGeneral   = "general"
	ChannelStudy     = "study"
	ChannelFun       = "fun"
	ChannelImportant = "important"
)

type ChatMessage struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	ParentID   string    `gorm:"column:parent_id" json:"parent_id"`
	SenderID   string    `gorm:"column:sender_id" json:"sender_id"`
	SenderName string    `gorm:"column:sender_name" json:"sender_name"`
	Message    string    `gorm:"column:message" json:"message"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// MessageID  uint      `json:"message_id,omitempty"` // ID сообщения в базе данных

	// Минимальный набор полей для простого чата
}
