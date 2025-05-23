package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
)

// Убираем дублирование - используем переменные из parent_controller.go
// var authService *services.AuthService
// var translationService *services.TranslationService

// Используем эти переменные только в auth_controller.go
var authService *services.AuthService

func SetAuthService(service *services.AuthService) {
	authService = service
}

// Убираем дублирование - эта функция должна быть в одном файле
// func SetTranslationService(service *services.TranslationService) {
// 	translationService = service
// }

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

	// Проверяем, существует ли уже пользователь с таким email
	existingParent, err := parentService.FindByEmail(input.Email)
	if existingParent.ID != 0 {
		// Генерируем JWT токен для существующего пользователя
		token, tokenErr := authService.GenerateToken(existingParent.FirebaseUID)
		if tokenErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка генерации токена"})
			return
		}

		// Если email не подтвержден, повторно отправляем код
		if !existingParent.EmailVerified {
			// Отправляем код верификации повторно
			sendErr := parentService.SendVerificationCode(&existingParent)
			if sendErr != nil {
				fmt.Printf("Error sending verification code: %v\n", sendErr)
			}

			// Возвращаем токен и информацию, что код отправлен повторно
			c.JSON(http.StatusOK, gin.H{
				"status":  true,
				"message": "Email уже зарегистрирован. Код подтверждения отправлен повторно.",
				"token":   token,
				"data": gin.H{
					"id":             existingParent.ID,
					"name":           existingParent.Name,
					"email":          existingParent.Email,
					"firebase_uid":   existingParent.FirebaseUID,
					"role":           existingParent.Role,
					"email_verified": existingParent.EmailVerified,
				},
			})
		} else {
			// Если email уже подтвержден, просто возвращаем токен и информацию
			c.JSON(http.StatusOK, gin.H{
				"status":  true,
				"message": "Email уже зарегистрирован и подтвержден.",
				"token":   token,
				"data": gin.H{
					"id":             existingParent.ID,
					"name":           existingParent.Name,
					"email":          existingParent.Email,
					"firebase_uid":   existingParent.FirebaseUID,
					"role":           existingParent.Role,
					"email_verified": existingParent.EmailVerified,
				},
			})
		}
		return
	}

	// Регистрируем нового пользователя
	parent, token, err := authService.RegisterParent(input.Lang, input.Name, input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// ВАЖНО: Находим только что созданную запись по email или Firebase UID
	createdParent, err := parentService.FindByEmail(input.Email)
	if err != nil || createdParent.ID == 0 {
		// Если не нашли по email, пробуем найти по Firebase UID
		createdParent, err = parentService.ReadParent(parent.FirebaseUID)
		if err != nil {
			fmt.Printf("ERROR: Не удалось найти созданного пользователя: %v\n", err)
			// Продолжаем с исходным объектом, хотя это может привести к дублированию
		} else {
			// Заменяем parent на найденный объект с корректным ID
			parent = createdParent
		}
	} else {
		// Заменяем parent на найденный объект с корректным ID
		parent = createdParent
	}

	// Теперь отправляем код на правильный объект с ID
	err = parentService.SendVerificationCode(&parent)
	if err != nil {
		// Логируем ошибку, но продолжаем процесс регистрации
		fmt.Printf("Error sending verification email: %v\n", err)
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  true,
		"message": "Регистрация успешна. Проверьте email для подтверждения.",
		"token":   token,
		"data": gin.H{
			"id":             parent.ID,
			"name":           parent.Name,
			"email":          parent.Email,
			"firebase_uid":   parent.FirebaseUID,
			"role":           parent.Role,
			"email_verified": parent.EmailVerified,
		},
	})
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
	// Проверяем, подтвержден ли email
	if !parent.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{
			"status":            false,
			"message":           "Email не подтвержден. Пожалуйста, проверьте почту или запросите новый код.",
			"need_verification": true,
			"firebase_uid":      parent.FirebaseUID,
		})
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
	parent, isParent := user.(models.Parent)
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
					// ВАЖНОЕ ИЗМЕНЕНИЕ: Не вычисляем оставшееся время,
					// а извлекаем оригинальную продолжительность из строки Duration
					var remainingMins int

					// Извлекаем числовое значение из строки Duration (например, "7 минут")
					re := regexp.MustCompile(`\d+`)
					matches := re.FindString(rule.Duration)
					if matches != "" {
						remainingMins, _ = strconv.Atoi(matches)
					} else {
						// Если не удалось извлечь число, используем 0
						remainingMins = 0
					}

					// НЕ используем time.Until - это именно то, что вызывало уменьшение значения
					// if !rule.OneTimeEndAt.IsZero() {
					//     remainingMins = int(time.Until(rule.OneTimeEndAt).Minutes())
					//     if remainingMins < 0 {
					//         remainingMins = 0
					//     }
					// }

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

			// Получаем информацию о последнем обновлении переводов
			lastUpdateTime, err := translationService.GetLastUpdateTime()
			if err != nil {
				// Если ошибка, используем текущее время чтобы избежать ошибок
				lastUpdateTime = time.Now()
			}

			// Формируем информацию о переводах
			translationInfo := gin.H{
				"lastUpdatedAt": lastUpdateTime,
				"langUpdate":    true, // Флаг для клиента, что нужно обновить переводы
			}

			// Добавляем информацию о переводах к ответу
			c.JSON(http.StatusOK, gin.H{
				"message": true,
				"user": gin.H{
					"id":                child.ID,
					"role":              child.Role,
					"lang":              child.Lang,
					"name":              child.Name,
					"family":            child.Family,
					"firebase_uid":      child.FirebaseUID,
					"is_binded":         child.IsBinded,
					"usage_data":        child.UsageData,
					"gender":            child.Gender,
					"age":               child.Age,
					"birthday":          child.Birthday,
					"code":              child.Code,
					"blocks":            allBlocks,
					"translations_info": translationInfo, // Добавляем информацию о переводах
				},
			})
			return
		}
	} else if isParent {
		// Если пользователь - родитель
		// Определяем translationInfo для родителя
		lastUpdateTime, err := translationService.GetLastUpdateTime()
		if err != nil {
			lastUpdateTime = time.Now()
		}

		translationInfo := gin.H{
			"lastUpdatedAt": lastUpdateTime,
			"langUpdate":    true,
		}

		c.JSON(http.StatusOK, gin.H{
			"message": true,
			"user": gin.H{
				"id":                parent.ID,
				"role":              parent.Role,
				"lang":              parent.Lang,
				"name":              parent.Name,
				"family":            parent.Family,
				"firebase_uid":      parent.FirebaseUID,
				"email":             parent.Email,
				"code":              parent.Code,
				"code_expires_at":   parent.CodeExpiresAt,
				"translations_info": translationInfo,
			},
		})
		return
	}

	// Для других типов пользователей
	// Определяем translationInfo для всех остальных типов пользователей
	lastUpdateTime, err := translationService.GetLastUpdateTime()
	if err != nil {
		lastUpdateTime = time.Now()
	}

	translationInfo := gin.H{
		"lastUpdatedAt": lastUpdateTime,
		"langUpdate":    true,
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           true,
		"user":              user,
		"translations_info": translationInfo,
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
