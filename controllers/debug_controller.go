package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DebugAuth выводит информацию из контекста аутентификации
func DebugAuth(c *gin.Context) {
	// Собираем всю информацию из контекста
	firebaseUID, uidExists := c.Get("firebase_uid")
	userType, typeExists := c.Get("user_type")
	claims, claimsExist := c.Get("claims")

	c.JSON(http.StatusOK, gin.H{
		"firebase_uid_exists": uidExists,
		"firebase_uid":        firebaseUID,
		"user_type_exists":    typeExists,
		"user_type":           userType,
		"claims_exists":       claimsExist,
		"claims":              claims,
		"all_context_keys":    c.Keys,
	})
}
