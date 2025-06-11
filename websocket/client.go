package websocket

import (
	"PinguinMobile/models"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Время ожидания записи сообщения
	writeWait = 10 * time.Second

	// Время ожидания чтения сообщений от клиента
	pongWait = 60 * time.Second

	// Период отправки пингов - делаем более частыми
	pingPeriod = 2 * time.Second // Изменено с 5 секунд на 2 секунды

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
	hub       *Hub                  // Экспортируемое поле
	conn      *websocket.Conn       // Экспортируемое поле
	UserID    string                // ID пользователя (firebase_uid)
	ParentID  string                // ID семьи
	UserName  string                // имя пользователя
	send      chan WebSocketMessage // Экспортируемое поле
	closed    chan struct{}         // Канал для координации закрытия горутин
	isClosing bool                  // Флаг, указывающий, что клиент закрывается
	mu        sync.Mutex            // Мьютекс для защиты isClosing
}

// NewClient создает нового клиента с правильно инициализированными полями
func NewClient(hub *Hub, conn *websocket.Conn, userID, parentID, userName string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan WebSocketMessage, 256),
		closed:   make(chan struct{}), // Инициализируем канал
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
		c.closeConnection("ReadPump exited") // Используем централизованное закрытие
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(appData string) error {
		log.Printf("[WebSocket] Received pong from client %s", c.UserID)
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Добавляем обработчик закрытия, чтобы видеть код и причину закрытия
	c.conn.SetCloseHandler(func(code int, text string) error {
		log.Printf("[WebSocket] Client %s initiated close: code=%d, text=%s", c.UserID, code, text)
		return nil
	})

	for {
		select {
		case <-c.closed:
			return // Выходим, если соединение закрыто
		default:
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					log.Printf("[WebSocket] Read error: %v", err)
				}
				return
			}

			// Обрабатываем текстовые ping сообщения от клиента
			if messageType == websocket.TextMessage && string(message) == "ping" {
				log.Printf("[WebSocket] Received ping text from client %s", c.UserID)

				// Отправляем текстовый pong
				c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.conn.WriteMessage(websocket.TextMessage, []byte("pong")); err != nil {
					log.Printf("[WebSocket] Error sending pong text: %v", err)
					return
				}

				// Обновляем дедлайн
				c.conn.SetReadDeadline(time.Now().Add(pongWait))
				continue
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
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[PANIC] Recovered in WritePump: %v", r)
		}
		ticker.Stop()
		c.closeConnection("WritePump exited") // Используем централизованное закрытие
	}()

	for {
		select {
		case <-c.closed:
			return // Выходим, если соединение закрыто
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Канал закрыт
				c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Channel closed"))
				return
			}

			// Отправляем сообщение
			err := c.conn.WriteJSON(message)
			if err != nil {
				log.Printf("[WebSocket] Write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			// Отправляем ping
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("[WebSocket] Ping error: %v", err)
				return
			}
			log.Printf("[WebSocket] Ping sent to client %s", c.UserID)
		}
	}
}

// SendMessage отправляет сообщение конкретному клиенту
func (c *Client) SendMessage(message WebSocketMessage) {
	c.send <- message
}

// Добавьте метод для отправки сообщений клиенту
func (c *Client) Send(message WebSocketMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosing {
		log.Printf("[WebSocket] Attempted to send to closed client %s", c.UserID)
		return
	}

	select {
	case c.send <- message:
		// Сообщение успешно отправлено в канал
	default:
		// Если канал полный, закрываем соединение через централизованный метод
		log.Printf("[WebSocket] Buffer full for client %s, closing connection", c.UserID)
		c.isClosing = true
		go c.closeConnection("Send buffer full")
	}
}

// KeepAlive отправляет периодические сообщения для поддержания соединения
// func (c *Client) KeepAlive() {
// 	ticker := time.NewTicker(5 * time.Second)
// 	defer func() {
// 		if r := recover(); r != nil {
// 			log.Printf("[PANIC] Recovered in KeepAlive: %v", r)
// 		}
// 		ticker.Stop()
// 		log.Printf("[WebSocket] KeepAlive routine stopped for client %s", c.UserID)
// 	}()

// 	for {
// 		select {
// 		case <-c.closed:
// 			log.Printf("[WebSocket] KeepAlive terminated for client %s (connection closed)", c.UserID)
// 			return
// 		case <-ticker.C:
// 			// Проверяем, не закрывается ли соединение
// 			c.mu.Lock()
// 			if c.isClosing {
// 				c.mu.Unlock()
// 				return
// 			}
// 			c.mu.Unlock()

// 			// Создаем keep-alive сообщение
// 			keepAliveMsg := WebSocketMessage{
// 				Type:      "keep_alive",
// 				ParentID:  c.ParentID,
// 				SenderID:  "system",
// 				Timestamp: time.Now(),
// 			}

// 			// Безопасно отправляем сообщение
// 			select {
// 			case c.send <- keepAliveMsg:
// 				log.Printf("[WebSocket] Keep-alive sent to client %s", c.UserID)
// 			default:
// 				log.Printf("[WebSocket] Cannot send keep-alive to client %s (buffer full)", c.UserID)
// 				c.closeConnection("Keep-alive send buffer full") // Используем централизованный метод закрытия
// 				return
// 			}
// 		}
// 	}
// }

// closeConnection централизованное закрытие соединения
func (c *Client) closeConnection(reason string) {
	c.mu.Lock()
	if c.isClosing {
		c.mu.Unlock()
		return // Уже закрывается
	}
	c.isClosing = true
	c.mu.Unlock()

	log.Printf("[WebSocket] Closing connection for client %s. Reason: %s", c.UserID, reason)

	// Отправляем закрывающее сообщение клиенту
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server closing connection")
	// Игнорируем ошибку, так как соединение может быть уже закрыто
	c.conn.WriteMessage(websocket.CloseMessage, closeMsg)

	// Сигнализируем всем горутинам о закрытии
	close(c.closed)

	// Отменяем регистрацию клиента в хабе
	c.hub.UnregisterClient(c)
}

// Удаляем ServeWs отсюда, так как эта функциональность должна быть в контроллере
