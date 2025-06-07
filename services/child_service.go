package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"firebase.google.com/go/auth"
)

type ChildService struct {
	ChildRepo    repositories.ChildRepository
	ParentRepo   repositories.ParentRepository
	FirebaseAuth *auth.Client
	NotifySrv    *NotificationService // Добавляем поле для сервиса уведомлений
}

func NewChildService(
	childRepo repositories.ChildRepository,
	parentRepo repositories.ParentRepository,
	firebaseAuth *auth.Client,
	notifySrv *NotificationService, // Добавляем параметр
) *ChildService {
	return &ChildService{
		ChildRepo:    childRepo,
		ParentRepo:   parentRepo,
		FirebaseAuth: firebaseAuth,
		NotifySrv:    notifySrv,
	}
}

func (s *ChildService) ReadChild(firebaseUID string) (models.Child, error) {
	return s.ChildRepo.FindByFirebaseUID(firebaseUID)
}

func (s *ChildService) UpdateChild(firebaseUID string, input models.Child) (models.Child, error) {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return models.Child{}, err
	}

	// Обновляем поля ребенка
	child.Lang = input.Lang
	child.Name = input.Name
	child.Gender = input.Gender
	child.Age = input.Age
	child.Birthday = input.Birthday

	if err := s.ChildRepo.Save(child); err != nil {
		return models.Child{}, err
	}

	// Обновляем информацию в JSON родителя
	var familyData map[string]interface{}
	if err := json.Unmarshal([]byte(child.Family), &familyData); err != nil {
		return models.Child{}, err
	}

	parentFirebaseUID, ok := familyData["parent_firebase_uid"].(string)
	if !ok {
		return models.Child{}, errors.New("parent_firebase_uid is missing or not a string")
	}

	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err == nil {
		var family []map[string]interface{}
		if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
			return models.Child{}, err
		}

		// Проверяем существование записи о ребенке в массиве family
		// и обновляем ПО FIREBASE_UID, а не по child_id
		childExists := false
		for i, member := range family {
			memberFirebaseUID, exists := member["firebase_uid"]
			if exists && memberFirebaseUID == firebaseUID {
				// Обновляем существующую запись
				family[i] = map[string]interface{}{
					"child_id":     child.ID,
					"name":         child.Name,
					"lang":         child.Lang,
					"gender":       child.Gender,
					"age":          child.Age,
					"birthday":     child.Birthday,
					"firebase_uid": child.FirebaseUID,
					"isBinded":     child.IsBinded,
					"usage_data":   child.UsageData,
					"code":         child.Code,
				}
				childExists = true
				break
			}
		}

		// Если запись о ребенке не найдена, добавляем новую
		if !childExists {
			family = append(family, map[string]interface{}{
				"child_id":     child.ID,
				"name":         child.Name,
				"lang":         child.Lang,
				"gender":       child.Gender,
				"age":          child.Age,
				"birthday":     child.Birthday,
				"firebase_uid": child.FirebaseUID,
				"isBinded":     child.IsBinded,
				"usage_data":   child.UsageData,
				"code":         child.Code,
			})
		}

		// Сохраняем обновленный массив family
		familyJSON, _ := json.Marshal(family)
		parent.Family = string(familyJSON)
		s.ParentRepo.Save(parent)
	}

	return child, nil
}

func (s *ChildService) DeleteChild(firebaseUID string) error {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return err
	}
	if err := s.ChildRepo.Delete(child); err != nil {
		return err
	}
	return nil
}

