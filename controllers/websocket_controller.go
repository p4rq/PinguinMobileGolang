package controllers

import (
	"PinguinMobile/websocket"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

var WebSocketHub *websocket.Hub

func SetWebSocketHub(hub *websocket.Hub) {
	WebSocketHub = hub
	go WebSocketHub.Run()
}

// ServeWs обрабатывает WebSocket запрос от клиента
func ServeWs(c *gin.Context) {
	// Получаем ID пользователя из токена
	userID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Получаем тип пользователя
	userType, exists := c.Get("user_type")
	if !exists {
		userType = "unknown"
	}

	// Получаем ID родителя (семьи)
	parentID := c.Query("parent_id")
	if parentID == "" {
		// Если parentID не указан в запросе, проверяем не является ли пользователь родителем
		if userType == "parent" {
			parentID = userID.(string)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "parent_id is required"})
			return
		}
	}

	fmt.Printf("WebSocket connection: user %s (%s) connecting to family %s\n",
		userID.(string), userType.(string), parentID)

	// Обработка WebSocket соединения
	websocket.ServeWs(WebSocketHub, c.Writer, c.Request, userID.(string), parentID, userType.(string))
}
