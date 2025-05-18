package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type ParentService struct {
	ParentRepo repositories.ParentRepository
	ChildRepo  repositories.ChildRepository
}

func NewParentService(parentRepo repositories.ParentRepository, childRepo repositories.ChildRepository) *ParentService {
	return &ParentService{ParentRepo: parentRepo, ChildRepo: childRepo}
}

func (s *ParentService) ReadParent(firebaseUID string) (models.Parent, error) {
	return s.ParentRepo.FindByFirebaseUID(firebaseUID)
}

func (s *ParentService) UpdateParent(firebaseUID string, input models.Parent) (models.Parent, error) {
	parent, err := s.ParentRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return models.Parent{}, err
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

	if err := s.ParentRepo.Save(parent); err != nil {
		return models.Parent{}, err
	}

	return parent, nil
}

func (s *ParentService) DeleteParent(firebaseUID string) error {
	return s.ParentRepo.DeleteByFirebaseUID(firebaseUID)
}

func (s *ParentService) ReadChild(firebaseUID string) (models.Child, error) {
	return s.ChildRepo.FindByFirebaseUID(firebaseUID)
}

func (s *ParentService) UpdateChild(child models.Child) error {
	return s.ChildRepo.Save(child)
}
func (s *ParentService) UnbindChild(parentFirebaseUID, childFirebaseUID string) error {
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return errors.New("parent not found")
	}

	var family []map[string]interface{}
	if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
		return errors.New("failed to parse family JSON")
	}

	childIndex := -1
	for i, member := range family {
		if member["firebase_uid"] == childFirebaseUID {
			childIndex = i
			break
		}
	}
	if childIndex == -1 {
		return errors.New("child not found in parent's family")
	}

	// Remove the child from the family array
	family = append(family[:childIndex], family[childIndex+1:]...)
	familyJson, err := json.Marshal(family)
	if err != nil {
		return errors.New("failed to marshal family JSON")
	}
	parent.Family = string(familyJson)

	// Update the parent in the database
	if err := s.ParentRepo.Save(parent); err != nil {
		return err
	}

	// Update the child in the database
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return errors.New("child not found")
	}
	child.IsBinded = false
	child.Family = "[]"
	if err := s.ChildRepo.Save(child); err != nil {
		return err
	}

	return nil
}

func (s *ParentService) MonitorChildrenUsage(firebaseUID string) ([]map[string]interface{}, error) {
	parent, err := s.ParentRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return nil, err
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)

	var usageData []map[string]interface{}
	for _, member := range family {
		child, err := s.ChildRepo.FindByFirebaseUID(member["firebase_uid"].(string))
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

	return usageData, nil
}

func (s *ParentService) MonitorChildUsage(parentFirebaseUID, childFirebaseUID string) (map[string]interface{}, error) {
	_, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return nil, err
	}

	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return nil, err
	}

	var usageData map[string]interface{}
	json.Unmarshal([]byte(child.UsageData), &usageData)
	usageData = map[string]interface{}{
		"child_id":   child.FirebaseUID,
		"name":       child.Name,
		"usage_data": usageData,
	}

	return usageData, nil
}

func (s *ParentService) BlockApps(parentFirebaseUID, childFirebaseUID string, apps []string) error {
	_, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return errors.New("parent not found")
	}

	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return errors.New("child not found")
	}

	var blockedApps []string
	if child.BlockedApps != "" {
		json.Unmarshal([]byte(child.BlockedApps), &blockedApps)
	}

	blockedApps = append(blockedApps, apps...)
	blockedAppsJson, err := json.Marshal(blockedApps)
	if err != nil {
		return errors.New("failed to marshal blocked apps JSON")
	}

	child.BlockedApps = string(blockedAppsJson)
	if err := s.ChildRepo.Save(child); err != nil {
		return err
	}

	return nil
}