// Обновляем метод LogoutChild чтобы принимать причину выхода и отправлять уведомление
func (s *ChildService) LogoutChild(firebaseUID string, reason string) (models.Child, error) {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return models.Child{}, err
	}

	// Запоминаем старый токен и очищаем его при выходе
	child.DeviceToken = ""

	// Устанавливаем флаг isBinded в false
	child.IsBinded = false
	if err := s.ChildRepo.Save(child); err != nil {
		return models.Child{}, err
	}

	// Update parent's family JSON
	var familyData map[string]interface{}
	if err := json.Unmarshal([]byte(child.Family), &familyData); err != nil {
		return models.Child{}, err
	}

	parentFirebaseUID, ok := familyData["parent_firebase_uid"].(string)
	if !ok {
		return models.Child{}, errors.New("parent_firebase_uid is missing or not a string")
	}

	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err == nil {
		var family []map[string]interface{}
		if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
			return models.Child{}, err
		}

		for i, member := range family {
			// Используем более гибкий поиск по child_id
			childID, ok := member["child_id"].(float64)
			if ok && uint(childID) == child.ID {
				member["isBinded"] = false
				family[i] = member
				break
			}
		}

		familyJSON, _ := json.Marshal(family)
		parent.Family = string(familyJSON)
		s.ParentRepo.Save(parent)

		// Отправляем уведомление родителю, если доступен сервис уведомлений
		if s.NotifySrv != nil && parent.DeviceToken != "" {
			// Формируем заголовок и тело уведомления
			title := "Ребенок вышел из приложения"
			body := fmt.Sprintf("%s вышел из приложения Pinguin", child.Name)

			// Если указана причина выхода, добавляем её в сообщение
			if reason != "" {
				body += fmt.Sprintf(" (причина: %s)", reason)
			}

			// Дополнительные данные для уведомления
			data := map[string]string{
				"notification_type":  "child_logout",
				"child_name":         child.Name,
				"child_id":           fmt.Sprintf("%d", child.ID),
				"child_firebase_uid": child.FirebaseUID,
				"timestamp":          fmt.Sprintf("%d", time.Now().Unix()),
			}

			// Асинхронно отправляем уведомление родителю
			go func() {
				err := s.NotifySrv.SendNotification(
					parent.DeviceToken,
					title,
					body,
					data,
					parent.Lang, // Используем язык родителя для перевода
				)

				// Логирование результата отправки
				if err != nil {
					fmt.Printf("[PUSH ERROR] Failed to send child logout notification to parent %s: %v\n",
						parentFirebaseUID, err)
				} else {
					fmt.Printf("[PUSH SUCCESS] Sent child logout notification to parent %s\n",
						parentFirebaseUID)
				}
			}()
		}
	}

	return child, nil
}

func (s *ChildService) MonitorChild(childFirebaseUID string, sessions []models.Session) error {
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return errors.New("child not found")
	}

	var existingSessions []models.Session
	if child.UsageData != "" {
		if err := json.Unmarshal([]byte(child.UsageData), &existingSessions); err != nil {
			return errors.New("failed to unmarshal existing sessions JSON")
		}
	}

	// Объединение сессий
	for _, newSession := range sessions {
		merged := false
		for i, existingSession := range existingSessions {
			if existingSession.App == newSession.App && newSession.Timestamp.Sub(existingSession.Timestamp) < 24*time.Hour {
				existingSessions[i].Duration += newSession.Duration
				merged = true
				break
			}
		}
		if !merged {
			existingSessions = append(existingSessions, newSession)
		}
	}

	sessionsJson, err := json.Marshal(existingSessions)
	if err != nil {
		return errors.New("failed to marshal sessions JSON")
	}

	child.UsageData = string(sessionsJson)
	if err := s.ChildRepo.Save(child); err != nil {
		return err
	}

	return nil
}

