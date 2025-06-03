package websocket

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// WebSocketMessage упрощенная структура для сообщений
type WebSocketMessage struct {
	Type       string      `json:"type"`                  // "chat_message" или "message_history"
	ParentID   string      `json:"parent_id"`             // ID родителя/семьи
	SenderID   string      `json:"sender_id"`             // ID отправителя
	SenderName string      `json:"sender_name,omitempty"` // Имя отправителя
	Message    interface{} `json:"message"`               // Содержимое сообщения или массив сообщений для history
	Timestamp  time.Time   `json:"timestamp"`             // Время отправки
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

	// Сервис для работы с сообщениями (экспортируемое поле)
	MessageService ChatMessageService // Изменено с messageService на MessageService

	// База данных для получения информации о пользователях
	db *gorm.DB

	// Сервис для отправки уведомлений
	NotifySrv *services.NotificationService
}

// ChatMessageService интерфейс для работы с сообщениями чата
type ChatMessageService interface {
	SaveMessage(message *models.ChatMessage) error
	GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error)
}

// NewHub создает новый хаб
func NewHub(messageService ChatMessageService, notifySrv *services.NotificationService, db *gorm.DB) *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]bool),
		broadcast:      make(chan WebSocketMessage),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		MessageService: messageService, // Изменено с messageService на MessageService
		NotifySrv:      notifySrv,
		db:             db,
	}
}

// Run запускает хаб WebSocket
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// RegisterClient экспортируемый метод для регистрации клиента
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient экспортируемый метод для отключения клиента
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// BroadcastMessage экспортируемый метод для отправки сообщения
func (h *Hub) BroadcastMessage(message WebSocketMessage) {
	h.broadcast <- message
}

// registerClient внутренний метод для регистрации клиента
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Создаем группу для семьи, если ее еще нет
	if _, ok := h.clients[client.ParentID]; !ok {
		h.clients[client.ParentID] = make(map[*Client]bool)
	}

	// Добавляем клиента в семью
	h.clients[client.ParentID][client] = true
	log.Printf("Клиент %s добавлен в семью %s", client.UserID, client.ParentID)

	// Загружаем и отправляем историю сообщений клиенту
	go h.sendMessageHistory(client)
}

// unregisterClient внутренний метод для отключения клиента
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Проверяем существование семьи
	if clients, ok := h.clients[client.ParentID]; ok {
		// Удаляем клиента
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send) // Исправьте Send на send
			log.Printf("Клиент %s удален из семьи %s", client.UserID, client.ParentID)
		}

		// Если в семье не осталось клиентов, удаляем семью
		if len(clients) == 0 {
			delete(h.clients, client.ParentID)
			log.Printf("Семья %s удалена (нет подключенных клиентов)", client.ParentID)
		}
	}
}

// broadcastMessage внутренний метод для отправки сообщений
func (h *Hub) broadcastMessage(message WebSocketMessage) {
	// Если это чатовое сообщение, сохраняем его в базу
	// if message.Type == "chat_message" {
	// 	// Сохраняем сообщение в БД
	// 	if msgText, ok := message.Message.(string); ok && h.MessageService != nil {
	// 		chatMessage := &models.ChatMessage{
	// 			ParentID:   message.ParentID,
	// 			SenderID:   message.SenderID,
	// 			SenderName: message.SenderName,
	// 			Message:    msgText,
	// 			CreatedAt:  message.Timestamp,
	// 		}

	// 		err := h.MessageService.SaveMessage(chatMessage)
	// 		if err != nil {
	// 			log.Printf("Ошибка при сохранении сообщения: %v", err)
	// 		}
	// 	}
	// }

	// Отправляем сообщение всем членам семьи
	h.mu.Lock()
	defer h.mu.Unlock()

	// Сохраняем список активных пользователей для дальнейшей проверки
	activeUsers := make(map[string]bool)

	clients, ok := h.clients[message.ParentID]
	if !ok {
		log.Printf("Семья %s не найдена, сообщение не доставлено", message.ParentID)
		h.mu.Unlock()
		return
	}

	for client := range clients {
		// Добавляем пользователя в список активных
		activeUsers[client.UserID] = true

		select {
		case client.send <- message: // Исправьте Send на send
			// Сообщение успешно отправлено
		default:
			// Буфер клиента переполнен, отключаем его
			h.unregisterClient(client)
		}
	}
	h.mu.Unlock()

	// Отправляем push-уведомления пользователям, которые не в сети
	if message.Type == "chat_message" && h.NotifySrv != nil {
		// Если это сообщение чата и сервис уведомлений инициализирован
		if msgText, ok := message.Message.(string); ok {
			// Найдем всех членов семьи, которые НЕ активны сейчас
			// и отправим им push-уведомления
			// Это нужно делать в отдельной горутине, чтобы не блокировать основной поток
			go func() {
				// Список пользователей для пропуска (отправитель и активные пользователи)
				skipUsers := []string{message.SenderID}
				for u := range activeUsers {
					skipUsers = append(skipUsers, u)
				}

				// Отправляем push-уведомления всем членам семьи, кроме активных
				h.NotifySrv.SendNotificationToFamily(
					message.ParentID,
					message.SenderName, // Имя отправителя как заголовок
					msgText,            // Текст сообщения как тело
					map[string]string{
						"notification_type": "chat_message",
						"sender_id":         message.SenderID,
						"parent_id":         message.ParentID,
					},
					skipUsers...,
				)
			}()
		}
	}
}

// sendMessageHistory отправляет историю сообщений новому клиенту
func (h *Hub) sendMessageHistory(client *Client) {
	if h.MessageService == nil {
		log.Printf("Сервис сообщений не инициализирован, история не будет отправлена")
		return
	}

	// Получаем историю сообщений из базы данных
	messages, err := h.MessageService.GetMessages(client.ParentID, client.UserID, 30)
	if err != nil {
		log.Printf("Ошибка при загрузке истории сообщений: %v", err)
		return
	}

	// Преобразуем сообщения в простой формат для отправки
	history := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		history[i] = map[string]interface{}{
			"sender_id":   msg.SenderID,
			"sender_name": msg.SenderName,
			"text":        msg.Message,
			"time":        msg.CreatedAt,
		}
	}

	// Отправляем историю клиенту
	historyMessage := WebSocketMessage{
		Type:     "message_history",
		ParentID: client.ParentID,
		Message:  history,
	}

	select {
	case client.send <- historyMessage:
		log.Printf("История сообщений (%d) отправлена клиенту %s", len(history), client.UserID)
	default:
		log.Printf("Не удалось отправить историю клиенту %s", client.UserID)
	}
}
