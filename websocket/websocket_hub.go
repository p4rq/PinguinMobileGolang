package websocket

import (
	"PinguinMobile/models"
	"log"
	"strings"
	"sync"
	"time"
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

	// Новое поле для хранения истории сообщений по parent_id
	// Ограничим историю до 100 последних сообщений для каждой семьи
	messageHistory   map[string][]WebSocketMessage
	historyMaxSize   int
	messageHistoryMu sync.Mutex // Отдельный мьютекс для истории сообщений

	// Добавьте сервис сообщений
	messageService ChatMessageService
}

// ChatMessageService интерфейс для работы с сообщениями чата
type ChatMessageService interface {
	SaveMessage(message *models.ChatMessage) error
	GetMessages(parentID string, limit int) ([]*models.ChatMessage, error)
}

// NewHub создает новый хаб
func NewHub(messageService ChatMessageService) *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]bool),
		broadcast:      make(chan WebSocketMessage),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		messageHistory: make(map[string][]WebSocketMessage),
		historyMaxSize: 100, // Хранить до 100 последних сообщений
		messageService: messageService,
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

			// Отправляем историю новому клиенту
			go h.sendMessageHistory(client)

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

			// Сохраняем сообщение в историю
			h.saveMessageToHistory(message)
		}
	}
}

// BroadcastChatMessage отправляет сообщение чата всем клиентам семьи и сохраняет его в истории
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

	// Дополнительно можно сохранить сообщение в базу данных через отдельный сервис
	// messageService.SaveMessage(chatMessage)
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

// saveMessageToHistory сохраняет сообщение в историю
func (h *Hub) saveMessageToHistory(message WebSocketMessage) {
	// Сохраняем только сообщения чата и системные уведомления
	if message.Type != "chat_message" && !strings.HasPrefix(message.Type, "system_") {
		return // Игнорируем другие типы сообщений
	}

	h.messageHistoryMu.Lock()
	defer h.messageHistoryMu.Unlock()

	// Инициализируем историю для данной семьи, если еще не существует
	if _, exists := h.messageHistory[message.ParentID]; !exists {
		h.messageHistory[message.ParentID] = make([]WebSocketMessage, 0, h.historyMaxSize)
	}

	// Добавляем сообщение в историю
	history := h.messageHistory[message.ParentID]

	// Если история достигла максимального размера, удаляем самое старое сообщение
	if len(history) >= h.historyMaxSize {
		history = history[1:] // Удаляем первый элемент (самое старое сообщение)
	}

	// Добавляем новое сообщение
	history = append(history, message)
	h.messageHistory[message.ParentID] = history

	// Сохраняем сообщение в БД, если это сообщение чата
	if message.Type == "chat_message" && h.messageService != nil {
		// Проверяем, что сообщение содержит необходимые поля
		if msg, ok := message.Message.(map[string]interface{}); ok {
			if text, exists := msg["text"].(string); exists {
				// Создаем объект сообщения с правильными именами полей
				// ВАЖНО: используйте фактические имена полей из models.ChatMessage
				chatMessage := &models.ChatMessage{
					ParentID:  message.ParentID,
					SenderID:  message.SenderID,
					Message:   text,
					CreatedAt: time.Now(),
					Channel:   message.Channel,
				}

				// Вызываем сервис для сохранения сообщения в БД
				err := h.messageService.SaveMessage(chatMessage)
				if err != nil {
					log.Printf("Error saving message to database: %v", err)
				}
			}
		}
	}
}

// getMessageHistory возвращает историю сообщений для указанной семьи
func (h *Hub) getMessageHistory(parentID string) []WebSocketMessage {
	h.messageHistoryMu.Lock()
	defer h.messageHistoryMu.Unlock()

	if history, exists := h.messageHistory[parentID]; exists {
		// Создаем копию слайса, чтобы избежать проблем с конкурентным доступом
		result := make([]WebSocketMessage, len(history))
		copy(result, history)
		return result
	}

	return make([]WebSocketMessage, 0)
}

// sendMessageHistory отправляет историю сообщений новому клиенту
func (h *Hub) sendMessageHistory(client *Client) {
	history := h.getMessageHistory(client.parentID)

	if len(history) == 0 {
		return // Нет истории для отправки
	}

	// Создаем контейнер для истории сообщений
	historyContainer := WebSocketMessage{
		Type:     "message_history",
		ParentID: client.parentID,
		Message:  history,
	}

	// Отправляем историю клиенту
	select {
	case client.send <- historyContainer:
		// Успешно отправлено
	default:
		// Канал клиента переполнен или закрыт
	}
}
