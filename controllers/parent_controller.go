package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"

	// Теперь импорт используется
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var parentService *services.ParentService
var translationService *services.TranslationService

func SetParentService(service *services.ParentService) {
	parentService = service
}

func SetTranslationService(service *services.TranslationService) {
	translationService = service
}

func ReadParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	parent, err := parentService.ReadParent(firebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": parent})
}

func UpdateParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var input struct {
		Lang     string `json:"lang"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	parent, err := parentService.ReadParent(firebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	// Сохраняем текущий язык для сравнения
	prevLang := parent.Lang

	if input.Lang != "" {
		parent.Lang = input.Lang
	}
	if input.Name != "" {
		parent.Name = input.Name
	}
	if input.Email != "" {
		parent.Email = input.Email
	}
	if input.Password != "" {
		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		parent.Password = string(hashedPassword)
	}

	updatedParent, err := parentService.UpdateParent(firebaseUID, parent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Подготовка ответа
	response := gin.H{
		"message": "Parent updated successfully",
		"data":    updatedParent,
	}

	// Если язык изменился и установлен translation_service, добавляем переводы к ответу
	if translationService != nil && input.Lang != "" && input.Lang != prevLang {
		translations := translationService.GetAllTranslations(input.Lang)
		response["translations"] = translations
		response["lang_changed"] = true
	}

	c.JSON(http.StatusOK, response)
}

func DeleteParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")

	// 1. Сначала удаляем из Firebase
	ctx := context.Background()

	// Используем GetAuthClient из services вместо firebase
	client, err := services.GetAuthClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка инициализации Firebase: " + err.Error()})
		return
	}

	// Логируем для отладки
	fmt.Printf("Удаление пользователя из Firebase: %s\n", firebaseUID)

	// Пытаемся удалить из Firebase
	err = client.DeleteUser(ctx, firebaseUID)
	if err != nil {
		// Логируем ошибку, но продолжаем (возможно, пользователя уже нет в Firebase)
		fmt.Printf("Ошибка удаления из Firebase: %v\n", err)
	} else {
		fmt.Printf("Пользователь успешно удален из Firebase\n")
	}

	// 2. Затем удаляем из нашей БД
	err = parentService.DeleteParent(firebaseUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления из базы данных: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "Родитель успешно удален из Firebase и базы данных",
	})
}

func UnbindChild(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string `json:"parentFirebaseUid" binding:"required"`
		ChildFirebaseUID  string `json:"childFirebaseUid" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := parentService.UnbindChild(request.ParentFirebaseUID, request.ChildFirebaseUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	parent, err := parentService.ReadParent(request.ParentFirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child unbound successfully", "parent": parent})
}

func MonitorChildrenUsage(c *gin.Context) {
	var input struct {
		FirebaseUID string `json:"firebase_uid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	parent, err := parentService.ReadParent(input.FirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)

	var usageData []map[string]interface{}
	for _, member := range family {
		child, err := parentService.ReadChild(member["firebase_uid"].(string))
		if err == nil {
			var childUsageData map[string]interface{}
			json.Unmarshal([]byte(child.UsageData), &childUsageData)
			usageData = append(usageData, map[string]interface{}{
				"child_id":   child.FirebaseUID,
				"name":       child.Name,
				"usage_data": childUsageData,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "data": usageData})
}

func MonitorChildUsage(c *gin.Context) {
	var input struct {
		ParentFirebaseUID string `json:"parent_firebase_uid" binding:"required"`
		ChildFirebaseUID  string `json:"child_firebase_uid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	_, err := parentService.ReadParent(input.ParentFirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	child, err := parentService.ReadChild(input.ChildFirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}

	// ИСПРАВЛЕНО: Правильно обрабатываем формат данных
	var usageData interface{}

	// Если данные пустые, возвращаем пустой массив вместо null
	if child.UsageData == "" {
		usageData = []interface{}{}
	} else {
		// Проверяем, является ли формат массивом (начинается с '[')
		if len(child.UsageData) > 0 && child.UsageData[0] == '[' {
			var dataArray []interface{}
			if err := json.Unmarshal([]byte(child.UsageData), &dataArray); err == nil {
				// Добавляем информацию о том, что это кумулятивные данные за день
				result := map[string]interface{}{
					"child_id":   child.FirebaseUID,
					"name":       child.Name,
					"usage_data": dataArray,
					"data_type":  "cumulative_daily",                                       // Метка о типе данных
					"day_start":  time.Now().Truncate(24 * time.Hour).Format(time.RFC3339), // Начало текущего дня
				}
				c.JSON(http.StatusOK, gin.H{"message": true, "data": result})
				return
			} else {
				// Логируем ошибку разбора
				fmt.Printf("Error parsing usage data array: %v\n", err)
				usageData = []interface{}{}
			}
		} else {
			// Если не массив, пробуем как объект
			var dataObject map[string]interface{}
			if err := json.Unmarshal([]byte(child.UsageData), &dataObject); err == nil {
				usageData = dataObject
			} else {
				// Логируем ошибку разбора
				fmt.Printf("Error parsing usage data object: %v\n", err)
				usageData = []interface{}{}
			}
		}
	}

	// Формируем результат
	result := map[string]interface{}{
		"child_id":   child.FirebaseUID,
		"name":       child.Name,
		"usage_data": usageData,
		"data_type":  "cumulative_daily",                                       // Метка о типе данных
		"day_start":  time.Now().Truncate(24 * time.Hour).Format(time.RFC3339), // Начало текущего дня
	}
	c.JSON(http.StatusOK, gin.H{"message": true, "data": result})
}
func BlockApps(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string   `json:"parentFirebaseUid" binding:"required"`
		ChildFirebaseUID  string   `json:"childFirebaseUid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := parentService.BlockApps(request.ParentFirebaseUID, request.ChildFirebaseUID, request.Apps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Apps blocked successfully"})
}

func UnblockApps(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string   `json:"parentFirebaseUid" binding:"required"`
		ChildFirebaseUID  string   `json:"childFirebaseUid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := parentService.UnblockApps(request.ParentFirebaseUID, request.ChildFirebaseUID, request.Apps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked successfully"})
}

// BlockAppsTempOnce обрабатывает запрос на одноразовую временную блокировку приложений
func BlockAppsTempOnce(c *gin.Context) {
	var request services.TempBlockRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parentFirebaseUID := c.Param("firebase_uid")

	blocks, err := parentService.BlockAppsTempOnce(parentFirebaseUID, request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Получаем токен устройства ребенка для отправки уведомлений через другой контроллер
	child, err := GetChildData(request.ChildFirebaseUID)
	if err != nil {
		// Логируем ошибку, но продолжаем выполнение
		fmt.Printf("[ERROR] BlockAppsTempOnce: Не удалось найти ребенка для уведомлений: %v\n", err)
	} else if child != nil && child.DeviceToken != "" {
		// Вызываем метод из websocket_controller для отправки уведомления
		fmt.Printf("[WebSocket] BlockAppsTempOnce: Отправка уведомления о смене лимитов для родителя %s, ребенка %s\n",
			parentFirebaseUID, request.ChildFirebaseUID)

		// Асинхронно отправляем уведомление через WebSocket
		go NotifyLimitChange(parentFirebaseUID, child.DeviceToken)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Приложения временно заблокированы",
		"blocks":  blocks,
	})
}

// Вспомогательный метод для получения данных ребенка
func GetChildData(childFirebaseUID string) (*models.Child, error) {
	child, err := childService.ReadChild(childFirebaseUID)
	if err != nil {
		return nil, err
	}
	// Возвращаем указатель на полученную структуру
	return &child, nil
}

// Вспомогательный метод для отправки уведомлений через WebSocket
func NotifyLimitChange(parentID, childToken string) {
	if WebSocketHub == nil {
		fmt.Printf("[WARN] WebSocketHub не инициализирован, уведомление не отправлено\n")
		return
	}

	fmt.Printf("[INFO] Отправка уведомления о смене лимитов через WebSocket: parent=%s, child_token=%s\n",
		parentID, childToken)

	// Вызываем метод NotifyLimitChange
	WebSocketHub.NotifyLimitChange(parentID, childToken)

	fmt.Printf("[INFO] Уведомление о смене лимитов успешно отправлено\n")
}

// GetOneTimeBlocks возвращает список одноразовых блокировок для ребенка
func GetOneTimeBlocks(c *gin.Context) {
	// Получаем ID ребенка из запроса (как параметр URL)
	childID := c.Param("firebase_uid")
	if childID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "child ID is required"})
		return
	}

	// Получаем FirebaseUID родителя напрямую из контекста
	parentFirebaseUID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: missing firebase_uid"})
		return
	}

	// Получаем одноразовые блокировки
	oneTimeBlocks, err := parentService.GetOneTimeBlocks(parentFirebaseUID.(string), childID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()

	// Группируем блоки по времени окончания
	groupedBlocks := make(map[string]map[string]interface{})

	for _, block := range oneTimeBlocks {
		// Создаем ключ для группировки по времени окончания
		key := block.OneTimeEndAt.Format(time.RFC3339)

		// Вычисляем оставшееся время в минутах
		// remainingMin := int(math.Max(0, block.OneTimeEndAt.Sub(now).Minutes()))

		// Получаем исходную длительность из строки Duration
		var originalDurationMin int
		if block.Duration != "" {
			// Пытаемся извлечь числовое значение из строки формата "60 минут"
			_, err := fmt.Sscanf(block.Duration, "%d", &originalDurationMin)
			if err != nil {
				// В случае ошибки оставляем вычисленное оставшееся время
				originalDurationMin = int(math.Max(0, block.OneTimeEndAt.Sub(now).Minutes()))
			}
		}

		// Извлекаем числовое значение из строки Duration
		// var durationMin int
		// if block.OriginalDuration > 0 {
		// 	durationMin = block.OriginalDuration
		// } else {
		// 	// Извлекаем из строки как запасной вариант
		// 	re := regexp.MustCompile(`\d+`)
		// 	matches := re.FindString(block.Duration)
		// 	if matches != "" {
		// 		durationMin, _ = strconv.Atoi(matches)
		// 	}
		// }

		if group, exists := groupedBlocks[key]; exists {
			// Добавляем приложение в существующую группу
			apps := group["apps"].([]string)
			apps = append(apps, block.AppPackage)
			group["apps"] = apps
		} else {
			// Создаем новую группу с информацией в минутах и новым полем duration_min
			groupedBlocks[key] = map[string]interface{}{
				"id":       block.ID,
				"end_time": block.OneTimeEndAt,
				"duration": block.Duration, // "7 минут"
				// "duration_min":  durationMin,    // 7 (как число)
				"block_name":    block.BlockName,
				"is_one_time":   true,
				"remaining_min": block.OriginalDuration, // значение, которое не меняется
				"apps":          []string{block.AppPackage},
			}
		}
	}

	// Преобразуем в массив для возврата
	var result []map[string]interface{}
	for _, group := range groupedBlocks {
		result = append(result, group)
	}

	c.JSON(http.StatusOK, gin.H{"blocks": result})
}

