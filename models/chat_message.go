package models

import (
	"time"
)

type ChatMessage struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ParentID    string    `json:"parent_id" gorm:"index"` // firebase_uid родителя (владельца семьи)
	SenderID    string    `json:"sender_id" gorm:"index"` // firebase_uid отправителя
	SenderType  string    `json:"sender_type"`            // "parent" или "child"
	SenderName  string    `json:"sender_name"`            // Имя отправителя для отображения
	Message     string    `json:"message"`                // Текст сообщения
	MessageType string    `json:"message_type"`           // "text", "image", "system"
	Attachment  string    `json:"attachment,omitempty"`   // URL вложения, если есть
	IsRead      bool      `json:"is_read"`                // Прочитано ли сообщение
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
}
