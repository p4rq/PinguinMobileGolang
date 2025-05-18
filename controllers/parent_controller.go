package controllers

import (
	"PinguinMobile/services"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var parentService *services.ParentService

func SetParentService(service *services.ParentService) {
	parentService = service
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

	c.JSON(http.StatusOK, gin.H{"message": "Parent updated successfully", "data": updatedParent})
}

func DeleteParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	if err := parentService.DeleteParent(firebaseUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Parent deleted successfully"})
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

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Приложения временно заблокированы",
		"blocks":  blocks,
	})
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
		remainingMin := int(math.Max(0, block.OneTimeEndAt.Sub(now).Minutes()))

		if group, exists := groupedBlocks[key]; exists {
			// Добавляем приложение в существующую группу
			apps := group["apps"].([]string)
			apps = append(apps, block.AppPackage)
			group["apps"] = apps
		} else {
			// Создаем новую группу без поля duration
			groupedBlocks[key] = map[string]interface{}{
				"id":            block.ID,
				"end_time":      block.OneTimeEndAt,
				"remaining_min": remainingMin, // Оставшееся время в минутах
				"apps":          []string{block.AppPackage},
				"block_name":    block.BlockName, // Сохраняем название блока
				"is_one_time":   true,
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверка наличия необходимых для блокировки параметров
	if request.Action == "block" {
		// Отслеживаем созданные блоки для возврата ID
		var createdBlocks []map[string]interface{}

		// Проверяем новый формат
		if len(request.TimeBlocks) > 0 {
			// Используем новый формат с множественными блоками
			for _, timeBlock := range request.TimeBlocks {
				for _, app := range request.Apps {
					// Создаем блок времени
					blockID := time.Now().UnixNano() // Генерируем уникальный ID

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
						c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
						return
					}

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

			// ИСПРАВЛЕНО: Группируем блоки для нового формата так же, как для старого
			groupedBlocks := make(map[string]map[string]interface{})

			for _, block := range createdBlocks {
				// Создаем ключ для группировки
				key := fmt.Sprintf("%s_%s_%s_%s",
					block["start_time"],
					block["end_time"],
					block["block_name"],
					block["days_of_week"])

				if group, exists := groupedBlocks[key]; exists {
					// Добавляем приложение в существующую группу
					apps := group["apps"].([]string)
					apps = append(apps, block["app_package"].(string))
					group["apps"] = apps
				} else {
					// Создаем новую группу
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
			for _, group := range groupedBlocks {
				groupedResult = append(groupedResult, group)
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Apps blocked by time successfully",
				"blocks":  groupedResult,
			})
			return
		}

		// Проверяем старый формат
		if request.StartTime == "" || request.EndTime == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "start_time and end_time are required for block action"})
			return
		}

		// Используем старый формат с генерацией ID
		for _, app := range request.Apps {
			blockID := time.Now().UnixNano() + int64(len(createdBlocks)) // Генерируем уникальный ID

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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Добавляем созданный блок в список для ответа
			createdBlocks = append(createdBlocks, map[string]interface{}{
				"id":           blockID,
				"app_package":  app,
				"start_time":   request.StartTime,
				"end_time":     request.EndTime,
				"days_of_week": "1,2,3,4,5,6,7",
			})
		}

		// Вместо возврата массива блоков, группируем их:
		groupedBlocks := make(map[string]map[string]interface{})

		for _, block := range createdBlocks {
			// Создаем ключ для группировки
			key := fmt.Sprintf("%s_%s_%s_%s",
				block["start_time"],
				block["end_time"],
				block["block_name"],
				block["days_of_week"])

			if group, exists := groupedBlocks[key]; exists {
				// Добавляем приложение в существующую группу
				apps := group["apps"].([]string)
				apps = append(apps, block["app_package"].(string))
				group["apps"] = apps
			} else {
				// Создаем новую группу
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

		c.JSON(http.StatusOK, gin.H{
			"message": "Apps blocked by time successfully",
			"blocks":  groupedBlocks,
		})
	} else if request.Action == "unblock" {
		// Проверяем, есть ли ID блоков для разблокировки
		if len(request.BlockIDs) > 0 {
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
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked by block IDs successfully"})
			return
		}

		// Стандартная разблокировка по имени приложения
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked by time successfully"})
	}
}

// ManageOneTimeRules обрабатывает как создание, так и отмену одноразовой блокировки приложений
func ManageOneTimeRules(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string   `json:"parent_firebase_uid" binding:"required"`
		ChildFirebaseUID  string   `json:"child_firebase_uid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
		Action            string   `json:"action" binding:"required,oneof=block unblock"` // Определяет действие

		// Параметры для блокировки - только минуты
		DurationMinutes int    `json:"duration_mins" binding:"required_if=Action block"`
		BlockName       string `json:"block_name,omitempty"` // Добавляем название блока

		// Параметры для разблокировки
		BlockIDs []int64 `json:"block_ids,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Action == "block" {
		// Проверка наличия необходимого параметра
		if request.DurationMinutes <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duration_mins is required for block action"})
			return
		}

		// Создаем объект запроса для BlockAppsTempOnce с минутами
		tempBlockRequest := services.TempBlockRequest{
			ChildFirebaseUID: request.ChildFirebaseUID,
			AppPackages:      request.Apps,
			DurationMins:     request.DurationMinutes,
			BlockName:        request.BlockName,
		}

		// Вызываем метод блокировки в сервисе с правильными аргументами
		blocks, err := parentService.BlockAppsTempOnce(
			request.ParentFirebaseUID,
			tempBlockRequest,
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Группируем результат для ответа
		if len(blocks) > 0 {
			// Группируем блоки по времени окончания
			groupedBlocks := make(map[string]map[string]interface{})

			for _, block := range blocks {
				// Создаем ключ для группировки по времени окончания
				key := block.OneTimeEndAt.Format(time.RFC3339)

				if group, exists := groupedBlocks[key]; exists {
					// Добавляем приложение в существующую группу
					apps := group["apps"].([]string)
					apps = append(apps, block.AppPackage)
					group["apps"] = apps
				} else {
					// Создаем новую группу с информацией в минутах
					groupedBlocks[key] = map[string]interface{}{
						"id":            block.ID,
						"end_time":      block.OneTimeEndAt,
						"duration":      block.Duration,
						"block_name":    block.BlockName,
						"apps":          []string{block.AppPackage},
						"remaining_min": request.DurationMinutes,
						"is_one_time":   true,
					}
				}
			}

			// Преобразуем в массив для ответа
			var result []map[string]interface{}
			for _, group := range groupedBlocks {
				result = append(result, group)
			}

			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Apps blocked temporarily",
				"blocks":  result,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "No blocks created",
			})
		}
	} else if request.Action == "unblock" {
		if len(request.BlockIDs) > 0 {
			// Если указаны конкретные приложения, отменяем блокировки только для них
			if len(request.Apps) > 0 {
				err := parentService.CancelOneTimeBlocks(
					request.ParentFirebaseUID,
					request.ChildFirebaseUID,
					request.Apps,
				)

				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"status":  "success",
					"message": "One-time blocks successfully canceled for specified apps",
				})
				return
			}

			// Отменяем все одноразовые блокировки, связанные с указанными ID блоков
			err := parentService.CancelOneTimeBlocksByIDs(
				request.ParentFirebaseUID,
				request.ChildFirebaseUID,
				request.BlockIDs,
			)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "One-time blocks successfully canceled",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "block_ids are required for unblock action"})
		}
	}
}
