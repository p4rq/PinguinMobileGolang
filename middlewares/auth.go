package middlewares

import (
	"net/http"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

var jwtKey = []byte("your_secret_key")

type Claims struct {
	Email string `json:"email"`
	jwt.StandardClaims
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			// Проверяем и извлекаем firebase_uid
			if firebaseUID, exists := claims["firebase_uid"].(string); exists {
				c.Set("firebase_uid", firebaseUID)
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: missing firebase_uid"})
				c.Abort()
				return
			}

			// Проверяем и извлекаем user_type
			if userType, exists := claims["user_type"].(string); exists {
				c.Set("user_type", userType)
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: missing user_type"})
				c.Abort()
				return
			}

			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
	}
}
