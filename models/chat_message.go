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
	ID          uint      `json:"id" gorm:"primaryKey"`
	ParentID    string    `json:"parent_id" gorm:"index"` // firebase_uid родителя (семьи)
	SenderID    string    `json:"sender_id" gorm:"index"` // firebase_uid отправителя
	SenderType  string    `json:"sender_type"`            // "parent" или "child"
	SenderName  string    `json:"sender_name"`            // Имя отправителя для отображения
	RecipientID string    `json:"recipient_id,omitempty"` // firebase_uid получателя (если персональное)
	IsPrivate   bool      `json:"is_private"`             // Флаг персонального сообщения
	Channel     string    `json:"channel"`                // Канал сообщения (general, study, fun, important)
	Message     string    `json:"message"`                // Текст сообщения
	MessageType string    `json:"message_type"`           // "text", "image", "file", "video", "audio"
	MediaURL    string    `json:"media_url,omitempty"`    // URL медиа-файла
	MediaName   string    `json:"media_name,omitempty"`   // Имя файла
	MediaSize   int64     `json:"media_size,omitempty"`   // Размер файла в байтах
	IsModerated bool      `json:"is_moderated"`           // Промодерировано ли сообщение
	IsRead      bool      `json:"is_read"`                // Прочитано ли сообщение
	IsHidden    bool      `json:"is_hidden"`              // Скрыто ли сообщение модератором
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
}
