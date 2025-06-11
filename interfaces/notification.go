package interfaces

import (
	"time"
)

// NotificationService определяет интерфейс для сервиса уведомлений
type NotificationService interface {
	SendNotificationToFamily(parentID, title, body string, data map[string]string, skipUsers ...string) error
	SendNotification(token, title, body string, data map[string]string, lang string) error
}

// WebSocketHubService определяет интерфейс для WebSocket хаба
type WebSocketHubService interface {
	NotifyLimitChange(parentID string, childToken string)
	// Добавьте другие методы, которые вы используете...
}

// WebSocketMessage определяет структуру сообщения для WebSocket
type WebSocketMessage struct {
	Type          string      `json:"type"`
	ParentID      string      `json:"parent_id,omitempty"`
	SenderID      string      `json:"sender_id,omitempty"`
	SenderName    string      `json:"sender_name,omitempty"`
	Message       interface{} `json:"message,omitempty"`
	Timestamp     time.Time   `json:"timestamp"`
	MessageID     uint        `json:"message_id,omitempty"`
	ChildToken    string      `json:"child_token,omitempty"`
	IsChangeLimit bool        `json:"isChangeLimit,omitempty"`
}
