package controllers

import (
	"PinguinMobile/models"
	"net/http"
	"time"

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
	// Получаем ID ребенка из параметра запроса
	childID := c.Query("child_id")
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "child_id is required"})
		return
	}

	// Получаем пакет приложения из запроса
	appPackage := c.Query("app_package") // Изменил с "package" на "app_package"
	if appPackage == "" {
		// Проверяем альтернативное имя параметра
		appPackage = c.Query("package")
		if appPackage == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "app_package is required"})
			return
		}
	}

	// Проверяем блокировку
	isBlocked, blockType, err := childService.CheckAppBlocking(childID, appPackage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Базовый ответ
	response := gin.H{
		"blocked": isBlocked,
		"type":    blockType,
	}

	// Если это одноразовая блокировка, добавляем дополнительную информацию
	if isBlocked && blockType == "one_time" {
		// Получаем ребенка
		child, err := childService.ReadChild(childID)
		if err == nil {
			// Получаем временные блокировки
			blocks, err := childService.ChildRepo.GetTimeBlockedApps(child.ID)
			if err == nil {
				for _, block := range blocks {
					if block.AppPackage == appPackage && block.IsOneTime {
						remainingTime := time.Until(block.OneTimeEndAt)
						response["end_time"] = block.OneTimeEndAt
						response["remaining_minutes"] = int(remainingTime.Minutes())
						response["duration"] = block.Duration
						break
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}
