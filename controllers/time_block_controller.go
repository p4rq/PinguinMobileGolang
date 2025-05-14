package controllers

import (
	"PinguinMobile/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// BlockAppsByTime обрабатывает запрос на блокировку приложений по времени
func BlockAppsByTime(c *gin.Context) {
	// Получаем данные из запроса
	var request struct {
		ChildID string                `json:"child_id" binding:"required"`
		Blocks  []models.AppTimeBlock `json:"blocks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем FirebaseUID родителя напрямую из контекста (установлен в AuthMiddleware)
	parentFirebaseUID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing firebase_uid"})
		return
	}

	// Проверяем тип пользователя
	userType, exists := c.Get("user_type")
	if !exists || userType.(string) != "parent" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: only parents can block apps"})
		return
	}

	// Блокируем приложения на указанное время через Family JSON
	err := parentService.BlockAppsByTime(parentFirebaseUID.(string), request.ChildID, request.Blocks)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UnblockAppsByTime обрабатывает запрос на отмену временной блокировки
func UnblockAppsByTime(c *gin.Context) {
	// Получаем данные из запроса
	var request struct {
		ChildID string   `json:"child_id" binding:"required"`
		Apps    []string `json:"apps" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем FirebaseUID родителя напрямую из контекста (установлен в AuthMiddleware)
	parentFirebaseUID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing firebase_uid"})
		return
	}

	// Проверяем тип пользователя
	userType, exists := c.Get("user_type")
	if !exists || userType.(string) != "parent" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: only parents can unblock apps"})
		return
	}

	// Разблокируем приложения через Family JSON
	err := parentService.UnblockAppsByTime(parentFirebaseUID.(string), request.ChildID, request.Apps)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetTimeBlockedApps возвращает список временных блокировок
func GetTimeBlockedApps(c *gin.Context) {
	// Получаем ID ребенка из запроса
	childID := c.Param("firebase_uid")
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "child ID is required"})
		return
	}

	// Получаем FirebaseUID родителя напрямую из контекста (установлен в AuthMiddleware)
	parentFirebaseUID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing firebase_uid"})
		return
	}

	// Проверяем тип пользователя
	userType, exists := c.Get("user_type")
	if !exists || userType.(string) != "parent" {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden: only parents can get blocked apps"})
		return
	}

	// Получаем список блокировок через Family JSON
	blocks, err := parentService.GetTimeBlockedApps(parentFirebaseUID.(string), childID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, blocks)
}

// CheckAppBlocking проверяет, заблокировано ли приложение
func CheckAppBlocking(c *gin.Context) {
	// Получаем FirebaseUID ребенка из контекста
	childID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Получаем пакет приложения из запроса
	appPackage := c.Query("package")
	if appPackage == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "app package is required"})
		return
	}

	// Проверяем блокировку
	isBlocked, reason, err := childService.CheckAppBlocking(childID.(string), appPackage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"blocked": isBlocked,
		"reason":  reason,
	})
}