// RebindChild связывает ребенка с родителем только по коду
func (s *ChildService) RebindChild(code string) (models.Child, error) {
	// Ищем ребенка по коду
	child, err := s.ChildRepo.FindByCode(code)
	if err != nil {
		return models.Child{}, fmt.Errorf("child with code %s not found: %w", code, err)
	}

	// Если ребенок не связан с родителем, это ошибка
	if child.Family == "" {
		return models.Child{}, fmt.Errorf("child with code %s has no family information", code)
	}

	// Распаковываем информацию о родителе из family JSON
	var familyData map[string]interface{}
	if err := json.Unmarshal([]byte(child.Family), &familyData); err != nil {
		return models.Child{}, fmt.Errorf("invalid family data: %w", err)
	}

	// Извлекаем parent_firebase_uid из family JSON
	parentFirebaseUID, ok := familyData["parent_firebase_uid"].(string)
	if !ok || parentFirebaseUID == "" {
		return models.Child{}, fmt.Errorf("parent_firebase_uid not found in family data")
	}

	// Ищем родителя по FirebaseUID
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return models.Child{}, fmt.Errorf("parent not found: %w", err)
	}

	// Устанавливаем флаг isBinded в true
	child.IsBinded = true
	if err := s.ChildRepo.Save(child); err != nil {
		return models.Child{}, fmt.Errorf("failed to save child: %w", err)
	}

	// Обновляем семью родителя, если нужно
	var familyArray []map[string]interface{}
	if parent.Family != "" {
		if err := json.Unmarshal([]byte(parent.Family), &familyArray); err != nil {
			return models.Child{}, fmt.Errorf("failed to parse parent's family: %w", err)
		}
	}

	// Проверяем, есть ли этот ребенок уже в семье родителя
	childFound := false
	for i, member := range familyArray {
		childID, ok := member["child_id"].(float64)
		if ok && uint(childID) == child.ID {
			// Обновляем статус связи
			member["isBinded"] = true
			familyArray[i] = member
			childFound = true
			break
		}
	}

	// Если ребенка нет в семье родителя, добавляем его
	if !childFound {
		// Создаем информацию о ребенке для родителя
		childInfo := map[string]interface{}{
			"child_id":     float64(child.ID),
			"name":         child.Name,
			"firebase_uid": child.FirebaseUID,
			"isBinded":     true,
			"gender":       child.Gender,
			"age":          child.Age,
			"birthday":     child.Birthday,
		}
		familyArray = append(familyArray, childInfo)
	}

	// Сохраняем обновленную семью родителя
	familyJSON, _ := json.Marshal(familyArray)
	parent.Family = string(familyJSON)
	if err := s.ParentRepo.Save(parent); err != nil {
		return models.Child{}, fmt.Errorf("failed to update parent: %w", err)
	}

	// Отправляем уведомление родителю, если у нас есть сервис уведомлений
	if s.NotifySrv != nil && parent.DeviceToken != "" {
		title := "Повторное подключение устройства"
		body := fmt.Sprintf("Устройство %s снова подключено к вашей семье", child.Name)

		data := map[string]string{
			"notification_type":  "child_rebind",
			"child_id":           fmt.Sprintf("%d", child.ID),
			"child_firebase_uid": child.FirebaseUID,
		}

		go func() {
			err := s.NotifySrv.SendNotification(parent.DeviceToken, title, body, data, parent.Lang)
			if err != nil {
				fmt.Printf("[PUSH ERROR] Ошибка отправки уведомления о повторном подключении: %v\n", err)
			} else {
				fmt.Printf("[PUSH SUCCESS] Отправлено уведомление о повторном подключении ребенка %s\n", child.Name)
			}
		}()
	}

	return child, nil
}

// CheckAppBlocking проверяет, заблокировано ли приложение (постоянно или временно)
func (s *ChildService) CheckAppBlocking(childFirebaseUID string, appPackage string) (bool, string, error) {
	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return false, "", err
	}

	// Проверяем постоянную блокировку
	if child.BlockedApps != "" {
		blockedApps := strings.Split(child.BlockedApps, ",")
		for _, app := range blockedApps {
			if app == appPackage {
				return true, "permanently blocked", nil
			}
		}
	}

	// Проверяем временную блокировку
	if child.TimeBlockedApps == "" {
		return false, "", nil
	}

	var timeBlocks []models.AppTimeBlock
	if err := json.Unmarshal([]byte(child.TimeBlockedApps), &timeBlocks); err != nil {
		return false, "", err
	}

	// Получаем текущее время и день недели
	now := time.Now()
	currentTime := now.Format("15:04")
	currentDayOfWeek := int(now.Weekday())
	if currentDayOfWeek == 0 { // В Go воскресенье = 0, мы используем 7
		currentDayOfWeek = 7
	}

	// Проверяем каждую временную блокировку для данного приложения
	for _, block := range timeBlocks {
		if block.AppPackage == appPackage {
			zeroTime := time.Time{}
			// Проверяем одноразовые блокировки
			if block.IsOneTime {
				// ИЗМЕНЕНИЕ: убираем проверку времени окончания
				// Было: if block.IsOneTime && block.OneTimeEndAt != zeroTime && block.OneTimeEndAt.After(now) {
				if block.OneTimeEndAt != zeroTime {
					// Если блокировка постоянная (IsPermanent) или время еще не истекло
					if block.IsPermanent || block.OneTimeEndAt.After(now) {
						return true, "one_time", nil
					}
					// ИЗМЕНЕНИЕ: Даже если время истекло, все равно считаем блокировку активной
					return true, "one_time", nil
				}
			}

			// Проверяем регулярные блокировки по времени
			if !block.IsOneTime && block.DaysOfWeek != "" {
				// Проверяем, применяется ли блокировка в текущий день недели
				if strings.Contains(block.DaysOfWeek, strconv.Itoa(currentDayOfWeek)) {
					// Проверяем, находится ли текущее время в интервале блокировки
					if isTimeInRange(currentTime, block.StartTime, block.EndTime) {
						// Добавляем информацию о временном блоке для более информативного ответа
						blockInfo := fmt.Sprintf("time blocked from %s to %s", block.StartTime, block.EndTime)
						return true, blockInfo, nil
					}
				}
			}
		}
	}

	// Ни одна блокировка не активна
	return false, "", nil
}

