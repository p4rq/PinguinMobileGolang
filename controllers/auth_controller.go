package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

var authService *services.AuthService

func SetAuthService(service *services.AuthService) {
	authService = service
}

func RegisterParent(c *gin.Context) {
	var input struct {
		Lang     string `json:"lang" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	if input.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password cannot be empty"})
		return
	}

	// Определяем язык из запроса или устанавливаем по умолчанию
	if input.Lang == "" {
		input.Lang = "ru" // Русский как язык по умолчанию
	}

	parent, token, err := authService.RegisterParent(input.Lang, input.Name, input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": true, "token": token, "data": parent})
}

func LoginParent(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	parent, token, err := authService.LoginParent(input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "token": token, "user": parent})
}

func RegisterChild(c *gin.Context) {
	var input struct {
		Lang string `json:"lang" binding:"required"`
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	// Передаем пустую строку в качестве имени
	child, token, err := authService.RegisterChild(input.Lang, input.Code, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": true, "token": token, "data": child})
}

func TokenVerify(c *gin.Context) {
	var input struct {
		UID string `json:"uid" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	firebaseUID := input.UID

	// Вызываем сервис для верификации пользователя
	user, err := authService.VerifyToken(firebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Проверяем, является ли пользователь ребенком
	child, isChild := user.(models.Child)
	if isChild && child.TimeBlockedApps != "" {
		// Создаем структуры для разных типов блокировок
		type PermanentBlockResponse struct {
			Apps         []string  `json:"apps"`
			BlockName    string    `json:"block_name"`
			Duration     string    `json:"duration"`
			EndTime      time.Time `json:"end_time,omitempty"`
			ID           int64     `json:"id"`
			IsOneTime    bool      `json:"is_one_time"`
			RemainingMin int       `json:"remaining_min"`
		}

		type ScheduleBlockResponse struct {
			Apps       []string `json:"apps"`
			BlockName  string   `json:"block_name"`
			DaysOfWeek string   `json:"days_of_week"`
			EndTime    string   `json:"end_time"`
			ID         int64    `json:"id"`
			StartTime  string   `json:"start_time"`
		}

		// Интерфейс для объединения разных типов блокировок
		type BlockResponse interface{}

		// Десериализуем TimeBlockedApps в структуру
		var timeBlockRules []struct {
			ID           int64     `json:"id"`
			AppPackage   string    `json:"app_package"`
			StartTime    string    `json:"start_time"`
			EndTime      string    `json:"end_time"`
			DaysOfWeek   string    `json:"days_of_week"`
			IsOneTime    bool      `json:"is_one_time"`
			OneTimeEndAt time.Time `json:"one_time_end_at"`
			Duration     string    `json:"duration"`
			BlockName    string    `json:"block_name"`
			IsPermanent  bool      `json:"is_permanent"`
		}

		if err := json.Unmarshal([]byte(child.TimeBlockedApps), &timeBlockRules); err == nil {
			// Создаем группировки для каждого типа блокировок
			groupedBlocksMap := make(map[string]map[string]interface{})  // Для временных блокировок
			permanentBlocksMap := make(map[int64]map[string]interface{}) // Для постоянных блокировок

			for _, rule := range timeBlockRules {
				if rule.IsPermanent {
					// Обработка постоянных блокировок
					if block, exists := permanentBlocksMap[rule.ID]; exists {
						// Добавляем приложение к существующей блокировке
						apps := block["apps"].([]string)
						apps = append(apps, rule.AppPackage)
						block["apps"] = apps
					} else {
						// Создаем новую запись постоянной блокировки
						permanentBlocksMap[rule.ID] = map[string]interface{}{
							"id":            rule.ID,
							"apps":          []string{rule.AppPackage},
							"block_name":    rule.BlockName,
							"duration":      rule.Duration,
							"end_time":      rule.OneTimeEndAt,
							"is_one_time":   rule.IsOneTime,
							"remaining_min": 0,
						}
					}
				} else {
					// Обработка временных блокировок (существующий код)
					key := rule.StartTime + "_" + rule.EndTime + "_" + rule.BlockName + "_" + rule.DaysOfWeek

					if group, exists := groupedBlocksMap[key]; exists {
						apps := group["apps"].([]string)
						apps = append(apps, rule.AppPackage)
						group["apps"] = apps
					} else {
						groupedBlocksMap[key] = map[string]interface{}{
							"id":           rule.ID,
							"start_time":   rule.StartTime,
							"end_time":     rule.EndTime,
							"block_name":   rule.BlockName,
							"days_of_week": rule.DaysOfWeek,
							"apps":         []string{rule.AppPackage},
						}
					}
				}
			}

			// Объединяем все блоки в один массив
			var allBlocks []map[string]interface{}

			// Добавляем временные блокировки
			for _, block := range groupedBlocksMap {
				allBlocks = append(allBlocks, block)
			}

			// Добавляем постоянные блокировки
			for _, block := range permanentBlocksMap {
				allBlocks = append(allBlocks, block)
			}

			// Добавляем ответ с правильно сгруппированными блоками
			type ChildWithBlocks struct {
				models.Child
				Blocks []map[string]interface{} `json:"blocks"`
			}

			childWithBlocks := ChildWithBlocks{
				Child:  child,
				Blocks: allBlocks,
			}

			c.JSON(http.StatusOK, gin.H{
				"message": true,
				"user":    childWithBlocks,
			})
			return
		}
	}

	// Если пользователь не ребенок или нет блокировок - возвращаем стандартный ответ
	c.JSON(http.StatusOK, gin.H{
		"message": true,
		"user":    user,
	})
}

// LoginChild logs in a child using their code
func LoginChild(c *gin.Context) {
	var input struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	child, token, err := authService.LoginChild(input.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "token": token, "user": child})
}
