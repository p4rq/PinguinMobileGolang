package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub *Hub

	// User ID for this connection
	UserID string

	// Parent ID for routing messages
	ParentID string

	// The websocket connection.
	Conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// NewClient создает нового клиента WebSocket
func NewClient(hub *Hub, userID string, parentID string, conn *websocket.Conn) *Client {
	return &Client{
		Hub:      hub,
		UserID:   userID,
		ParentID: parentID,
		Conn:     conn,
		send:     make(chan []byte, 256),
	}
}

// ReadPump pumps messages from the websocket connection to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, messageData, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Разбор JSON сообщения
		var msg map[string]interface{}
		if err := json.Unmarshal(messageData, &msg); err != nil {
			log.Printf("error parsing message: %v", err)
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			log.Printf("message type not specified")
			continue
		}

		// Обработка разных типов сообщений
		switch msgType {
		case "chat":
			// Обработка сообщения чата
			parentID, ok := msg["parent_id"].(string)
			if !ok {
				log.Printf("parent_id not specified in chat message")
				continue
			}

			messageText, _ := msg["message"].(string)
			channel, _ := msg["channel"].(string)
			isPrivate, _ := msg["is_private"].(bool)
			recipientID, _ := msg["recipient_id"].(string)

			// Определяем, куда отправлять сообщение
			targetID := parentID
			if isPrivate && recipientID != "" {
				// Для приватных сообщений отправляем только конкретным пользователям
				// Используем специальное форматирование ID для личных чатов
				targetID = "private_" + parentID + "_" + recipientID
			}

			// Создаем сообщение для широковещательной рассылки
			chatMsg := struct {
				Type        string `json:"type"`
				SenderID    string `json:"sender_id"`
				SenderName  string `json:"sender_name"`
				ParentID    string `json:"parent_id"`
				RecipientID string `json:"recipient_id,omitempty"`
				IsPrivate   bool   `json:"is_private"`
				Channel     string `json:"channel,omitempty"`
				Message     string `json:"message"`
				Timestamp   int64  `json:"timestamp"`
			}{
				Type:        "chat",
				SenderID:    c.UserID,
				SenderName:  "User", // Здесь нужно добавить получение имени пользователя
				ParentID:    parentID,
				RecipientID: recipientID,
				IsPrivate:   isPrivate,
				Channel:     channel,
				Message:     messageText,
				Timestamp:   time.Now().Unix(),
			}

			// Преобразуем обратно в JSON и отправляем
			broadcastMsg, err := json.Marshal(chatMsg)
			if err != nil {
				log.Printf("error marshaling broadcast message: %v", err)
				continue
			}

			c.Hub.Broadcast(&Message{ParentID: targetID, Data: broadcastMsg})

		case "auth":
			// Аутентификация может быть выполнена здесь,
			// но уже должна быть обработана middleware
			log.Printf("auth message received from %s", c.UserID)

		default:
			log.Printf("unknown message type: %s", msgType)
		}
	}
}

// WritePump pumps messages from the hub to the websocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