// Метод для разблокировки приложений
func (s *ParentService) UnblockApps(parentFirebaseUID, childFirebaseUID string, apps []string) error {
	_, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return errors.New("parent not found")
	}

	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return errors.New("child not found")
	}

	var blockedApps []string
	if child.BlockedApps != "" {
		json.Unmarshal([]byte(child.BlockedApps), &blockedApps)
	}

	// Создаем карту для быстрого поиска
	appsToUnblock := make(map[string]bool)
	for _, app := range apps {
		appsToUnblock[app] = true
	}

	// Формируем новый список, исключая разблокированные
	newBlockedApps := []string{}
	for _, app := range blockedApps {
		if !appsToUnblock[app] {
			newBlockedApps = append(newBlockedApps, app)
		}
	}

	blockedAppsJson, err := json.Marshal(newBlockedApps)
	if err != nil {
		return errors.New("failed to marshal blocked apps JSON")
	}

	child.BlockedApps = string(blockedAppsJson)
	if err := s.ChildRepo.Save(child); err != nil {
		return err
	}

	return nil
}

// BlockAppsByTime блокирует приложения на определенное время

// UnblockAppsByTime отменяет временную блокировку приложений

// GetTimeBlockedApps возвращает список приложений с временной блокировкой
func (s *ParentService) GetTimeBlockedApps(parentFirebaseUID string, childFirebaseUID string) ([]models.AppTimeBlock, error) {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return nil, errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return nil, errors.New("child not found")
	}

	// Проверяем связь родитель-ребенок через Family JSON
	if !s.isChildInFamily(parent, childFirebaseUID) {
		return nil, errors.New("child does not belong to this parent")
	}

	// Получаем список блокировок
	return s.ChildRepo.GetTimeBlockedApps(child.ID)
}

// isChildInFamily проверяет, принадлежит ли ребенок семье родителя
func (s *ParentService) isChildInFamily(parent models.Parent, childFirebaseUID string) bool {
	// Проверяем, что Family не пустой
	if parent.Family == "" {
		return false
	}

	// Парсим JSON
	var family []map[string]interface{}
	if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
		return false
	}

	// Проверяем, есть ли ребенок в семье
	for _, member := range family {
		if firebaseUID, ok := member["firebase_uid"].(string); ok && firebaseUID == childFirebaseUID {
			return true
		}
	}

	return false
}

// Добавьте структуру запроса для одноразовой блокировки
type TempBlockRequest struct {
	ChildFirebaseUID string   `json:"child_firebase_uid" binding:"required"`
	AppPackages      []string `json:"app_packages" binding:"required"`
	DurationHours    float64  `json:"duration_hours" binding:"required,min=0.5,max=24"`
}

// BlockAppsTempOnce блокирует приложения одноразово на указанное количество часов
func (s *ParentService) BlockAppsTempOnce(parentFirebaseUID string, request TempBlockRequest) ([]models.AppTimeBlock, error) {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return nil, errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(request.ChildFirebaseUID)
	if err != nil {
		return nil, errors.New("child not found")
	}

	// Проверяем связь родитель-ребенок через Family JSON
	if !s.isChildInFamily(parent, request.ChildFirebaseUID) {
		return nil, errors.New("child does not belong to this parent")
	}

	// Вычисляем время окончания блокировки
	now := time.Now()
	endTime := now.Add(time.Duration(request.DurationHours * float64(time.Hour)))

	// Установить более осмысленные значения для StartTime и EndTime
	startTimeStr := now.Format("15:04")
	endTimeStr := endTime.Format("15:04")

	// Создаем новые блоки для одноразовой блокировки
	var newBlocks []models.AppTimeBlock
	for _, appPackage := range request.AppPackages {
		block := models.AppTimeBlock{
			ID:           time.Now().UnixNano(), // Генерируем ID
			AppPackage:   appPackage,
			StartTime:    startTimeStr, // Текущее время как начало блокировки
			EndTime:      endTimeStr,   // Рассчитанное время окончания
			DaysOfWeek:   "1,2,3,4,5,6,7",
			IsOneTime:    true,
			OneTimeEndAt: endTime,
			Duration:     formatDuration(request.DurationHours),
		}
		newBlocks = append(newBlocks, block)
	}

	// Получаем существующие блокировки
	existingBlocks, err := s.ChildRepo.GetTimeBlockedApps(child.ID)
	if err != nil {
		return nil, err
	}

	// Фильтруем существующие блоки, убирая ранее созданные одноразовые блокировки
	// для тех же приложений
	var filteredBlocks []models.AppTimeBlock
	appsMap := make(map[string]bool)
	for _, app := range request.AppPackages {
		appsMap[app] = true
	}

	for _, block := range existingBlocks {
		// Пропускаем одноразовые блоки для тех же приложений
		if block.IsOneTime && appsMap[block.AppPackage] {
			continue
		}
		filteredBlocks = append(filteredBlocks, block)
	}

	// Объединяем отфильтрованные существующие и новые блоки
	allBlocks := append(filteredBlocks, newBlocks...)

	// Сохраняем обновленный список блоков
	if err := s.ChildRepo.RemoveAllTimeBlockedApps(child.ID); err != nil {
		return nil, err
	}

	if err := s.ChildRepo.AddTimeBlockedApps(child.ID, allBlocks); err != nil {
		return nil, err
	}

	// Возвращаем созданные блокировки
	return newBlocks, nil
}

