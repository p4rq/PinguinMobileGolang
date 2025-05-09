package websocket

import (
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
	hub *Hub

	// WebSocket соединение
	conn *websocket.Conn

	// Буферизованный канал для исходящих сообщений
	send chan WebSocketMessage

	// Идентификаторы для маршрутизации сообщений
	userID   string
	parentID string
	userType string
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
		var message WebSocketMessage
		err := c.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Обогащаем сообщение данными отправителя
		message.SenderID = c.userID
		message.SenderType = c.userType
		message.ParentID = c.parentID

		c.hub.broadcast <- message
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

// ServeWs обрабатывает WebSocket запрос от клиента
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID, parentID, userType string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

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
	go client.writePump()
	go client.readPump()
}
