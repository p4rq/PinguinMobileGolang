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
	endTime := time.Now().Add(time.Duration(request.DurationHours * float64(time.Hour)))

	// Создаем блоки для одноразовой блокировки
	var blocks []models.AppTimeBlock
	for _, appPackage := range request.AppPackages {
		block := models.AppTimeBlock{
			AppPackage:   appPackage,
			StartTime:    "00:00",                               // Начало дня
			EndTime:      "23:59",                               // Конец дня
			DaysOfWeek:   "1,2,3,4,5,6,7",                       // Все дни недели
			IsOneTime:    true,                                  // Флаг одноразовой блокировки
			OneTimeEndAt: endTime,                               // Время окончания блокировки
			Duration:     formatDuration(request.DurationHours), // Длительность в читаемом формате
		}
		blocks = append(blocks, block)
	}

	// Сохраняем блокировки через репозиторий ребенка
	if err := s.ChildRepo.AddTimeBlockedApps(child.ID, blocks); err != nil {
		return nil, err
	}

	// Возвращаем созданные блокировки
	return blocks, nil
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