// GetOneTimeBlocks возвращает список активных одноразовых блокировок для ребенка
func (s *ParentService) GetOneTimeBlocks(parentFirebaseUID, childFirebaseUID string) ([]models.AppTimeBlock, error) {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return nil, errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return nil, errors.New("child not found")
	}

	// Проверяем связь родитель-ребенок через Family JSON
	if !s.isChildInFamily(parent, childFirebaseUID) {
		return nil, errors.New("child does not belong to this parent")
	}

	// Получаем все временные блокировки
	allBlocks, err := s.ChildRepo.GetTimeBlockedApps(child.ID)
	if err != nil {
		return nil, err
	}

	// Фильтруем только одноразовые блокировки, которые еще активны
	var oneTimeBlocks []models.AppTimeBlock
	now := time.Now()
	for _, block := range allBlocks {
		if block.IsOneTime && block.OneTimeEndAt.After(now) {
			oneTimeBlocks = append(oneTimeBlocks, block)
		}
	}

	return oneTimeBlocks, nil
}

// CancelOneTimeBlocks отменяет одноразовые блокировки для указанных приложений
func (s *ParentService) CancelOneTimeBlocks(parentFirebaseUID, childFirebaseUID string, appPackages []string) error {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return errors.New("child not found")
	}

	// Проверяем связь родитель-ребенок через Family JSON
	if !s.isChildInFamily(parent, childFirebaseUID) {
		return errors.New("child does not belong to this parent")
	}

	// Получаем все временные блокировки
	allBlocks, err := s.ChildRepo.GetTimeBlockedApps(child.ID)
	if err != nil {
		return err
	}

	// Создаем карту приложений для отмены блокировки
	appsToCancel := make(map[string]bool)
	for _, app := range appPackages {
		appsToCancel[app] = true
	}

	// Фильтруем блокировки, удаляя одноразовые для указанных приложений
	var updatedBlocks []models.AppTimeBlock
	for _, block := range allBlocks {
		// Оставляем блок, если это не одноразовая блокировка или приложение не в списке для отмены
		if !block.IsOneTime || !appsToCancel[block.AppPackage] {
			updatedBlocks = append(updatedBlocks, block)
		}
	}

	// Удаляем все блокировки и добавляем обновленные
	if err := s.ChildRepo.RemoveAllTimeBlockedApps(child.ID); err != nil {
		return err
	}

	if len(updatedBlocks) > 0 {
		if err := s.ChildRepo.AddTimeBlockedApps(child.ID, updatedBlocks); err != nil {
			return err
		}
	}

	return nil
}

// CancelOneTimeBlocksByIDs отменяет одноразовые блокировки по их ID
func (s *ParentService) CancelOneTimeBlocksByIDs(parentUID, childUID string, blockIDs []int64) error {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentUID)
	if err != nil {
		return errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childUID)
	if err != nil {
		return errors.New("child not found")
	}

	// Проверяем, принадлежит ли ребенок родителю
	if !s.isChildInFamily(parent, childUID) {
		return errors.New("child does not belong to this parent")
	}

	// Получаем текущие блоки
	existingBlocks, err := s.GetOneTimeBlocksFromDB(child.ID)
	if err != nil {
		return err
	}

	// Создаем карту ID для быстрой проверки
	idsToRemove := make(map[int64]bool)
	for _, id := range blockIDs {
		idsToRemove[id] = true
	}

	// Фильтруем блоки - оставляем только те, которых нет в списке удаления
	var updatedBlocks []models.AppTimeBlock
	for _, block := range existingBlocks {
		if !idsToRemove[block.ID] {
			updatedBlocks = append(updatedBlocks, block)
		}
	}

	// Сохраняем обновленный список
	return s.SaveOneTimeBlocksToDB(child.ID, updatedBlocks)
}

