package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Время ожидания для записи сообщения клиенту
	writeWait = 10 * time.Second

	// Время ожидания для чтения следующего pong от клиента
	pongWait = 60 * time.Second

	// Период отправки ping-сообщений клиенту
	pingPeriod = (pongWait * 9) / 10

	// Максимальный размер сообщения, разрешенного от клиента
	maxMessageSize = 10240
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем все origins для упрощения разработки
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client представляет собой соединение WebSocket
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan WebSocketMessage
	userID   string // ID пользователя (firebase_uid)
	parentID string // ID семьи
	userType string // тип пользователя (parent/child)
}

// readPump обрабатывает входящие сообщения от клиента
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
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
				log.Printf("Error reading from WebSocket: %v", err)
			}
			break
		}

		var msg WebSocketMessage
		// Обработка сообщения
		err = json.Unmarshal(message, &msg)
		if err != nil {
			log.Printf("Error unmarshalling message: %v", err)
			continue
		}

		// Обогащаем сообщение данными отправителя
		msg.SenderID = c.userID
		msg.SenderType = c.userType
		msg.ParentID = c.parentID

		c.hub.broadcast <- msg
	}
}

// writePump отправляет сообщения клиенту
func (c *Client) writePump() {
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
				// Hub закрыл канал
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteJSON(message)
			if err != nil {
				log.Printf("Error writing message: %v", err)
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

// ping отправляет ping-сообщения клиенту
func (c *Client) ping() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.conn.WriteControl(
				websocket.PingMessage,
				[]byte{},
				time.Now().Add(10*time.Second)); err != nil {
				log.Printf("Failed to send ping: %v", err)
				return
			}
		}
	}
}

// ServeWs обрабатывает WebSocket запрос от клиента
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID, parentID, userType string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// При установлении соединения
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	// Установите лимит на размер сообщения
	conn.SetReadLimit(512 * 1024) // 512KB максимальный размер сообщения

	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan WebSocketMessage, 256),
		userID:   userID,
		parentID: parentID,
		userType: userType,
	}

	client.hub.register <- client

	// Разрешаем коллекции горутин на сохранение буфера после того, как функция вернется
	go client.writePump() // Отправляет сообщения клиенту
	go client.readPump()  // Читает сообщения от клиента
	go client.ping()
}
