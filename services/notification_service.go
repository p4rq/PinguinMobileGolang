package services

import (
	"PinguinMobile/repositories"
	"context"
	"encoding/json"
	"fmt"
	"log"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

// NotificationService сервис для работы с push-уведомлениями
type NotificationService struct {
	FCMClient      *messaging.Client
	TranslationSrv *TranslationService
	ParentRepo     repositories.ParentRepository
	ChildRepo      repositories.ChildRepository
}

// NewNotificationService создает новый сервис уведомлений
func NewNotificationService(
	app *firebase.App,
	translationSrv *TranslationService,
	parentRepo repositories.ParentRepository,
	childRepo repositories.ChildRepository,
) (*NotificationService, error) {
	// Инициализация клиента Firebase Cloud Messaging
	ctx := context.Background()
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("error initializing FCM client: %w", err)
	}

	return &NotificationService{
		FCMClient:      client,
		TranslationSrv: translationSrv,
		ParentRepo:     parentRepo,
		ChildRepo:      childRepo,
	}, nil
}

// SendNotification отправляет push-уведомление на устройство с учетом языка
func (s *NotificationService) SendNotification(deviceToken, title, body string, data map[string]string, lang string) error {
	if deviceToken == "" {
		return fmt.Errorf("device token is empty")
	}

	// Логирование для отладки
	log.Printf("[FCM] Отправка уведомления. Title: %s, Body: %s, Token: %s", title, body, deviceToken)

	// Переводим уведомление на язык пользователя, если указан
	if lang != "" && s.TranslationSrv != nil {
		// Получаем все переводы для нужного языка
		translations := s.TranslationSrv.GetAllTranslations(lang)

		// Пытаемся найти перевод для заголовка
		if translatedTitle, exists := translations[title]; exists {
			title = translatedTitle
		}

		// Пытаемся найти перевод для тела уведомления
		if translatedBody, exists := translations[body]; exists {
			body = translatedBody
		}
	}

	// Создаем сообщение
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  data,
		Token: deviceToken,
	}

	// Отправляем уведомление
	ctx := context.Background()
	resp, err := s.FCMClient.Send(ctx, message)
	if err != nil {
		log.Printf("[FCM] Ошибка отправки уведомления: %v", err)
		return err
	}

	log.Printf("[FCM] Уведомление успешно отправлено. ID: %s, Title: %s", resp, title)
	return nil
}

// SendNotificationToParent отправляет уведомление родителю
func (s *NotificationService) SendNotificationToParent(parentUID, title, body string, data map[string]string) error {
	parent, err := s.ParentRepo.FindByFirebaseUID(parentUID)
	if err != nil {
		return fmt.Errorf("parent not found: %w", err)
	}

	if parent.DeviceToken == "" {
		return nil // Пропускаем отправку, если нет токена устройства
	}

	// Отправляем уведомление на языке родителя
	return s.SendNotification(parent.DeviceToken, title, body, data, parent.Lang)
}

// SendNotificationToChild отправляет уведомление ребенку
func (s *NotificationService) SendNotificationToChild(childUID, title, body string, data map[string]string) error {
	child, err := s.ChildRepo.FindByFirebaseUID(childUID)
	if err != nil {
		return fmt.Errorf("child not found: %w", err)
	}

	if child.DeviceToken == "" {
		return nil // Пропускаем отправку, если нет токена устройства
	}

	// Отправляем уведомление на языке ребенка
	return s.SendNotification(child.DeviceToken, title, body, data, child.Lang)
}

// SendNotificationToFamily отправляет уведомление всем членам семьи
func (s *NotificationService) SendNotificationToFamily(
	parentUID,
	title,
	body string,
	data map[string]string,
	skipUIDs ...string,
) error {
	// Карта пользователей для пропуска
	skipMap := make(map[string]bool)
	for _, uid := range skipUIDs {
		skipMap[uid] = true
	}

	// Получаем родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(parentUID)
	if err != nil {
		return fmt.Errorf("parent not found: %w", err)
	}

	// Отправляем уведомление родителю, если он не в списке пропуска
	if !skipMap[parent.FirebaseUID] && parent.DeviceToken != "" {
		if err := s.SendNotificationToParent(parent.FirebaseUID, title, body, data); err != nil {
			log.Printf("Error sending notification to parent %s: %v", parent.FirebaseUID, err)
		}
	}

	// Получаем список детей в семье
	var family []map[string]interface{}
	if parent.Family != "" {
		if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
			log.Printf("Error parsing family JSON: %v", err)
		} else {
			// Отправляем уведомления каждому ребенку в семье
			for _, member := range family {
				if childUID, ok := member["firebase_uid"].(string); ok {
					if !skipMap[childUID] {
						if err := s.SendNotificationToChild(childUID, title, body, data); err != nil {
							log.Printf("Error sending notification to child %s: %v", childUID, err)
						}
					}
				}
			}
		}
	}

	return nil
}