// formatDuration форматирует продолжительность в часах в человекочитаемый формат
func formatDuration(hours float64) string {
	if hours < 1 {
		return fmt.Sprintf("%.0f минут", hours*60)
	} else if hours == 1 {
		return "1 час"
	} else if hours < 5 {
		return fmt.Sprintf("%.1f часа", hours)
	} else {
		return fmt.Sprintf("%.1f часов", hours)
	}
}

// MonitorChildWithDailyData сохраняет кумулятивные данные использования устройства за текущий день
func (s *ParentService) MonitorChildWithDailyData(firebaseUID string, usageData json.RawMessage) error {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return err
	}

	// Обрабатываем входные данные как кумулятивные за день
	// Проверяем, что данные имеют правильный формат
	var dataArray []map[string]interface{}
	if err := json.Unmarshal(usageData, &dataArray); err != nil {
		return fmt.Errorf("invalid usage data format: %v", err)
	}

	// Добавляем метку времени для отслеживания последнего обновления
	now := time.Now()
	for i := range dataArray {
		dataArray[i]["last_updated"] = now.Format(time.RFC3339)
	}

	// Преобразуем обратно в JSON
	updatedData, err := json.Marshal(dataArray)
	if err != nil {
		return err
	}

	// Обновляем данные ребенка
	child.UsageData = string(updatedData)

	// Сохраняем в базу данных
	return s.ChildRepo.Save(child)
}

// ManageAppTimeRules обрабатывает как блокировку, так и разблокировку приложений по времени
func (s *ParentService) ManageAppTimeRules(parentUID, childUID string, apps []string, action string, startTime, endTime string, blockIDs ...int64) error {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentUID)
	if err != nil {
		return errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childUID)
	if err != nil {
		return errors.New("child not found")
	}

	// Проверяем, принадлежит ли ребенок родителю
	if !s.isChildInFamily(parent, childUID) {
		return errors.New("child does not belong to this parent")
	}

	if action == "block" {
		// Получаем существующие блокировки
		existingBlocks, err := s.ChildRepo.GetTimeBlockedApps(child.ID)
		if err != nil {
			return err
		}

		// Создаем записи о временной блокировке
		var newBlocks []models.AppTimeBlock

		// Используем переданный ID или генерируем новый
		blockID := int64(0)
		if len(blockIDs) > 0 {
			blockID = blockIDs[0]
		} else {
			blockID = time.Now().UnixNano() // Генерируем ID на основе текущего времени
		}

		for _, app := range apps {
			block := models.AppTimeBlock{
				ID:         blockID, // Устанавливаем ID
				AppPackage: app,
				StartTime:  startTime,
				EndTime:    endTime,
				DaysOfWeek: "1,2,3,4,5,6,7", // По умолчанию все дни недели
				IsOneTime:  false,
			}
			newBlocks = append(newBlocks, block)

			// Увеличиваем ID для следующего блока, если нужно создать несколько
			blockID++
		}

		// Объединяем существующие и новые блоки
		updatedBlocks := append(existingBlocks, newBlocks...)

		// Сохраняем обновленный список блоков
		return s.ChildRepo.AddTimeBlockedApps(child.ID, updatedBlocks)
	} else if action == "unblock" && len(blockIDs) > 0 {
		// Получение существующих блоков
		existingBlocks, err := s.ChildRepo.GetTimeBlockedApps(child.ID)
		if err != nil {
			return err
		}

		// Находим блоки, соответствующие указанным ID
		var blocksToRemove []models.AppTimeBlock
		for _, id := range blockIDs {
			for _, block := range existingBlocks {
				if block.ID == id {
					blocksToRemove = append(blocksToRemove, block)
					break
				}
			}
		}

		// Теперь ищем все блоки, которые принадлежат к тем же группам
		var groupKeysToRemove []string
		for _, blockToRemove := range blocksToRemove {
			// Создаем ключ группы
			groupKey := fmt.Sprintf("%s_%s_%s_%s",
				blockToRemove.StartTime,
				blockToRemove.EndTime,
				blockToRemove.BlockName,
				blockToRemove.DaysOfWeek)
			groupKeysToRemove = append(groupKeysToRemove, groupKey)
		}

		// Фильтрация блоков - оставляем только те, которых нет в списке удаления
		var updatedBlocks []models.AppTimeBlock
		for _, block := range existingBlocks {
			shouldKeep := true

			// Проверяем, принадлежит ли блок к группе, которую нужно удалить
			groupKey := fmt.Sprintf("%s_%s_%s_%s",
				block.StartTime,
				block.EndTime,
				block.BlockName,
				block.DaysOfWeek)

			for _, keyToRemove := range groupKeysToRemove {
				if keyToRemove == groupKey {
					shouldKeep = false
					break
				}
			}

			if shouldKeep {
				updatedBlocks = append(updatedBlocks, block)
			}
		}

		// Удаление всех блоков и добавление обновленных
		if err := s.ChildRepo.RemoveAllTimeBlockedApps(child.ID); err != nil {
			return err
		}

		return s.ChildRepo.AddTimeBlockedApps(child.ID, updatedBlocks)
	}

	return nil
}

