package websocket

import (
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	// Registered clients by parent ID (firebase_uid of parent)
	clients map[string]map[*Client]bool

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Send message to specific family
	broadcast chan *Message

	// Mutex for thread-safe operations
	mu sync.Mutex
}

type Message struct {
	ParentID string
	Data     []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *Message),
	}
}

// Register регистрирует нового клиента в хабе
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister отменяет регистрацию клиента
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast отправляет сообщение всем клиентам семьи
func (h *Hub) Broadcast(message *Message) {
	h.broadcast <- message
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.ParentID]; !ok {
				h.clients[client.ParentID] = make(map[*Client]bool)
			}
			h.clients[client.ParentID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ParentID]; ok {
				delete(h.clients[client.ParentID], client)
				close(client.send)
				if len(h.clients[client.ParentID]) == 0 {
					delete(h.clients, client.ParentID)
				}
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			if clients, ok := h.clients[message.ParentID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Data:
					default:
						close(client.send)
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.clients, message.ParentID)
						}
					}
				}
			}
			h.mu.Unlock()
		}
	}
}
