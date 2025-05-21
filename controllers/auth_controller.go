package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"fmt"
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
			// Дедупликация данных при десериализации
			// Создаем уникальные идентификаторы правил для отбора дубликатов
			uniqueRules := make(map[string]struct {
				Rule struct {
					ID           int64
					AppPackage   string
					StartTime    string
					EndTime      string
					DaysOfWeek   string
					IsOneTime    bool
					OneTimeEndAt time.Time
					Duration     string
					BlockName    string
					IsPermanent  bool
				}
				Apps []string
			})

			// Группируем правила по ключу и собираем для них уникальные приложения
			for _, rule := range timeBlockRules {
				// Создаем ключ для дедупликации, учитывая все важные параметры
				key := fmt.Sprintf("%s_%s_%s_%s_%t_%t_%s",
					rule.StartTime, rule.EndTime, rule.BlockName,
					rule.DaysOfWeek, rule.IsOneTime, rule.IsPermanent,
					rule.Duration)

				if existingRule, exists := uniqueRules[key]; exists {
					// Добавляем приложение только если его еще нет в списке
					appExists := false
					for _, app := range existingRule.Apps {
						if app == rule.AppPackage {
							appExists = true
							break
						}
					}
					if !appExists {
						item := uniqueRules[key]
						item.Apps = append(item.Apps, rule.AppPackage)
						uniqueRules[key] = item
					}
				} else {
					uniqueRules[key] = struct {
						Rule struct {
							ID           int64
							AppPackage   string
							StartTime    string
							EndTime      string
							DaysOfWeek   string
							IsOneTime    bool
							OneTimeEndAt time.Time
							Duration     string
							BlockName    string
							IsPermanent  bool
						}
						Apps []string
					}{
						Rule: struct {
							ID           int64
							AppPackage   string
							StartTime    string
							EndTime      string
							DaysOfWeek   string
							IsOneTime    bool
							OneTimeEndAt time.Time
							Duration     string
							BlockName    string
							IsPermanent  bool
						}{
							ID:           rule.ID,
							AppPackage:   rule.AppPackage,
							StartTime:    rule.StartTime,
							EndTime:      rule.EndTime,
							DaysOfWeek:   rule.DaysOfWeek,
							IsOneTime:    rule.IsOneTime,
							OneTimeEndAt: rule.OneTimeEndAt,
							Duration:     rule.Duration,
							BlockName:    rule.BlockName,
							IsPermanent:  rule.IsPermanent,
						},
						Apps: []string{rule.AppPackage},
					}
				}
			}

			// Теперь у нас есть уникальные правила с дедуплицированными приложениями
			// Далее создаем результат для ответа API
			var allBlocks []map[string]interface{}

			for _, uniqueRule := range uniqueRules {
				rule := uniqueRule.Rule
				apps := uniqueRule.Apps

				if rule.IsPermanent {
					// Формат для постоянных блокировок
					allBlocks = append(allBlocks, map[string]interface{}{
						"id":            rule.ID,
						"apps":          apps,
						"block_name":    rule.BlockName,
						"duration":      rule.Duration,
						"end_time":      rule.OneTimeEndAt,
						"is_one_time":   rule.IsOneTime,
						"remaining_min": 0,
						"is_permanent":  true,
					})
				} else if rule.IsOneTime {
					// Формат для одноразовых блокировок
					var remainingMins int
					if !rule.OneTimeEndAt.IsZero() {
						remainingMins = int(time.Until(rule.OneTimeEndAt).Minutes())
						if remainingMins < 0 {
							remainingMins = 0
						}
					}

					allBlocks = append(allBlocks, map[string]interface{}{
						"id":            rule.ID,
						"apps":          apps,
						"block_name":    rule.BlockName,
						"duration":      rule.Duration,
						"is_one_time":   true,
						"remaining_min": remainingMins,
					})
				} else {
					// Формат для временных блокировок по расписанию
					allBlocks = append(allBlocks, map[string]interface{}{
						"id":           rule.ID,
						"apps":         apps,
						"block_name":   rule.BlockName,
						"start_time":   rule.StartTime,
						"end_time":     rule.EndTime,
						"days_of_week": rule.DaysOfWeek,
					})
				}
			}

			// Продолжаем с существующим кодом...
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
