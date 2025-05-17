package websocket

import (
	"PinguinMobile/models"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
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

// FamilyMember представляет члена семьи для WebSocket
type FamilyMember struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "parent" или "child"
	ParentID string `json:"parent_id"`
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

	// Для хранения истории сообщений по parent_id
	messageHistory   map[string][]WebSocketMessage
	historyMaxSize   int
	messageHistoryMu sync.Mutex

	// Кэш для информации о семьях
	familyMembers   map[string][]FamilyMember // key: parent_id, value: члены семьи
	familyMembersMu sync.Mutex

	// Сервисы
	messageService ChatMessageService
	db             *gorm.DB // Для доступа к базе данных
}

// ChatMessageService интерфейс для работы с сообщениями чата
type ChatMessageService interface {
	SaveMessage(message *models.ChatMessage) error
	GetMessages(parentID string, limit int) ([]*models.ChatMessage, error)
}

// NewHub создает новый хаб
func NewHub(messageService ChatMessageService, db *gorm.DB) *Hub {
	return &Hub{
		clients:        make(map[string]map[*Client]bool),
		broadcast:      make(chan WebSocketMessage),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		messageHistory: make(map[string][]WebSocketMessage),
		historyMaxSize: 100,
		messageService: messageService,
		familyMembers:  make(map[string][]FamilyMember),
		db:             db,
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
			h.mu.Unlock()

			// Загружаем данные о членах семьи, если еще не загружены
			h.loadFamilyMembers(client.parentID)

			// Отправляем историю новому клиенту
			go h.sendMessageHistory(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.parentID]; ok {
				delete(h.clients[client.parentID], client)
				close(client.send)
				if len(h.clients[client.parentID]) == 0 {
					delete(h.clients, client.parentID)
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			clients, ok := h.clients[message.ParentID]
			h.mu.Unlock()

			if ok {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						h.mu.Lock()
						delete(h.clients[message.ParentID], client)
						close(client.send)
						if len(h.clients[message.ParentID]) == 0 {
							delete(h.clients, message.ParentID)
						}
						h.mu.Unlock()
					}
				}
			}

			// Сохраняем сообщение в истории
			if message.Type == "chat_message" || strings.HasPrefix(message.Type, "system_") {
				h.saveMessageToHistory(message)
			}
		}
	}
}

// Упрощенная функция BroadcastChatMessage
func (h *Hub) BroadcastChatMessage(message *models.ChatMessage) {
	// Создаем упрощенное WebSocket сообщение
	wsMessage := WebSocketMessage{
		Type:     "chat_message",
		ParentID: message.ParentID,
		SenderID: message.SenderID,
		Message: map[string]interface{}{
			"text": message.Message,
			// Добавляем только необходимую информацию
			"message_id": message.ID,
			// Если есть медиа, добавляем ссылку
			"media_url": message.MediaURL,
		},
	}

	// Отправляем сообщение всем
	h.broadcast <- wsMessage
	h.saveMessageToHistory(wsMessage)
}

// loadFamilyMembers загружает информацию о членах семьи из базы данных
func (h *Hub) loadFamilyMembers(parentID string) {
	h.familyMembersMu.Lock()
	defer h.familyMembersMu.Unlock()

	// Проверяем, есть ли уже данные о семье в кэше
	if _, exists := h.familyMembers[parentID]; exists {
		return
	}

	members := []FamilyMember{}

	// 1. Загружаем основного родителя
	var parent models.Parent
	if err := h.db.Where("firebase_uid = ?", parentID).First(&parent).Error; err != nil {
		log.Printf("Error loading parent %s: %v", parentID, err)
		return
	}

	// Добавляем родителя в список членов семьи
	members = append(members, FamilyMember{
		ID:       parent.FirebaseUID,
		Name:     parent.Name,
		Type:     "parent",
		ParentID: parent.FirebaseUID,
	})

	// 2. Загружаем других родителей из той же семьи (если есть)
	var otherParents []models.Parent
	if err := h.db.Where("family = ? AND firebase_uid != ?", parent.Family, parentID).Find(&otherParents).Error; err != nil {
		log.Printf("Error loading other parents: %v", err)
	} else {
		for _, op := range otherParents {
			members = append(members, FamilyMember{
				ID:       op.FirebaseUID,
				Name:     op.Name,
				Type:     "parent",
				ParentID: parentID,
			})
		}
	}

	// 3. Загружаем детей, привязанных к родителю
	var children []models.Child
	if err := h.db.Where("parent_id = ?", parentID).Find(&children).Error; err != nil {
		log.Printf("Error loading children: %v", err)
	} else {
		for _, child := range children {
			members = append(members, FamilyMember{
				ID:       child.FirebaseUID,
				Name:     child.Name,
				Type:     "child",
				ParentID: parentID,
			})
		}
	}

	// Сохраняем список членов семьи в кэше
	h.familyMembers[parentID] = members
	log.Printf("Loaded %d family members for parent %s", len(members), parentID)
}

