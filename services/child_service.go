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

func (s *ChildService) RebindChild(childCode, parentFirebaseUID string) (models.Child, error) {
	child, err := s.ChildRepo.FindByCode(childCode)
	if err != nil {
		return models.Child{}, err
	}

	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return models.Child{}, err
	}

	child.IsBinded = true
	familyData := map[string]interface{}{
		"parent_id":           parent.ID,
		"parent_name":         parent.Name,
		"parent_email":        parent.Email,
		"parent_firebase_uid": parent.FirebaseUID,
	}
	familyJSON, err := json.Marshal(familyData)
	if err != nil {
		return models.Child{}, err
	}
	child.Family = string(familyJSON)
	if err := s.ChildRepo.Save(child); err != nil {
		return models.Child{}, err
	}

	var family []map[string]interface{}
	if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
		return models.Child{}, err
	}

	// Check if the child entry exists in the family slice
	childExists := false
	for i, member := range family {
		if uint(member["child_id"].(float64)) == child.ID {
			family[i] = map[string]interface{}{
				"child_id":     child.ID,
				"name":         child.Name,
				"lang":         child.Lang,
				"gender":       child.Gender,
				"age":          child.Age,
				"birthday":     child.Birthday,
				"firebase_uid": child.FirebaseUID,
				"isBinded":     true,
				"code":         child.Code,
			}
			childExists = true
			break
		}
	}

	// If the child entry does not exist, add a new entry
	if !childExists {
		family = append(family, map[string]interface{}{
			"child_id":     child.ID,
			"name":         child.Name,
			"lang":         child.Lang,
			"gender":       child.Gender,
			"age":          child.Age,
			"birthday":     child.Birthday,
			"firebase_uid": child.FirebaseUID,
			"isBinded":     true,
			"code":         child.Code,
		})
	}

	familyJSON, _ = json.Marshal(family)
	parent.Family = string(familyJSON)
	if err := s.ParentRepo.Save(parent); err != nil {
		return models.Child{}, err
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
