package controllers

import (
	"PinguinMobile/config" // Для доступа к DB
	"PinguinMobile/models"
	ws "PinguinMobile/websocket"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5" // или используйте тот же импорт, что и в auth_service.go
	"github.com/gorilla/websocket"
)

var (
	// Настройка websocket upgrader
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Разрешаем подключение с любого источника
		},
	}
	wsHub     *ws.Hub
	jwtSecret = []byte("your_secret_key") // Используем тот же ключ, что и в auth_service.go
)

// PinguinClaims должна соответствовать Claims в auth_service.go
type PinguinClaims struct {
	Email                string `json:"email"`
	FirebaseUID          string `json:"firebase_uid"`
	UserType             string `json:"user_type"`
	jwt.RegisteredClaims        // Если используется jwt v5, иначе используйте jwt.StandardClaims
}

// SetWebSocketHub устанавливает хаб для WebSocket соединений
func SetWebSocketHub(hub *ws.Hub) {
	wsHub = hub
	WebSocketHub = hub // Устанавливаем значение для глобальной переменной, используемой в chat_controller.go
	go wsHub.Run()
}

// extractTokenFromRequest извлекает JWT токен из запроса
func extractTokenFromRequest(c *gin.Context) (string, error) {
	// Отладочные сообщения
	log.Printf("Headers: %v", c.Request.Header)
	token := c.Query("token")
	if token != "" {
		log.Printf("Found token in query: %s", token)
		return token, nil
	}

	authHeader := c.GetHeader("Authorization")
	log.Printf("Authorization header: %s", authHeader)
	if authHeader == "" {
		return "", errors.New("отсутствует токен авторизации")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", errors.New("неверный формат токена авторизации")
	}

	log.Printf("Extracted token: %s", parts[1])
	return parts[1], nil
}

// getUserInfoFromToken извлекает информацию о пользователе из JWT токена
func getUserInfoFromToken(tokenString string) (string, string, error) {
	log.Printf("Validating token with secret: %s", string(jwtSecret))

	token, err := jwt.ParseWithClaims(tokenString, &PinguinClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		log.Printf("JWT Parse error: %v", err)
		return "", "", err
	}

	if !token.Valid {
		log.Printf("Token is invalid")
		return "", "", errors.New("недействительный токен")
	}

	claims, ok := token.Claims.(*PinguinClaims)
	if !ok {
		log.Printf("Failed to parse claims")
		return "", "", errors.New("неверный формат данных токена")
	}

	log.Printf("Successfully parsed claims. firebase_uid: %s, type: %s",
		claims.FirebaseUID, claims.UserType)

	return claims.FirebaseUID, claims.UserType, nil
}

