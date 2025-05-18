package controllers

import (
	"PinguinMobile/websocket"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

var WebSocketHub *websocket.Hub

func SetWebSocketHub(hub *websocket.Hub) {
	WebSocketHub = hub
	go WebSocketHub.Run()
}

// verifyTokenForWebSocket проверяет JWT токен специально для WebSocket соединений
// без дополнительных проверок в БД
func verifyTokenForWebSocket(tokenString string) (map[string]interface{}, error) {
	// Очищаем токен от "Bearer " префикса
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// Парсим JWT токен
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Проверка метода подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Получаем секрет из переменной окружения
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			// Если переменная окружения не установлена, используем значение по умолчанию
			jwtSecret = "your-secret-key" // Замените на ваш реальный секретный ключ
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	// Проверяем валидность токена
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Проверяем необходимые поля в токене
	if _, ok := claims["firebase_uid"]; !ok {
		return nil, errors.New("token does not contain firebase_uid")
	}

	return claims, nil
}

// ServeWs обрабатывает WebSocket запрос от клиента
func ServeWs(c *gin.Context) {
	// Получаем токен из query параметра
	token := c.Query("token")
	if token == "" {
		// Пробуем из заголовка Authorization
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
		return
	}

	// ЗАМЕНЯЕМ эту строку:
	// claims, err := authService.VerifyToken(token)
	// НА нашу новую функцию:
	claimsMap, err := verifyTokenForWebSocket(token)
	if err != nil {
		log.Printf("WebSocket auth error: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Извлекаем данные пользователя напрямую из распарсенного токена
	userID, _ := claimsMap["firebase_uid"].(string)
	userType, _ := claimsMap["user_type"].(string)

	// Получаем ID родителя (для родителя это его собственный ID)
	parentID := userID
	if userType == "child" {
		// Для ребенка - получаем parent_id из токена или запроса
		if parentIDFromClaims, exists := claimsMap["parent_id"].(string); exists {
			parentID = parentIDFromClaims
		} else {
			parentIDParam := c.Query("parent_id")
			if parentIDParam != "" {
				parentID = parentIDParam
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Missing parent_id for child user"})
				return
			}
		}
	}

	log.Printf("WebSocket: Authorized as %s (%s), parentID: %s", userID, userType, parentID)

	// Проверяем, что хаб инициализирован
	if WebSocketHub == nil {
		log.Printf("Error: WebSocketHub not initialized")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "WebSocket service unavailable"})
		return
	}

	// Передаем запрос в ServeWs
	websocket.ServeWs(WebSocketHub, c.Writer, c.Request, userID, parentID, userType)
}
