package controllers

import (
	"PinguinMobile/config"
	"PinguinMobile/websocket"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var WebSocketHub *websocket.Hub

func verifyToken(tokenString string) (map[string]interface{}, error) {
	// Очищаем токен от "Bearer " префикса, если такой есть
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// Проверяем токен через Firebase
	token, err := config.FirebaseAuth.VerifyIDToken(context.Background(), tokenString)
	if err != nil {
		return nil, err
	}

	// Получаем дополнительную информацию о пользователе
	user, err := config.FirebaseAuth.GetUser(context.Background(), token.UID)
	if err != nil {
		return nil, err
	}

	// Создаем map с claims для дальнейшего использования
	claims := map[string]interface{}{
		"firebase_uid": token.UID,
	}

	// Добавляем user_type и parent_id из пользовательских claims
	for key, claim := range user.CustomClaims {
		claims[key] = claim
	}

	// Если тип пользователя не определен, устанавливаем по умолчанию "parent"
	if _, exists := claims["user_type"]; !exists {
		claims["user_type"] = "parent"
	}

	return claims, nil
}
func SetWebSocketHub(hub *websocket.Hub) {
	WebSocketHub = hub
	go WebSocketHub.Run()
}

// ServeWs обрабатывает WebSocket запрос от клиента
func ServeWs(c *gin.Context) {
	// Извлекаем токен из запроса
	token := c.Query("token")

	// Проверяем и извлекаем информацию из токена
	claims, err := verifyToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Важно: правильно извлекаем userID и parentID из токена
	userID := claims["firebase_uid"].(string)
	parentID := userID // По умолчанию parentID = userID (для родителей)

	// Если пользователь - ребенок, то parentID будет отличаться от userID
	if claims["user_type"] == "child" && claims["parent_id"] != nil {
		parentID = claims["parent_id"].(string)
	}

	fmt.Printf("WebSocket connection: user %s connecting to family %s\n",
		userID, parentID)

	// Обработка WebSocket соединения
	websocket.ServeWs(WebSocketHub, c.Writer, c.Request, userID, parentID, claims["user_type"].(string))
}
