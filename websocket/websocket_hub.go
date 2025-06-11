package websocket

import (
	"PinguinMobile/models"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Определение интерфейса NotificationService
type NotificationService interface {
	SendNotificationToFamily(parentID, title, body string, data map[string]string, skipUsers ...string) error
	SendNotification(token, title, body string, data map[string]string, lang string) error
}

// WebSocketMessage упрощенная структура для сообщений
type WebSocketMessage struct {
	Type          string      `json:"type"`                    // "chat_message" или "message_history"
	ParentID      string      `json:"parent_id"`               // ID родителя/семьи
	SenderID      string      `json:"sender_id"`               // ID отправителя
	SenderName    string      `json:"sender_name,omitempty"`   // Имя отправителя
	Message       interface{} `json:"message"`                 // Содержимое сообщения или массив сообщений для history
	Timestamp     time.Time   `json:"timestamp"`               // Время отправки
	ChildToken    string      `json:"child_token,omitempty"`   // Токен устройства ребенка
	IsChangeLimit bool        `json:"isChangeLimit,omitempty"` // Флаг изменения лимитов
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

	// Сервис для отправки уведомлений - заменить на интерфейс
	NotifySrv NotificationService
}

// ChatMessageService интерфейс для работы с сообщениями чата
type ChatMessageService interface {
	SaveMessage(message *models.ChatMessage) error
	GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error)
}

// NewHub создает новый хаб
func NewHub(messageService ChatMessageService, notifySrv NotificationService, db *gorm.DB) *Hub {
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
	// Логирование для отладки
	log.Printf("[WebSocket] Broadcasting message: type=%s, sender=%s", message.Type, message.SenderID)

	// Захватываем блокировку перед доступом к clients
	h.mu.Lock()

	// Проверяем наличие клиентов для указанного parent_id
	clients, ok := h.clients[message.ParentID]
	if !ok || len(clients) == 0 {
		log.Printf("[WebSocket] No clients found for family %s", message.ParentID)
		h.mu.Unlock() // Важно: разблокируем мьютекс перед выходом из функции
		return
	}

	// Копируем клиентов, чтобы избежать проблем с конкурентным доступом
	clientsCopy := make([]*Client, 0, len(clients))
	for client := range clients {
		clientsCopy = append(clientsCopy, client)
	}

	// Разблокируем мьютекс после копирования клиентов
	h.mu.Unlock()

	// Отправляем сообщение каждому клиенту из копии
	for _, client := range clientsCopy {
		log.Printf("[WebSocket] Sending message to client %s", client.UserID)
		client.Send(message)
	}

	// Дополнительная отправка уведомления, если это сообщение чата
	if message.Type == "chat_message" && h.NotifySrv != nil {
		// Запускаем отправку в отдельной горутине, так как она может быть длительной
		go func() {
			data := map[string]string{
				"type":      "chat_message",
				"message":   fmt.Sprintf("%v", message.Message),
				"sender_id": message.SenderID,
			}

			log.Printf("[WebSocket] Sending push notification for chat message to family %s", message.ParentID)

			// Не передаем message.SenderID в skipUsers, чтобы отправитель тоже получил уведомление
			if err := h.NotifySrv.SendNotificationToFamily(
				message.ParentID,
				"Новое сообщение",
				fmt.Sprintf("%s: %v", message.SenderName, message.Message),
				data,
			); err != nil {
				log.Printf("[WebSocket] Error sending push notification: %v", err)
			}
		}()
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
func (h *Hub) NotifyLimitChange(parentID string, childToken string) {
	message := WebSocketMessage{
		Type:          "limit_change",
		ParentID:      parentID,
		ChildToken:    childToken,
		IsChangeLimit: true,
		Timestamp:     time.Now(),
	}

	// Отправляем всем клиентам с тем же parentID
	h.mu.Lock()
	defer h.mu.Unlock()

	// Исправьте итерацию по клиентам
	if clients, ok := h.clients[parentID]; ok {
		for client := range clients {
			select {
			case client.send <- message:
				log.Printf("[WEBSOCKET] Отправлено уведомление об изменении лимитов клиенту %s", client.UserID)
			default:
				log.Printf("[WEBSOCKET] Не удалось отправить уведомление клиенту %s", client.UserID)
				h.unregisterClient(client)
			}
		}
	}
}
