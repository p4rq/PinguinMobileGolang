package websocket

import (
	"PinguinMobile/models"
	"fmt"
	"log"
	"strings"
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
		_, isString := message.Message.(string)
		isSystemMessage := isString && (message.SenderID == "system" || message.SenderID == "Система")
		isJoinMessage := isSystemMessage && strings.Contains(fmt.Sprintf("%v", message.Message), "присоединился к чату")

		// Пропускаем сообщения о присоединении к чату
		if isJoinMessage {
			// Просто логируем, но не отправляем и не сохраняем
			log.Printf("[WebSocket] Skipping join message: %v", message.Message)
			return // Полностью прерываем обработку сообщения о присоединении
		}

		// Отправляем уведомление только для не-системных сообщений или системных, но не о присоединении
		if !isSystemMessage || (isSystemMessage && !isJoinMessage) {
			// Запускаем отправку в отдельной горутине
			go func() {
				// Дополнительные данные для FCM
				data := map[string]string{
					"type":          "chat_message",
					"message":       fmt.Sprintf("%v", message.Message),
					"sender_id":     message.SenderID,
					"sender_type":   h.determineUserType(message.SenderID), // Добавляем тип отправителя
					"receiver_type": "all",                                 // Указываем, что получатель - все (родители и дети)
				}

				log.Printf("[WebSocket] Sending push notification for message from user %s (type: %s)",
					message.SenderID, data["sender_type"])

				// Отправляем уведомления ВСЕМ членам семьи, кроме отправителя
				if err := h.NotifySrv.SendNotificationToFamily(
					message.ParentID,
					"Новое сообщение",
					fmt.Sprintf("%s: %v", message.SenderName, message.Message),
					data,
					message.SenderID, // Исключаем отправителя
				); err != nil {
					log.Printf("[WebSocket] Error sending push notification: %v", err)
				}
			}()
		} else {
			log.Printf("[WebSocket] Skipping push notification for system join message: %v", message.Message)
		}
	}
}