// getFamilyMember возвращает информацию о члене семьи по его ID
func (h *Hub) getFamilyMember(parentID, memberID string) *FamilyMember {
	h.familyMembersMu.Lock()
	defer h.familyMembersMu.Unlock()

	members, exists := h.familyMembers[parentID]
	if !exists {
		return nil
	}

	for i := range members {
		if members[i].ID == memberID {
			return &members[i]
		}
	}

	return nil
}

// saveMessageToHistory сохраняет сообщение в истории и в БД
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

	// Сохранение сообщения в БД для семейного чата
	if message.Type == "chat_message" && h.messageService != nil {
		// Проверяем, что сообщение содержит текст
		if msg, ok := message.Message.(map[string]interface{}); ok {
			if text, exists := msg["text"].(string); exists {
				// Получаем информацию об отправителе
				senderName := ""
				senderType := ""

				// Проверяем есть ли в сообщении sender_name
				if name, ok := msg["sender_name"].(string); ok && name != "" {
					senderName = name
				} else {
					// Ищем отправителя среди членов семьи
					if member := h.getFamilyMember(message.ParentID, message.SenderID); member != nil {
						senderName = member.Name
						senderType = member.Type
					}
				}

				// Если тип отправителя не определен, определяем его
				if senderType == "" {
					senderType = message.SenderType
					if senderType == "" {
						senderType = getSenderType(message.SenderID, message.ParentID)
					}
				}

				// Базовая информация о сообщении для семейного чата
				chatMessage := &models.ChatMessage{
					ParentID:    message.ParentID,
					SenderID:    message.SenderID,
					Message:     text,
					CreatedAt:   time.Now(),
					SenderName:  senderName,
					SenderType:  senderType,
					MessageType: "text", // По умолчанию текст
					Channel:     getDefaultChannel(message.Channel),
					IsPrivate:   false, // Всегда публичное для семейного чата
					IsRead:      false, // По умолчанию не прочитано
				}

				err := h.messageService.SaveMessage(chatMessage)
				if err != nil {
					log.Printf("Error saving message to database: %v", err)
				}
			}
		}
	}
}

// getDefaultChannel возвращает канал по умолчанию, если не указан
func getDefaultChannel(channel string) string {
	if channel != "" {
		return channel
	}
	return "general" // Канал по умолчанию
}

// getSenderType определяет тип отправителя
func getSenderType(senderID, parentID string) string {
	if senderID == parentID {
		return "parent"
	}
	return "child"
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

// Упрощенная отправка истории
func (h *Hub) sendMessageHistory(client *Client) {
	log.Printf("Getting message history for parent_id: %s", client.parentID)

	// Проверяем сначала кэш в памяти
	cachedHistory := h.getMessageHistory(client.parentID)
	if len(cachedHistory) > 0 {
		log.Printf("Found %d cached messages for parent_id: %s", len(cachedHistory), client.parentID)

		// Отправляем историю из кэша
		client.send <- WebSocketMessage{
			Type:     "message_history",
			ParentID: client.parentID,
			Message:  cachedHistory,
		}
		return
	}

	// Если кэш пуст, обращаемся к БД
	var messages []*models.ChatMessage
	var err error

	if h.messageService != nil {
		log.Printf("Loading messages from DB for parent_id: %s", client.parentID)
		messages, err = h.messageService.GetMessages(client.parentID, 30)
		if err != nil {
			log.Printf("Error loading messages from DB: %v", err)
			// Отправляем пустой массив и ошибку
			client.send <- WebSocketMessage{
				Type:     "message_history",
				ParentID: client.parentID,
				Message:  []interface{}{},
			}
			return
		}
	}

	log.Printf("Found %d messages in DB for parent_id: %s", len(messages), client.parentID)

	// Даже если нет сообщений, отправляем пустой массив
	simpleHistory := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		simpleHistory[i] = map[string]interface{}{
			"sender_id":   msg.SenderID,
			"text":        msg.Message,
			"time":        msg.CreatedAt,
			"sender_name": msg.SenderName,
		}
	}

	// Отправляем результат с безопасной обработкой ошибок
	select {
	case client.send <- WebSocketMessage{
		Type:     "message_history",
		ParentID: client.parentID,
		Message:  simpleHistory,
	}:
		log.Printf("Successfully sent history (%d items) to client", len(simpleHistory))
	default:
		log.Printf("Failed to send history to client: channel blocked or closed")
	}
}
