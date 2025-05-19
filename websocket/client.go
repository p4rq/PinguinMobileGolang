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

	// Период отправки пингов
	pingPeriod = (pongWait * 9) / 10

	// Максимальный размер входящего сообщения
	maxMessageSize = 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем все origins для упрощения разработки
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client представляет собой соединение WebSocket с экспортируемыми полями
type Client struct {
	hub      *Hub                  // Экспортируем поле для доступа из других пакетов
	conn     *websocket.Conn       // Экспортируем поле
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
		c.hub.UnregisterClient(c)
		c.conn.Close()
		log.Printf("WebSocket соединение закрыто для пользователя %s", c.UserID)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
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

// WritePump отправляет сообщения клиенту (экспортируемый метод)
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Канал закрыт
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Отправляем сообщение клиенту
			if err := c.conn.WriteJSON(message); err != nil {
				log.Printf("Ошибка при отправке сообщения: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Ping отправляет ping-сообщения клиенту
func (c *Client) Ping() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.conn.WriteControl(
				websocket.PingMessage,
				[]byte{},
				time.Now().Add(10*time.Second)); err != nil {
				log.Printf("Не удалось отправить ping: %v", err)
				return
			}
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
