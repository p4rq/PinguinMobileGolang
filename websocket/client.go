package websocket

import (
	"PinguinMobile/models"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Время ожидания записи сообщения
	writeWait = 10 * time.Second

	// Время ожидания чтения сообщений от клиента
	pongWait = 60 * time.Second

	// Период отправки пингов - сделаем 5 секунд, чтобы уложиться в 10-секундный интервал
	pingPeriod = 5 * time.Second

	// Максимальный размер входящего сообщения
	maxMessageSize = 1024 * 16 // Увеличим до 16KB для надежности
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем все origins для упрощения разработки
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client представляет собой соединение WebSocket с экспортируемыми полями
type Client struct {
	hub      *Hub                  // Экспортируемое поле
	conn     *websocket.Conn       // Экспортируемое поле
	UserID   string                // ID пользователя (firebase_uid)
	ParentID string                // ID семьи
	UserName string                // имя пользователя
	send     chan WebSocketMessage // Экспортируемое поле
}

// NewClient создает нового клиента с правильно инициализированными полями
func NewClient(hub *Hub, conn *websocket.Conn, userID, parentID, userName string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan WebSocketMessage, 256),
		UserID:   userID,
		ParentID: parentID,
		UserName: userName,
	}
}

// ReadPump обрабатывает входящие сообщения от клиента
func (c *Client) ReadPump() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Recovered in ReadPump: %v", r)
		}
		c.hub.UnregisterClient(c)
		c.conn.Close()
		log.Printf("[WebSocket] Соединение закрыто для пользователя %s", c.UserID)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	// Улучшенная обработка pong с логированием
	c.conn.SetPongHandler(func(appData string) error {
		log.Printf("[WebSocket] Received pong from client %s", c.UserID)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Добавление обработчика закрытия соединения для логирования кода закрытия
	c.conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("[WebSocket] Connection closed for user %s with code %d: %s",
			c.UserID, code, text)
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Ошибка при чтении сообщения: %v", err)
			}
			break
		}

		// Разбор сообщения из JSON
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Ошибка при разборе JSON: %v", err)
			continue
		}

		// Проверка наличия поля message
		messageText, ok := msg["message"].(string)
		if !ok {
			log.Printf("Ошибка: поле message отсутствует в полезной нагрузке")
			continue
		}

		// Создаем новое сообщение чата
		chatMessage := &models.ChatMessage{
			ParentID:   c.ParentID,
			SenderID:   c.UserID,
			SenderName: c.UserName,
			Message:    messageText,
			CreatedAt:  time.Now(),
		}

		// Сохраняем сообщение в БД через сервис сообщений
		if c.hub.MessageService != nil {
			if err := c.hub.MessageService.SaveMessage(chatMessage); err != nil {
				log.Printf("Ошибка при сохранении сообщения в БД: %v", err)
				continue // Добавляем continue, чтобы не отправлять сообщение в случае ошибки
			} else {
				log.Printf("Message successfully saved to database with ID=%v", chatMessage.ID)
			}
		} else {
			log.Printf("WARNING: MessageService is nil, cannot save message")
			continue // Также добавляем continue
		}

		// Создаем WebSocket сообщение с ID из сохраненного сообщения
		wsMessage := WebSocketMessage{
			Type:       "chat_message",
			ParentID:   c.ParentID,
			SenderID:   c.UserID,
			SenderName: c.UserName,
			Message:    messageText,
			Timestamp:  time.Now(),
			// Добавляем ID сообщения
		}

		// Отправляем сообщение всем клиентам через хаб
		c.hub.broadcast <- wsMessage
	}
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Recovered in WritePump: %v", r)
		}
		ticker.Stop()
		c.conn.Close()
		log.Printf("[WebSocket] WritePump завершен для пользователя %s", c.UserID)
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Канал закрыт, отправляем сообщение о закрытии
				log.Printf("[WebSocket] Send channel closed for user %s, closing connection", c.UserID)
				c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}

			// Отправляем сообщение клиенту с подробным логированием
			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("[WebSocket] Error writing message to client %s: %v", c.UserID, err)
				return
			}
			log.Printf("[WebSocket] Message sent to client %s: %s", c.UserID, message.Type)

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			log.Printf("[WebSocket] Sending ping to client %s", c.UserID)

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[WebSocket] Error sending ping to client %s: %v", c.UserID, err)
				return
			}
			log.Printf("[WebSocket] Ping successfully sent to client %s", c.UserID)
		}
	}
}

// SendMessage отправляет сообщение конкретному клиенту
func (c *Client) SendMessage(message WebSocketMessage) {
	c.send <- message
}

// Добавьте метод для отправки сообщений клиенту
func (c *Client) Send(message WebSocketMessage) {
	select {
	case c.send <- message:
		// Сообщение успешно отправлено в канал
	default:
		// Если канал полный, закрываем соединение
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}
}

// Удаляем ServeWs отсюда, так как эта функциональность должна быть в контроллере
