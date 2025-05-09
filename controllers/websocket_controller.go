package controllers

import (
	"PinguinMobile/services"
	ws "PinguinMobile/websocket" // Используем алиас ws для пакета websocket
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket" // Импорт gorilla/websocket для Upgrader
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем все источники для примера
	},
}

var hub *ws.Hub
var wsChatService *services.ChatService

func InitWebsocket(chatService *services.ChatService) {
	wsChatService = chatService
	hub = ws.NewHub()
	go hub.Run()
}

// ServeWs обрабатывает WebSocket запросы от клиентов
func ServeWs(c *gin.Context) {
	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	parentID := c.Query("parent_id")
	if parentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parent_id is required"})
		return
	}

	// Для приватных чатов можно указать recipient_id
	recipientID := c.Query("recipient_id")
	isPrivate := recipientID != ""

	// Целевой ID для подключения
	targetID := parentID
	if isPrivate {
		targetID = "private_" + parentID + "_" + recipientID
	}

	// Проверяем, что пользователь имеет доступ к этой семье
	userType, _ := c.Get("user_type")
	isParent := userType == "parent"

	if isParent {
		// Если родитель, то должен быть владельцем семьи
		if userID.(string) != parentID {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied to this family"})
			return
		}
	} else {
		// Если ребенок, то проверяем принадлежность к семье
		inFamily, err := isChildBelongsToFamily(wsChatService, userID.(string), parentID)
		if err != nil || !inFamily {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied to this family"})
			return
		}
	}

	// Если это приватный чат, проверяем, что второй пользователь тоже в этой семье
	if isPrivate {
		if recipientID == parentID {
			// Родитель всегда в семье, все ок
		} else {
			// Проверяем, что указанный получатель в этой семье
			inFamily, err := isChildBelongsToFamily(wsChatService, recipientID, parentID)
			if err != nil || !inFamily {
				c.JSON(http.StatusForbidden, gin.H{"error": "recipient not in this family"})
				return
			}
		}
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Создаем клиента используя публичный конструктор
	client := ws.NewClient(hub, userID.(string), targetID, conn)

	// Регистрируем клиента
	hub.Register(client)

	// Запускаем горутины для обработки сообщений
	go client.WritePump()
	go client.ReadPump()
}

// Вспомогательная функция для проверки принадлежности ребенка к семье
func isChildBelongsToFamily(service *services.ChatService, childID, parentID string) (bool, error) {
	// Проверяем, существует ли ребенок с таким ID
	// Обратите внимание: мы не сохраняем результат в переменную child, так как она не используется
	_, err := service.ChildRepo.FindByFirebaseUID(childID)
	if err != nil {
		return false, err
	}

	parent, err := service.ParentRepo.FindByFirebaseUID(parentID)
	if err != nil {
		return false, err
	}

	// Проверяем, принадлежит ли ребенок к семье
	var family []map[string]interface{}
	if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
		return false, err
	}

	for _, member := range family {
		if firebaseUID, ok := member["firebase_uid"].(string); ok && firebaseUID == childID {
			return true, nil
		}
	}

	return false, nil
}
