package controllers

import (
	"PinguinMobile/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// BlockAppsByTime обрабатывает запрос на блокировку приложений по времени
func BlockAppsByTime(c *gin.Context) {
	// Получаем данные из запроса
	var request struct {
		ChildID    string   `json:"child_id" binding:"required"`
		Apps       []string `json:"apps" binding:"required"`
		TimeBlocks []struct {
			StartTime string `json:"start_time" binding:"required"`
			EndTime   string `json:"end_time" binding:"required"`
		} `json:"time_blocks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Получаем FirebaseUID родителя из контекста
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

	// Для каждого приложения и каждого временного блока создаем блокировку
	for _, app := range request.Apps {
		for _, timeBlock := range request.TimeBlocks {
			err := parentService.ManageAppTimeRules(
				parentFirebaseUID.(string),
				request.ChildID,
				[]string{app},
				"block",
				timeBlock.StartTime,
				timeBlock.EndTime,
				"",
			)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
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

	// Получаем FirebaseUID родителя напрямую из контекста
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

	// Используем новый единый метод для разблокировки
	err := parentService.ManageAppTimeRules(
		parentFirebaseUID.(string),
		request.ChildID,
		request.Apps,
		"unblock",
		"", // start_time не требуется для разблокировки
		"", // end_time не требуется для разблокировки
		"",
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetTimeBlockedApps возвращает список временных блокировок по расписанию (не одноразовых)
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

	// Фильтруем, оставляя только регулярные блокировки (не одноразовые)
	var regularBlocks []models.AppTimeBlock
	for _, block := range blocks {
		if !block.IsOneTime {
			regularBlocks = append(regularBlocks, block)
		}
	}

	// Группируем блоки по временным интервалам
	groupedBlocks := make(map[string]map[string]interface{})

	for _, block := range regularBlocks {
		// Создаем ключ для группировки по времени
		key := fmt.Sprintf("%s_%s_%s_%s", block.StartTime, block.EndTime, block.BlockName, block.DaysOfWeek)

		if group, exists := groupedBlocks[key]; exists {
			// Добавляем приложение в существующую группу
			apps := group["apps"].([]string)
			apps = append(apps, block.AppPackage)
			group["apps"] = apps
		} else {
			// Создаем новую группу
			groupedBlocks[key] = map[string]interface{}{
				"id":           block.ID,
				"start_time":   block.StartTime,
				"end_time":     block.EndTime,
				"block_name":   block.BlockName,
				"days_of_week": block.DaysOfWeek,
				"apps":         []string{block.AppPackage},
				"is_one_time":  false, // Явно указываем, что это не одноразовая блокировка
			}
		}
	}

	// Преобразуем в массив для ответа
	var result []map[string]interface{}
	for _, group := range groupedBlocks {
		result = append(result, group)
	}

	c.JSON(http.StatusOK, gin.H{
		"blocks": result,
	})
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