// BlockAppsWithMultipleTimeRanges блокирует приложения с несколькими временными интервалами
func (s *ParentService) BlockAppsWithMultipleTimeRanges(
	parentUID string,
	childUID string,
	apps []string,
	timeBlocks []struct {
		StartTime string `json:"start_time"`
		EndTime   string `json:"end_time"`
		BlockName string `json:"block_name,omitempty"`
	},
) error {
	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentUID)
	if err != nil {
		return errors.New("parent not found")
	}

	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childUID)
	if err != nil {
		return errors.New("child not found")
	}

	// Проверяем, принадлежит ли ребенок родителю
	if !s.isChildInFamily(parent, childUID) {
		return errors.New("child does not belong to this parent")
	}

	// Создаем записи о временной блокировке для каждого приложения и каждого временного интервала
	var blocks []models.AppTimeBlock
	for _, app := range apps {
		for _, timeBlock := range timeBlocks {
			block := models.AppTimeBlock{
				AppPackage: app,
				StartTime:  timeBlock.StartTime,
				EndTime:    timeBlock.EndTime,
				DaysOfWeek: "1,2,3,4,5,6,7", // По умолчанию все дни недели
				IsOneTime:  false,
				// BlockName:  timeBlock.BlockName,
			}
			blocks = append(blocks, block)
		}
	}

	// Сохраняем блокировки
	return s.ChildRepo.AddTimeBlockedApps(child.ID, blocks)
}

// GetOneTimeBlocksFromDB получает одноразовые блокировки из базы данных
func (s *ParentService) GetOneTimeBlocksFromDB(childID uint) ([]models.AppTimeBlock, error) {
	// Получаем все временные блокировки
	allBlocks, err := s.ChildRepo.GetTimeBlockedApps(childID)
	if err != nil {
		return nil, err
	}

	// Фильтруем только одноразовые блокировки
	var oneTimeBlocks []models.AppTimeBlock
	for _, block := range allBlocks {
		if block.IsOneTime {
			oneTimeBlocks = append(oneTimeBlocks, block)
		}
	}

	return oneTimeBlocks, nil
}

// SaveOneTimeBlocksToDB сохраняет одноразовые блокировки в базу данных
func (s *ParentService) SaveOneTimeBlocksToDB(childID uint, blocks []models.AppTimeBlock) error {
	// Получаем все временные блокировки
	allBlocks, err := s.ChildRepo.GetTimeBlockedApps(childID)
	if err != nil {
		return err
	}

	// Фильтруем, оставляя только не-одноразовые блокировки
	var regularBlocks []models.AppTimeBlock
	for _, block := range allBlocks {
		if !block.IsOneTime {
			regularBlocks = append(regularBlocks, block)
		}
	}

	// Объединяем регулярные блокировки и новые одноразовые блокировки
	updatedBlocks := append(regularBlocks, blocks...)

	// Удаляем все текущие блокировки и добавляем обновленные
	if err := s.ChildRepo.RemoveAllTimeBlockedApps(childID); err != nil {
		return err
	}

	if len(updatedBlocks) > 0 {
		return s.ChildRepo.AddTimeBlockedApps(childID, updatedBlocks)
	}

	return nil
}