// ServeWs обрабатывает подключение WebSocket
func ServeWs(c *gin.Context) {
	// Устанавливаем более длинный тайм-аут для соединения
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Keep-Alive", "timeout=120")

	log.Printf("WebSocket connection attempt from IP: %s", c.ClientIP())

	// Получаем токен из запроса
	tokenString, err := extractTokenFromRequest(c)
	if err != nil {
		log.Printf("Error extracting token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Необходим токен авторизации",
		})
		return
	}

	// Извлекаем информацию о пользователе из токена
	userID, userType, err := getUserInfoFromToken(tokenString)
	if err != nil {
		log.Printf("Error validating token: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Ошибка проверки токена: " + err.Error(),
		})
		return
	}

	log.Printf("Successfully authenticated user: %s (type: %s)", userID, userType)

	// Получаем ID семьи (parent_id)
	var parentID string

	// Новый код для определения parent_id
	if userType == "child" {
		// Для ребенка получаем parent_id из поля Family (JSON)
		var child models.Child
		if err := config.DB.Where("firebase_uid = ?", userID).First(&child).Error; err != nil {
			log.Printf("Error finding child: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: child info not found"})
			return
		}

		// Парсим поле Family как JSON
		if child.Family != "" {
			log.Printf("Child's family data: %s", child.Family)

			// Сначала пытаемся разобрать parent_firebase_uid как строку
			var familyDataString struct {
				ParentFirebaseUID string `json:"parent_firebase_uid"`
			}

			if err := json.Unmarshal([]byte(child.Family), &familyDataString); err == nil && familyDataString.ParentFirebaseUID != "" {
				parentID = familyDataString.ParentFirebaseUID
				log.Printf("Found parent_firebase_uid (string) %s in child's family JSON", parentID)
			} else {
				// Если не получилось как строку, пробуем как число
				var familyDataNumber struct {
					ParentFirebaseUID json.Number `json:"parent_firebase_uid"`
				}

				if err := json.Unmarshal([]byte(child.Family), &familyDataNumber); err == nil && familyDataNumber.ParentFirebaseUID != "" {
					parentID = string(familyDataNumber.ParentFirebaseUID)
					log.Printf("Found parent_firebase_uid (number) %s in child's family JSON", parentID)
				} else if err != nil {
					// Если и так не получилось, пробуем общий подход через map
					var familyMap map[string]interface{}
					if err := json.Unmarshal([]byte(child.Family), &familyMap); err == nil {
						if parentIDVal, ok := familyMap["parent_firebase_uid"]; ok {
							// Конвертируем в строку, независимо от типа
							parentID = fmt.Sprintf("%v", parentIDVal)
							log.Printf("Found parent_firebase_uid (interface) %s in child's family JSON", parentID)
						} else {
							log.Printf("No parent_firebase_uid field found in family JSON map")
						}
					} else {
						log.Printf("Error parsing family JSON as map: %v", err)
					}
				}
			}
		}

		// Если не удалось найти parent_id в JSON, используем переданный в запросе
		if parentID == "" {
			parentID = c.Query("parent_id")
			log.Printf("Using provided parent_id for child: %s", parentID)
		}
	} else if userType == "parent" {
		// Для родителя используем его собственный ID
		parentID = userID
		log.Printf("Using parent's own ID as parent_id: %s", parentID)
	} else {
		// Для других типов пользователей используем переданный parent_id
		parentID = c.Query("parent_id")
		log.Printf("Using provided parent_id: %s", parentID)
	}

	// Проверяем обязательный параметр parent_id
	if parentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Не удалось определить ID семьи (parent_id)",
		})
		return
	}

	// Получаем имя пользователя
	userName := c.Query("user_name")
	if userName == "" {
		// Если имя не указано, пытаемся получить из базы данных
		if userType == "parent" {
			// Находим информацию о родителе
			var parent models.Parent
			if err := config.DB.Where("firebase_uid = ?", userID).First(&parent).Error; err == nil {
				userName = parent.Name
			} else {
				userName = "Родитель"
			}
		} else if userType == "child" {
			// Находим информацию о ребенке
			var child models.Child
			if err := config.DB.Where("firebase_uid = ?", userID).First(&child).Error; err == nil {
				userName = child.Name
			} else {
				userName = "Ребенок"
			}
		} else {
			userName = "Пользователь"
		}
	}

	// Повышаем соединение до WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Ошибка повышения до WebSocket:", err)
		return
	}

	// Создаем и регистрируем клиента
	client := ws.NewClient(wsHub, conn, userID, parentID, userName)
	wsHub.RegisterClient(client)

	// Отправляем keep-alive сообщение сразу после подключения
	keepAliveMsg := ws.WebSocketMessage{
		Type:       "keep_alive",
		ParentID:   parentID,
		SenderID:   "system",
		SenderName: "System",
		Message:    "Connection established",
		Timestamp:  time.Now(),
	}
	client.Send(keepAliveMsg)

	// Запускаем горутины для обработки сообщений
	go client.WritePump()
	go client.ReadPump()
	// go client.KeepAlive() // Запускаем периодические keep-alive сообщения

	// Отправляем системное сообщение о подключении
	wsHub.BroadcastMessage(ws.WebSocketMessage{
		Type:       "chat_message",
		ParentID:   parentID,
		SenderID:   "system",
		SenderName: "Система",
		Message:    userName + " присоединился к чату",
		Timestamp:  time.Now(),
	})
}

// ChatService интерфейс для адаптера сервиса чата
type ChatService interface {
	SaveMessage(message *models.ChatMessage) error
	GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error)
}

// ChatServiceAdapter адаптер для ChatService
type ChatServiceAdapter struct {
	ChatService ChatService
}

// SaveMessage сохраняет сообщение в БД
func (a *ChatServiceAdapter) SaveMessage(message *models.ChatMessage) error {
	return a.ChatService.SaveMessage(message)
}

// GetMessages получает историю сообщений
func (a *ChatServiceAdapter) GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error) {
	return a.ChatService.GetMessages(parentID, userID, limit)
}
