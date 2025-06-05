package services

import (
	"PinguinMobile/repositories"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
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
	// Проверяем, что app не nil
	if app == nil {
		// Создаем новый экземпляр Firebase App если он не передан
		ctx := context.Background()

		// Явное указание projectID, как в debug_controller.go
		projectID := os.Getenv("FIREBASE_PROJECT_ID")
		if projectID == "" {
			projectID = "pinguin-46f73" // ID вашего проекта Firebase
		}

		// Создаем конфигурацию с явным projectID
		config := &firebase.Config{
			ProjectID: projectID,
		}

		// Настраиваем опции Firebase аналогично debug_controller.go
		var opts []option.ClientOption
		credentialsPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

		if credentialsPath != "" {
			log.Printf("[FCM] Using credentials from file: %s", credentialsPath)
			opts = append(opts, option.WithCredentialsFile(credentialsPath))
		} else {
			credentialsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
			if credentialsJSON != "" {
				log.Printf("[FCM] Using credentials from environment JSON")
				opts = append(opts, option.WithCredentialsJSON([]byte(credentialsJSON)))
			} else {
				defaultPath := "pinguin-46f73-firebase-adminsdk-fbsvc-8610ba7d3e.json"
				if _, err := os.Stat(defaultPath); err == nil {
					log.Printf("[FCM] Using credentials from default path: %s", defaultPath)
					opts = append(opts, option.WithCredentialsFile(defaultPath))
				} else {
					return nil, fmt.Errorf("firebase credentials not found")
				}
			}
		}

		// Инициализируем Firebase с явными параметрами
		var err error
		app, err = firebase.NewApp(ctx, config, opts...)
		if err != nil {
			log.Printf("[FCM ERROR] Failed to initialize Firebase app: %v", err)
			return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
		}
		log.Printf("[FCM] Firebase app initialized successfully with project ID: %s", projectID)
	}

	// Инициализация клиента FCM
	ctx := context.Background()
	client, err := app.Messaging(ctx)
	if err != nil {
		log.Printf("[FCM ERROR] Failed to initialize FCM client: %v", err)
		return nil, fmt.Errorf("error initializing FCM client: %w", err)
	}
	log.Printf("[FCM] FCM client initialized successfully")

	return &NotificationService{
		FCMClient:      client,
		TranslationSrv: translationSrv,
		ParentRepo:     parentRepo,
		ChildRepo:      childRepo,
	}, nil
}

// SendNotification отправляет push-уведомление на устройство с учетом языка
func (s *NotificationService) SendNotification(deviceToken, title, body string, data map[string]string, lang string) error {
	// Проверка токена
	if deviceToken == "" {
		log.Printf("[FCM] Ошибка: пустой токен устройства")
		return fmt.Errorf("device token is empty")
	}

	// Проверка длины токена (как в debug_controller)
	if len(deviceToken) < 20 {
		log.Printf("[FCM] Предупреждение: подозрительно короткий токен устройства: %s (длина: %d)",
			deviceToken, len(deviceToken))
	}

	// Безопасное логирование для токена (с обрезкой, как в debug_controller)
	tokenDisplay := deviceToken
	if len(deviceToken) > 10 {
		tokenDisplay = deviceToken[:10] + "..."
	}

	log.Printf("[FCM] Отправка уведомления. Title: %s, Body: %s, Token: %s, Lang: %s",
		title, body, tokenDisplay, lang)

	// Переводим уведомление на язык пользователя, если указан
	if lang != "" && s.TranslationSrv != nil {
		// Получаем все переводы для нужного языка
		translations := s.TranslationSrv.GetAllTranslations(lang)

		// Пытаемся найти перевод для заголовка
		if translatedTitle, exists := translations[title]; exists {
			log.Printf("[FCM] Заголовок переведен: %s -> %s", title, translatedTitle)
			title = translatedTitle
		}

		// Пытаемся найти перевод для тела уведомления
		if translatedBody, exists := translations[body]; exists {
			log.Printf("[FCM] Тело уведомления переведено: %s -> %s", body, translatedBody)
			body = translatedBody
		}
	}

	// Вывод данных в лог
	dataStr, _ := json.Marshal(data)
	log.Printf("[FCM] Дополнительные данные: %s", string(dataStr))

	// Создаем сообщение
	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:  data,
		Token: deviceToken,
	}

	// Отправляем уведомление с более подробной обработкой ошибок
	ctx := context.Background()
	resp, err := s.FCMClient.Send(ctx, message)
	if err != nil {
		log.Printf("[FCM CRITICAL] Ошибка отправки уведомления: %v", err)

		// Проверка на известные ошибки токена
		if strings.Contains(err.Error(), "registration-token-not-registered") {
			log.Printf("[FCM] Токен устройства недействителен или устарел: %s", tokenDisplay)
			return fmt.Errorf("device token is invalid or expired: %w", err)
		}

		if strings.Contains(err.Error(), "invalid-argument") ||
			strings.Contains(err.Error(), "not a valid FCM registration token") {
			log.Printf("[FCM] Недействительный формат токена устройства: %s", tokenDisplay)
			return fmt.Errorf("invalid device token format: %w", err)
		}

		return fmt.Errorf("FCM send error: %w", err)
	}

	log.Printf("[FCM SUCCESS] Уведомление успешно отправлено. ID: %s, Title: %s, Token: %s",
		resp, title, tokenDisplay)
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
