package websocket

import (
	"PinguinMobile/models"
	"log"
	"sync"
)

// WebSocketMessage структура сообщения для WebSocket
type WebSocketMessage struct {
	Type       string      `json:"type"`
	ParentID   string      `json:"parent_id"`
	Message    interface{} `json:"message"`
	SenderID   string      `json:"sender_id,omitempty"`
	SenderType string      `json:"sender_type,omitempty"`
	Channel    string      `json:"channel,omitempty"`
}

// Hub управляет всеми соединениями WebSocket
type Hub struct {
	// Зарегистрированные клиенты, сгруппированные по parent_id
	clients map[string]map[*Client]bool

	// Канал входящих сообщений от клиентов
	broadcast chan WebSocketMessage

	// Регистрация новых клиентов
	register chan *Client

	// Отмена регистрации клиентов
	unregister chan *Client

	// Мьютекс для синхронизации доступа к clients
	mu sync.Mutex
}

// NewHub создает новый хаб
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan WebSocketMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run запускает хаб WebSocket
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.parentID]; !ok {
				h.clients[client.parentID] = make(map[*Client]bool)
			}
			h.clients[client.parentID][client] = true
			log.Printf("Client registered: %s, type: %s, parentID: %s", client.userID, client.userType, client.parentID)
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.parentID]; ok {
				delete(h.clients[client.parentID], client)
				close(client.send)
				// Если это был последний клиент для этой семьи, удаляем карту
				if len(h.clients[client.parentID]) == 0 {
					delete(h.clients, client.parentID)
				}
				log.Printf("Client unregistered: %s", client.userID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			if clients, ok := h.clients[message.ParentID]; ok {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.clients, client.parentID)
						}
					}
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastChatMessage отправляет сообщение чата всем клиентам семьи
func (h *Hub) BroadcastChatMessage(chatMessage *models.ChatMessage) {
	// Подготавливаем сообщение для отправки
	payload := WebSocketMessage{
		Type:       "chat_message",
		ParentID:   chatMessage.ParentID,
		Message:    chatMessage,
		SenderID:   chatMessage.SenderID,
		SenderType: chatMessage.SenderType,
		Channel:    chatMessage.Channel,
	}

	// Кодируем и отправляем
	h.broadcast <- payload
}

// BroadcastSystemMessage отправляет системное сообщение
func (h *Hub) BroadcastSystemMessage(parentID string, messageType string, content interface{}) {
	payload := WebSocketMessage{
		Type:     messageType,
		ParentID: parentID,
		Message:  content,
	}

	h.broadcast <- payload
}

// BroadcastDeviceUsage отправляет информацию об использовании устройства ребенка родителю
func (h *Hub) BroadcastDeviceUsage(parentID string, usageData map[string]interface{}) {
	message := WebSocketMessage{
		Type:     "device_usage",
		ParentID: parentID,
		Message:  usageData,
	}
	h.broadcast <- message
}

// BroadcastAppActivity отправляет информацию о текущем активном приложении
func (h *Hub) BroadcastAppActivity(parentID string, childID string, appData map[string]interface{}) {
	message := WebSocketMessage{
		Type:     "app_activity",
		ParentID: parentID,
		Message:  appData,
		SenderID: childID,
	}
	h.broadcast <- message
}

// BroadcastScreenTimeAlert отправляет уведомление о превышении лимита экранного времени
func (h *Hub) BroadcastScreenTimeAlert(parentID string, childID string, alertData map[string]interface{}) {
	message := WebSocketMessage{
		Type:     "screen_time_alert",
		ParentID: parentID,
		Message:  alertData,
		SenderID: childID,
	}
	h.broadcast <- message
}

// BroadcastBlockedContentAttempt отправляет уведомление о попытке доступа к запрещенному контенту
func (h *Hub) BroadcastBlockedContentAttempt(parentID string, childID string, contentData map[string]interface{}) {
	message := WebSocketMessage{
		Type:     "blocked_content_attempt",
		ParentID: parentID,
		Message:  contentData,
		SenderID: childID,
	}
	h.broadcast <- message
}

// BroadcastScreenTimeLimit отправляет информацию о новых настройках лимита экранного времени
func (h *Hub) BroadcastScreenTimeLimit(childID string, parentID string, limitData map[string]interface{}) {
	message := WebSocketMessage{
		Type:     "screen_time_limit_update",
		ParentID: parentID,
		Message:  limitData,
		SenderID: childID,
	}
	h.broadcast <- message
}