// isTimeInRange проверяет, входит ли время в указанный интервал
// Эта функция уже хорошо реализована, оставляем как есть
func isTimeInRange(current, start, end string) bool {
	// Парсим время
	layout := "15:04"
	currentTime, _ := time.Parse(layout, current)
	startTime, _ := time.Parse(layout, start)
	endTime, _ := time.Parse(layout, end)

	// Особый случай: если конечное время меньше начального (блокировка через полночь)
	if endTime.Before(startTime) {
		// Если текущее время >= начальное ИЛИ <= конечное, то оно в интервале
		return !currentTime.Before(startTime) || !currentTime.After(endTime)
	}

	// Обычный случай: проверка, что текущее время между началом и концом
	return !currentTime.Before(startTime) && !currentTime.After(endTime)
}

// IsAppBlocked проверяет, заблокировано ли приложение (включая одноразовые блокировки)
func (s *ChildService) IsAppBlocked(childFirebaseUID, appPackage string) (bool, string, error) {
	// Получаем ребенка
	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return false, "", err
	}

	// Получаем все временные блокировки
	blocks, err := s.ChildRepo.GetTimeBlockedApps(child.ID)
	if err != nil {
		return false, "", err
	}

	// Текущее время
	now := time.Now()
	currentDay := int(now.Weekday())
	if currentDay == 0 {
		currentDay = 7 // Воскресенье = 7 вместо 0
	}
	currentTime := now.Format("15:04")

	// Сначала проверяем одноразовые блокировки
	for _, block := range blocks {
		if block.AppPackage == appPackage && block.IsOneTime {
			// ИЗМЕНЕНИЕ: Убираем проверку времени окончания
			// Было: if block.AppPackage == appPackage && block.IsOneTime && block.OneTimeEndAt.After(now) {
			// Найдена одноразовая блокировка, считаем ее всегда активной
			return true, "one_time", nil
		}
	}

	// Затем проверяем регулярные блокировки по расписанию
	for _, block := range blocks {
		if block.AppPackage == appPackage && !block.IsOneTime {
			// Проверяем, действует ли блокировка в текущий день
			daysOfWeek := strings.Split(block.DaysOfWeek, ",")
			for _, day := range daysOfWeek {
				dayNum, _ := strconv.Atoi(day)
				if dayNum == currentDay {
					// Проверяем, действует ли блокировка в текущее время
					if (block.StartTime <= currentTime && block.EndTime >= currentTime) ||
						(block.StartTime > block.EndTime && (currentTime >= block.StartTime || currentTime <= block.EndTime)) {
						return true, "scheduled", nil
					}
				}
			}
		}
	}

	// Приложение не заблокировано
	return false, "", nil
}
func (s *ChildService) UpdateDeviceToken(firebaseUID, deviceToken string) error {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return err
	}

	// Обновляем токен устройства
	child.DeviceToken = deviceToken
	return s.ChildRepo.Save(child)
}

func (s *ChildService) UpdateChildPermissions(
	firebaseUID string,
	screenTimePermission bool,
	appearOnTop bool,
	alarmsPermission bool,
) error {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return fmt.Errorf("failed to find child: %w", err)
	}

	// Обновляем разрешения
	child.ScreenTimePermission = screenTimePermission
	child.AppearOnTop = appearOnTop
	child.AlarmsPermission = alarmsPermission

	// Сохраняем изменения
	if err := s.ChildRepo.Save(child); err != nil {
		return fmt.Errorf("failed to save child permissions: %w", err)
	}

	return nil
}