// determineUserType определяет тип пользователя по ID
func (h *Hub) determineUserType(userID string) string {
	// Проверяем, что есть доступ к базе данных
	if h.db == nil {
		log.Printf("[WebSocket] Database is nil, cannot determine user type")
		return "unknown"
	}

	// Проверяем существование в таблице родителей
	var parentCount int64
	h.db.Model(&models.Parent{}).Where("firebase_uid = ?", userID).Count(&parentCount)
	if parentCount > 0 {
		return "parent"
	}

	// Проверяем существование в таблице детей
	var childCount int64
	h.db.Model(&models.Child{}).Where("firebase_uid = ?", userID).Count(&childCount)
	if childCount > 0 {
		return "child"
	}

	return "unknown"
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

// NotifyLimitChange улучшенная версия для отправки уведомлений о смене лимитов
func (h *Hub) NotifyLimitChange(parentID string, childToken string) {
	// Создаем улучшенное сообщение для WebSocket
	chatMessage := WebSocketMessage{
		Type:          "chat_message", // Клиенты обычно обрабатывают этот тип
		ParentID:      parentID,
		ChildToken:    childToken,
		IsChangeLimit: true, // Флаг для отличия от обычных сообщений
		Timestamp:     time.Now(),
		SenderID:      "system",
		SenderName:    "Система",
		Message:       "Настройки лимитов были обновлены",
	}

	limitMessage := WebSocketMessage{
		Type:          "limit_change", // Специализированный тип для лимитов
		ParentID:      parentID,
		ChildToken:    childToken,
		IsChangeLimit: true,
		Timestamp:     time.Now(),
		SenderID:      "system",
		SenderName:    "Система",
		Message:       "Настройки лимитов были обновлены",
	}

	log.Printf("[WebSocket] Processing limit change notification for parent %s, child token %s",
		parentID, childToken)

	// 1. Отправляем WebSocket сообщения родителям
	h.mu.Lock()
	clients, ok := h.clients[parentID]
	if !ok || len(clients) == 0 {
		log.Printf("[WebSocket] No parent WebSocket clients found for family %s", parentID)
		h.mu.Unlock() // Разблокируем мьютекс если клиентов нет
	} else {
		// Копируем клиентов для безопасной итерации
		clientsCopy := make([]*Client, 0, len(clients))
		for client := range clients {
			clientsCopy = append(clientsCopy, client)
		}
		h.mu.Unlock() // Разблокируем мьютекс после копирования клиентов

		// Отправляем уведомление каждому клиенту-родителю
		for _, client := range clientsCopy {
			log.Printf("[WebSocket] Sending limit change notification to client %s", client.UserID)
			client.Send(chatMessage)
			client.Send(limitMessage) // Тип limit_change

		}
	}

	// 2. Отправляем прямое уведомление ребенку
	h.SendNotificationToChild(childToken, parentID)

	// 3. Добавляем сообщение в чат для всей семьи
	h.addLimitChangeMessageToChat(parentID, childToken)

	// 4. Отправляем Push уведомления через FCM всей семье
	if h.NotifySrv != nil {
		go func() {
			data := map[string]string{
				"type":            "limit_change",
				"child_token":     childToken,
				"is_change_limit": "true",
			}

			// Отправляем уведомления всем членам семьи
			if err := h.NotifySrv.SendNotificationToFamily(
				parentID,
				"Изменение лимитов",
				"Были изменены настройки лимитов для ребенка",
				data,
			); err != nil {
				log.Printf("[WebSocket] Error sending limit change push notification: %v", err)
			} else {
				log.Printf("[WebSocket] Successfully sent limit change push notifications to family %s", parentID)
			}
		}()
	} else {
		log.Printf("[WebSocket] NotificationService is nil, cannot send limit change push notifications")
	}
}

// SendNotificationToChild отправляет уведомление ребенку по токену устройства
func (h *Hub) SendNotificationToChild(childToken string, parentID string) {
	if childToken == "" {
		log.Printf("[WebSocket] Child token is empty, cannot send direct notification")
		return
	}

	if h.NotifySrv == nil {
		log.Printf("[WebSocket] NotificationService is nil, cannot send direct notification to child")
		return
	}

	if h.db == nil {
		log.Printf("[WebSocket] Database is nil, cannot find child information")
		return
	}

	// Найдем ребенка с данным токеном устройства
	var child models.Child
	result := h.db.Where("device_token = ?", childToken).First(&child)
	if result.Error != nil {
		log.Printf("[WebSocket] Error finding child with token %s: %v",
			childToken, result.Error)
		return
	}

	// Отправляем прямое уведомление через FCM
	data := map[string]string{
		"type":            "limit_change",
		"child_token":     childToken,
		"is_change_limit": "true",
		"parent_id":       parentID,
	}

	err := h.NotifySrv.SendNotification(
		childToken,
		"Обновление настроек",
		"Ваши настройки лимитов были обновлены",
		data,
		child.Lang) // Используем язык ребенка для локализации

	if err != nil {
		log.Printf("[WebSocket] Error sending direct notification to child: %v", err)
	} else {
		log.Printf("[WebSocket] Successfully sent direct notification to child %s", child.FirebaseUID)
	}
}

// addLimitChangeMessageToChat добавляет сообщение об изменении лимитов в чат
func (h *Hub) addLimitChangeMessageToChat(parentID string, childToken string) {
	log.Printf("[DEBUG] addLimitChangeMessageToChat начал выполнение: parentID=%s, childToken=%s", parentID, childToken)

	if h.MessageService == nil {
		log.Printf("[WebSocket] MessageService is nil, cannot add limit change message to chat")
		return
	}

	if h.db == nil {
		log.Printf("[WebSocket] Database is nil, cannot find child information")
		return
	}

	// Найдем ребенка с данным токеном устройства
	var child models.Child
	result := h.db.Where("device_token = ?", childToken).First(&child)
	if result.Error != nil {
		log.Printf("[WebSocket] Error finding child for chat message: %v", result.Error)

		// Пробуем найти ребенка по частичному токену (первые 20 символов)
		if len(childToken) > 20 {
			shortToken := childToken[:20]
			log.Printf("[WebSocket] Trying to find child with partial token: %s...", shortToken)
			result = h.db.Where("device_token LIKE ?", shortToken+"%").First(&child)
			if result.Error != nil {
				log.Printf("[WebSocket] Still cannot find child with partial token: %v", result.Error)

				// Если не нашли, создаем сообщение без имени ребенка
				chatMessage := models.ChatMessage{
					ParentID:   parentID,
					SenderID:   "system",
					SenderName: "Система",
					Message:    fmt.Sprintf("Обновлены настройки лимитов для ребенка (токен: %s...)", childToken[:10]),
				}

				if err := h.MessageService.SaveMessage(&chatMessage); err != nil {
					log.Printf("[WebSocket] Error saving limit change chat message: %v", err)
					return
				}

				// Отправляем сообщение всем клиентам
				wsMessage := WebSocketMessage{
					Type:       "chat_message",
					ParentID:   parentID,
					SenderID:   chatMessage.SenderID,
					SenderName: chatMessage.SenderName,
					Message:    chatMessage.Message,
					Timestamp:  time.Now(),
					ChildToken: childToken, // Добавляем токен ребенка в сообщение
				}

				h.BroadcastMessage(wsMessage)
				return
			}
		} else {
			return
		}
	}

	log.Printf("[DEBUG] Найден ребенок %s (ID: %d) с токеном %s",
		child.Name, child.ID, childToken)

	// Получаем имя ребенка, используя пустую строку, если оно не задано
	childName := child.Name
	if childName == "" {
		childName = "Без имени"
	}

	// Создаем сообщение для чата с именем ребенка
	chatMessage := models.ChatMessage{
		ParentID:   parentID,
		SenderID:   "system",
		SenderName: "Система",
		Message:    fmt.Sprintf("Обновлены настройки лимитов для ребенка %s", childName),
	}

	// Сохраняем в базу данных
	if err := h.MessageService.SaveMessage(&chatMessage); err != nil {
		log.Printf("[WebSocket] Error saving limit change chat message: %v", err)
		return
	}

	log.Printf("[DEBUG] Сообщение сохранено в базу данных")

	// Отправляем сообщение всем клиентам
	wsMessage := WebSocketMessage{
		Type:       "chat_message",
		ParentID:   parentID,
		SenderID:   chatMessage.SenderID,
		SenderName: chatMessage.SenderName,
		Message:    chatMessage.Message,
		Timestamp:  time.Now(),
		ChildToken: childToken, // Добавляем токен ребенка в сообщение
	}

	log.Printf("[DEBUG] Отправляем сообщение через BroadcastMessage с токеном ребенка")
	h.BroadcastMessage(wsMessage)

	log.Printf("[WebSocket] Added limit change message to chat for family %s", parentID)
}