// CancelOneTimeBlocks отменяет одноразовые блокировки для указанных приложений
func CancelOneTimeBlocks(c *gin.Context) {
	var appPackages []string
	if err := c.ShouldBindJSON(&appPackages); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	parentFirebaseUID := c.Param("firebase_uid")
	childFirebaseUID := c.Param("child_id")

	err := parentService.CancelOneTimeBlocks(parentFirebaseUID, childFirebaseUID, appPackages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Временные блокировки успешно отменены",
	})
}

// ManageAppTimeRules обрабатывает как блокировку, так и разблокировку приложений по времени
func ManageAppTimeRules(c *gin.Context) {
	fmt.Println("[ManageAppTimeRules] Начало обработки запроса")

	var request struct {
		ParentFirebaseUID string   `json:"parent_firebase_uid" binding:"required"`
		ChildFirebaseUID  string   `json:"child_firebase_uid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
		Action            string   `json:"action" binding:"required,oneof=block unblock"` // Определяет действие

		// Поддержка старого формата
		StartTime string  `json:"start_time,omitempty"`
		EndTime   string  `json:"end_time,omitempty"`
		BlockIDs  []int64 `json:"block_ids,omitempty"` // Для разблокировки по ID

		// Поддержка нового формата с множественными блоками
		TimeBlocks []struct {
			ID        int64  `json:"id,omitempty"`
			StartTime string `json:"start_time"`
			EndTime   string `json:"end_time"`
			BlockName string `json:"block_name,omitempty"`
		} `json:"time_blocks,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("[ManageAppTimeRules] Ошибка привязки JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Логируем детали запроса
	fmt.Printf("[ManageAppTimeRules] Получены данные: parentUID=%s, childUID=%s, action=%s, apps=%v\n",
		request.ParentFirebaseUID, request.ChildFirebaseUID, request.Action, request.Apps)

	// Проверка типов данных в массиве apps
	fmt.Printf("[ManageAppTimeRules] Детали массива apps (len=%d):\n", len(request.Apps))
	for i, app := range request.Apps {
		fmt.Printf("  - App[%d]: '%s' (тип: %T)\n", i, app, app)
		if strings.Contains(app, ",") {
			fmt.Printf("  ! ВНИМАНИЕ: App[%d] содержит запятые: '%s'\n", i, app)
		}
	}

	// Проверка наличия необходимых для блокировки параметров
	if request.Action == "block" {
		fmt.Println("[ManageAppTimeRules] Обработка действия блокировки")

		// Отслеживаем созданные блоки для возврата ID
		var createdBlocks []map[string]interface{}

		// Проверяем новый формат
		if len(request.TimeBlocks) > 0 {
			fmt.Printf("[ManageAppTimeRules] Используем новый формат с множественными блоками (%d блоков)\n",
				len(request.TimeBlocks))

			// Используем новый формат с множественными блоками
			for i, timeBlock := range request.TimeBlocks {
				fmt.Printf("[ManageAppTimeRules] Обработка блока %d: start=%s, end=%s, name=%s\n",
					i+1, timeBlock.StartTime, timeBlock.EndTime, timeBlock.BlockName)

				for j, app := range request.Apps {
					fmt.Printf("[ManageAppTimeRules] Блокировка приложения %d/%d: %s\n",
						j+1, len(request.Apps), app)

					// Создаем блок времени
					blockID := time.Now().UnixNano() // Генерируем уникальный ID
					fmt.Printf("[ManageAppTimeRules] Сгенерирован ID блока: %d\n", blockID)

					err := parentService.ManageAppTimeRules(
						request.ParentFirebaseUID,
						request.ChildFirebaseUID,
						[]string{app},
						request.Action,
						timeBlock.StartTime,
						timeBlock.EndTime,
						timeBlock.BlockName, // Добавляем название блока
						blockID,             // Передаем ID в метод
					)
					if err != nil {
						fmt.Printf("[ManageAppTimeRules] Ошибка при создании блока: %v\n", err)
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}

					fmt.Println("[ManageAppTimeRules] Блок успешно создан")

					// Добавляем созданный блок в список для ответа
					createdBlocks = append(createdBlocks, map[string]interface{}{
						"id":           blockID,
						"app_package":  app,
						"start_time":   timeBlock.StartTime,
						"end_time":     timeBlock.EndTime,
						"block_name":   timeBlock.BlockName,
						"days_of_week": "1,2,3,4,5,6,7",
					})
				}
			}

			fmt.Printf("[ManageAppTimeRules] Создано всего %d блоков\n", len(createdBlocks))
			fmt.Println("[ManageAppTimeRules] Группировка созданных блоков для ответа")

			// ИСПРАВЛЕНО: Группируем блоки для нового формата так же, как для старого
			groupedBlocks := make(map[string]map[string]interface{})

			for _, block := range createdBlocks {
				// Создаем ключ для группировки
				key := fmt.Sprintf("%s_%s_%s_%s",
					block["start_time"],
					block["end_time"],
					block["block_name"],
					block["days_of_week"])

				fmt.Printf("[ManageAppTimeRules] Обработка блока для группировки: app=%s, ключ=%s\n",
					block["app_package"].(string), key)

				if group, exists := groupedBlocks[key]; exists {
					// Добавляем приложение в существующую группу
					apps := group["apps"].([]string)
					apps = append(apps, block["app_package"].(string))
					group["apps"] = apps
					fmt.Printf("[ManageAppTimeRules] Добавлено приложение %s в существующую группу\n",
						block["app_package"].(string))
				} else {
					// Создаем новую группу
					fmt.Printf("[ManageAppTimeRules] Создание новой группы для ключа: %s\n", key)
					groupedBlocks[key] = map[string]interface{}{
						"id":           block["id"],
						"start_time":   block["start_time"],
						"end_time":     block["end_time"],
						"block_name":   block["block_name"],
						"days_of_week": block["days_of_week"],
						"apps":         []string{block["app_package"].(string)},
					}
				}
			}

			// Преобразуем в массив для ответа
			var groupedResult []map[string]interface{}
			for key, group := range groupedBlocks {
				fmt.Printf("[ManageAppTimeRules] Добавление группы '%s' в результат\n", key)
				groupedResult = append(groupedResult, group)
			}

			fmt.Printf("[ManageAppTimeRules] Итоговый ответ содержит %d групп блоков\n", len(groupedResult))

			c.JSON(http.StatusOK, gin.H{
				"message": "Apps blocked by time successfully",
				"blocks":  groupedResult,
			})
			return
		}

		// Проверяем старый формат
		if request.StartTime == "" || request.EndTime == "" {
			fmt.Println("[ManageAppTimeRules] Ошибка: не указаны start_time и end_time для старого формата")
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_time and end_time are required for block action"})
			return
		}

		fmt.Printf("[ManageAppTimeRules] Используем старый формат: start_time=%s, end_time=%s\n",
			request.StartTime, request.EndTime)

		// Используем старый формат с генерацией ID
		for i, app := range request.Apps {
			blockID := time.Now().UnixNano() + int64(i) // Генерируем уникальный ID
			fmt.Printf("[ManageAppTimeRules] Блокировка приложения %d/%d: %s (ID=%d)\n",
				i+1, len(request.Apps), app, blockID)

			err := parentService.ManageAppTimeRules(
				request.ParentFirebaseUID,
				request.ChildFirebaseUID,
				[]string{app},
				request.Action,
				request.StartTime,
				request.EndTime,
				"",
				blockID, // Передаем ID в метод
			)

			if err != nil {
				fmt.Printf("[ManageAppTimeRules] Ошибка при создании блока: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			fmt.Printf("[ManageAppTimeRules] Блок для приложения %s успешно создан\n", app)

			// Добавляем созданный блок в список для ответа
			createdBlocks = append(createdBlocks, map[string]interface{}{
				"id":           blockID,
				"app_package":  app,
				"start_time":   request.StartTime,
				"end_time":     request.EndTime,
				"days_of_week": "1,2,3,4,5,6,7",
			})
		}

		fmt.Printf("[ManageAppTimeRules] Создано всего %d блоков по старому формату\n", len(createdBlocks))
		fmt.Println("[ManageAppTimeRules] Группировка созданных блоков для ответа")

		// Вместо возврата массива блоков, группируем их:
		groupedBlocks := make(map[string]map[string]interface{})

		for _, block := range createdBlocks {
			// Создаем ключ для группировки
			key := fmt.Sprintf("%s_%s_%s_%s",
				block["start_time"],
				block["end_time"],
				block["block_name"],
				block["days_of_week"])

			fmt.Printf("[ManageAppTimeRules] Обработка блока для группировки: app=%s, ключ=%s\n",
				block["app_package"].(string), key)

			if group, exists := groupedBlocks[key]; exists {
				// Добавляем приложение в существующую группу
				apps := group["apps"].([]string)
				apps = append(apps, block["app_package"].(string))
				group["apps"] = apps
				fmt.Printf("[ManageAppTimeRules] Добавлено приложение %s в существующую группу\n",
					block["app_package"].(string))
			} else {
				// Создаем новую группу
				fmt.Printf("[ManageAppTimeRules] Создание новой группы для ключа: %s\n", key)
				groupedBlocks[key] = map[string]interface{}{
					"id":           block["id"],
					"start_time":   block["start_time"],
					"end_time":     block["end_time"],
					"block_name":   block["block_name"],
					"days_of_week": block["days_of_week"],
					"apps":         []string{block["app_package"].(string)},
				}
			}
		}

		fmt.Printf("[ManageAppTimeRules] Создано %d групп блоков\n", len(groupedBlocks))
		fmt.Println("[ManageAppTimeRules] Отправка успешного ответа")

		c.JSON(http.StatusOK, gin.H{
			"message": "Apps blocked by time successfully",
			"blocks":  groupedBlocks,
		})
	} else if request.Action == "unblock" {
		fmt.Println("[ManageAppTimeRules] Обработка действия разблокировки")

		// Проверяем, есть ли ID блоков для разблокировки
		if len(request.BlockIDs) > 0 {
			fmt.Printf("[ManageAppTimeRules] Разблокировка по ID блоков: %v\n", request.BlockIDs)

			// Используем обновленный метод ManageAppTimeRules для разблокировки по ID
			err := parentService.ManageAppTimeRules(
				request.ParentFirebaseUID,
				request.ChildFirebaseUID,
				[]string{}, // Пустой список, т.к. используем ID
				request.Action,
				"", // Для разблокировки время не нужно
				"", // Для разблокировки время не нужно
				"", // Для разблокировки по ID
				request.BlockIDs...,
			)

			if err != nil {
				fmt.Printf("[ManageAppTimeRules] Ошибка при разблокировке по ID: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			fmt.Println("[ManageAppTimeRules] Разблокировка по ID успешно выполнена")

			c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked by block IDs successfully"})
			return
		}

		// Стандартная разблокировка по имени приложения
		fmt.Printf("[ManageAppTimeRules] Разблокировка по именам приложений: %v\n", request.Apps)

		err := parentService.ManageAppTimeRules(
			request.ParentFirebaseUID,
			request.ChildFirebaseUID,
			request.Apps,
			request.Action,
			"", // Для разблокировки время не нужно
			"", // Для разблокировки время не нужно,
			"", // Добавляем пустое название блока
		)

		if err != nil {
			fmt.Printf("[ManageAppTimeRules] Ошибка при разблокировке приложений: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		fmt.Println("[ManageAppTimeRules] Разблокировка приложений успешно выполнена")

		c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked by time successfully"})
	}

	fmt.Println("[ManageAppTimeRules] Завершение обработки запроса")
}

// ManageOneTimeRules обрабатывает как создание, так и отмену одноразовой блокировки приложений
func ManageOneTimeRules(c *gin.Context) {
	fmt.Println("[ManageOneTimeRules] Начало обработки запроса")

	var request struct {
		ParentFirebaseUID string   `json:"parent_firebase_uid" binding:"required"`
		ChildFirebaseUID  string   `json:"child_firebase_uid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
		Action            string   `json:"action" binding:"required,oneof=block unblock"` // Определяет действие

		// Параметры для блокировки
		DurationMins int    `json:"duration_mins" binding:"required_if=action block"`
		BlockName    string `json:"block_name,omitempty"`

		// Параметры для разблокировки
		BlockIDs []int64 `json:"block_ids,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		fmt.Printf("[ManageOneTimeRules] Ошибка привязки JSON: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Логируем детали запроса
	fmt.Printf("[ManageOneTimeRules] Получены данные: parentUID=%s, childUID=%s, action=%s, duration=%d, apps=%v\n",
		request.ParentFirebaseUID, request.ChildFirebaseUID, request.Action, request.DurationMins, request.Apps)

	// Проверка типов данных в массиве apps
	fmt.Printf("[ManageOneTimeRules] Детали массива apps (len=%d):\n", len(request.Apps))
	for i, app := range request.Apps {
		fmt.Printf("  - App[%d]: '%s' (тип: %T)\n", i, app, app)
		if strings.Contains(app, ",") {
			fmt.Printf("  ! ВНИМАНИЕ: App[%d] содержит запятые: '%s'\n", i, app)
		}
	}

	if request.Action == "block" {
		fmt.Printf("[ManageOneTimeRules] Начинаем блокировку, duration_mins=%d\n", request.DurationMins)

		// Проверяем, что duration_mins не отрицательное
		if request.DurationMins < 0 {
			fmt.Println("[ManageOneTimeRules] Ошибка: отрицательное значение duration_mins")
			c.JSON(http.StatusBadRequest, gin.H{"error": "duration_mins cannot be negative"})
			return
		}

		// Логика для постоянной блокировки
		if request.DurationMins == 0 || request.DurationMins >= 10080 {
			fmt.Println("[ManageOneTimeRules] Обнаружена постоянная блокировка (duration_mins=0 или >=10080)")

			// Сначала получаем текущие одноразовые блокировки
			fmt.Printf("[ManageOneTimeRules] Получаем текущие блокировки для parent=%s, child=%s\n",
				request.ParentFirebaseUID, request.ChildFirebaseUID)

			currentBlocks, err := parentService.GetOneTimeBlocks(
				request.ParentFirebaseUID,
				request.ChildFirebaseUID,
			)
			if err != nil {
				fmt.Printf("[ManageOneTimeRules] Ошибка при получении блокировок: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			fmt.Printf("[ManageOneTimeRules] Получено %d существующих блокировок\n", len(currentBlocks))

			// Собираем ID блоков для указанных приложений
			var blocksToCancel []int64
			for _, block := range currentBlocks {
				for _, app := range request.Apps {
					if block.AppPackage == app {
						fmt.Printf("[ManageOneTimeRules] Найдена существующая блокировка для %s (ID=%d)\n",
							app, block.ID)
						blocksToCancel = append(blocksToCancel, block.ID)
						break
					}
				}
			}
			fmt.Printf("[ManageOneTimeRules] Найдено %d блокировок для отмены\n", len(blocksToCancel))

			// Отменяем существующие блокировки, если они есть
			if len(blocksToCancel) > 0 {
				fmt.Printf("[ManageOneTimeRules] Отменяем блокировки с ID: %v\n", blocksToCancel)
				err := parentService.CancelOneTimeBlocksByIDs(
					request.ParentFirebaseUID,
					request.ChildFirebaseUID,
					blocksToCancel,
				)
				if err != nil {
					fmt.Printf("[ManageOneTimeRules] Ошибка при отмене блокировок: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				fmt.Println("[ManageOneTimeRules] Успешно отменены существующие блокировки")
			}
		}

		// Создаем объект запроса для BlockAppsTempOnce с минутами
		fmt.Printf("[ManageOneTimeRules] Создаем запрос на блокировку для %d приложений, duration=%d\n",
			len(request.Apps), request.DurationMins)
		tempBlockRequest := services.TempBlockRequest{
			ChildFirebaseUID: request.ChildFirebaseUID,
			AppPackages:      request.Apps,
			DurationMins:     request.DurationMins,
			BlockName:        request.BlockName,
		}

		// Вызываем метод блокировки в сервисе
		fmt.Println("[ManageOneTimeRules] Вызываем BlockAppsTempOnce")
		blocks, err := parentService.BlockAppsTempOnce(
			request.ParentFirebaseUID,
			tempBlockRequest,
		)

		if err != nil {
			fmt.Printf("[ManageOneTimeRules] Ошибка при блокировке: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		fmt.Printf("[ManageOneTimeRules] BlockAppsTempOnce вернул %d блоков\n", len(blocks))

		// Группируем результат для ответа
		if len(blocks) > 0 {
			fmt.Println("[ManageOneTimeRules] Группируем блоки для ответа")
			// Группируем блоки по времени окончания
			groupedBlocks := make(map[string]map[string]interface{})

			for _, block := range blocks {
				// Создаем ключ для группировки по времени окончания
				key := block.OneTimeEndAt.Format(time.RFC3339)
				fmt.Printf("[ManageOneTimeRules] Обработка блока: app=%s, endTime=%s, key=%s\n",
					block.AppPackage, block.OneTimeEndAt, key)

				if group, exists := groupedBlocks[key]; exists {
					// Добавляем приложение в существующую группу
					apps := group["apps"].([]string)
					apps = append(apps, block.AppPackage)
					group["apps"] = apps
					fmt.Printf("[ManageOneTimeRules] Добавлено приложение %s в существующую группу\n", block.AppPackage)
				} else {
					// Создаем новую группу с информацией в минутах и новым полем duration_min
					fmt.Printf("[ManageOneTimeRules] Создаем новую группу для key=%s\n", key)
					groupedBlocks[key] = map[string]interface{}{
						"id":            block.ID,
						"end_time":      block.OneTimeEndAt,
						"duration":      block.Duration,       // "7 минут"
						"duration_min":  request.DurationMins, // 7 (как число)
						"block_name":    block.BlockName,
						"apps":          []string{block.AppPackage},
						"remaining_min": request.DurationMins, // не забудьте сохранить это поле для обратной совместимости
						"is_one_time":   true,
					}
				}
			}

			// Преобразуем в массив для ответа
			var result []map[string]interface{}
			for key, group := range groupedBlocks {
				fmt.Printf("[ManageOneTimeRules] Добавление группы '%s' в результат\n", key)
				result = append(result, group)
			}
			fmt.Printf("[ManageOneTimeRules] Итоговый результат: %d групп блокировок\n", len(result))

			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Apps blocked temporarily",
				"blocks":  result,
			})
		} else {
			fmt.Println("[ManageOneTimeRules] Не создано ни одной блокировки, возможно приложения уже заблокированы")
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "No blocks created",
			})
		}
	} else if request.Action == "unblock" {
		fmt.Println("[ManageOneTimeRules] Начинаем разблокировку")

		if len(request.BlockIDs) > 0 {
			fmt.Printf("[ManageOneTimeRules] Разблокировка по ID блоков: %v\n", request.BlockIDs)

			// Если указаны конкретные приложения, отменяем блокировки только для них
			if len(request.Apps) > 0 {
				fmt.Printf("[ManageOneTimeRules] Разблокировка конкретных приложений: %v\n", request.Apps)
				err := parentService.CancelOneTimeBlocks(
					request.ParentFirebaseUID,
					request.ChildFirebaseUID,
					request.Apps,
				)

				if err != nil {
					fmt.Printf("[ManageOneTimeRules] Ошибка при разблокировке приложений: %v\n", err)
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				fmt.Println("[ManageOneTimeRules] Успешная разблокировка приложений")
				c.JSON(http.StatusOK, gin.H{
					"status":  "success",
					"message": "One-time blocks successfully canceled for specified apps",
				})
				return
			}

			// Отменяем все одноразовые блокировки, связанные с указанными ID блоков
			fmt.Printf("[ManageOneTimeRules] Разблокировка по ID блоков: %v\n", request.BlockIDs)
			err := parentService.CancelOneTimeBlocksByIDs(
				request.ParentFirebaseUID,
				request.ChildFirebaseUID,
				request.BlockIDs,
			)

			if err != nil {
				fmt.Printf("[ManageOneTimeRules] Ошибка при разблокировке по ID: %v\n", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			fmt.Println("[ManageOneTimeRules] Успешная разблокировка по ID")
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "One-time blocks successfully canceled",
			})
		} else {
			fmt.Println("[ManageOneTimeRules] Ошибка: не указаны block_ids для разблокировки")
			c.JSON(http.StatusBadRequest, gin.H{"error": "block_ids are required for unblock action"})
		}
	}

	fmt.Println("[ManageOneTimeRules] Завершение обработки запроса")
}

// VerifyParentEmail проверяет код подтверждения email
func VerifyParentEmail(c *gin.Context) {
	// Получаем firebase_uid из контекста (который был установлен middleware)
	firebaseUID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Необходима авторизация"})
		return
	}

	// Получаем только код из запроса
	var input struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Вызываем сервис для проверки кода
	err := parentService.VerifyEmail(firebaseUID.(string), input.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": false,
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "Email успешно подтвержден",
	})
}

// ResendVerificationCode также использует токен
func ResendVerificationCode(c *gin.Context) {
	// Получаем firebase_uid из контекста (который был установлен middleware)
	firebaseUID, exists := c.Get("firebase_uid")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Необходима авторизация"})
		return
	}

	// Никаких дополнительных данных не требуется в запросе

	parent, err := parentService.ReadParent(firebaseUID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": false,
			"error":  "Родитель не найден",
		})
		return
	}

	err = parentService.SendVerificationCode(&parent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": false,
			"error":  fmt.Sprintf("Ошибка отправки кода: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  true,
		"message": "Новый код подтверждения отправлен на вашу почту",
	})
}
