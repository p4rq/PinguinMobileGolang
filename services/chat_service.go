package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type ChatService struct {
	ChatRepo   repositories.ChatRepository
	ParentRepo repositories.ParentRepository
	ChildRepo  repositories.ChildRepository
}

func NewChatService(chatRepo repositories.ChatRepository, parentRepo repositories.ParentRepository, childRepo repositories.ChildRepository) *ChatService {
	return &ChatService{
		ChatRepo:   chatRepo,
		ParentRepo: parentRepo,
		ChildRepo:  childRepo,
	}
}

// IsChildInFamily проверяет, принадлежит ли ребенок к семье родителя
func (s *ChatService) IsChildInFamily(childFirebaseUID, parentFirebaseUID string) (bool, error) {
	parent, err := s.ParentRepo.FindByFirebaseUID(parentFirebaseUID)
	if err != nil {
		return false, err
	}

	var family []map[string]interface{}
	if err := json.Unmarshal([]byte(parent.Family), &family); err != nil {
		return false, err
	}

	for _, member := range family {
		if firebaseUID, ok := member["firebase_uid"].(string); ok && firebaseUID == childFirebaseUID {
			return true, nil
		}
	}

	return false, nil
}

// SendMessage отправляет новое текстовое сообщение
func (s *ChatService) SendMessage(senderID, parentID, message, channel string, isPrivate bool, recipientID string, isParent bool) (*models.ChatMessage, error) {
	// Проверяем, что отправитель существует
	var senderName string
	var senderType string

	if isParent {
		parent, err := s.ParentRepo.FindByFirebaseUID(senderID)
		if err != nil {
			return nil, errors.New("parent not found")
		}
		senderName = parent.Name
		senderType = "parent"

		// Если родитель отправляет сообщение, проверяем, что это его семья
		if senderID != parentID {
			return nil, errors.New("unauthorized: parent can only send to their own family")
		}
	} else {
		child, err := s.ChildRepo.FindByFirebaseUID(senderID)
		if err != nil {
			return nil, errors.New("child not found")
		}
		senderName = child.Name
		senderType = "child"

		// Проверяем, что ребенок принадлежит к этой семье
		inFamily, err := s.IsChildInFamily(senderID, parentID)
		if err != nil {
			return nil, err
		}
		if !inFamily {
			return nil, errors.New("unauthorized: child not in this family")
		}
	}

	if message == "" {
		return nil, errors.New("message cannot be empty")
	}

	// Если указан получатель, проверяем, что он существует и принадлежит к семье
	if isPrivate && recipientID != "" {
		var recipientExists bool
		var err error

		// Проверяем, является ли получатель родителем
		if recipientID == parentID {
			_, err = s.ParentRepo.FindByFirebaseUID(recipientID)
			recipientExists = (err == nil)
		} else {
			// Проверяем, является ли получатель ребенком в этой семье
			recipientExists, err = s.IsChildInFamily(recipientID, parentID)
			if err != nil {
				return nil, err
			}
		}

		if !recipientExists {
			return nil, errors.New("recipient not found in family")
		}
	}

	// Определяем канал по умолчанию, если не указан
	if !isPrivate && channel == "" {
		channel = models.ChannelGeneral
	}

	chatMessage := &models.ChatMessage{
		ParentID:    parentID,
		SenderID:    senderID,
		SenderType:  senderType,
		SenderName:  senderName,
		RecipientID: recipientID,
		IsPrivate:   isPrivate,
		Channel:     channel,
		Message:     message,
		MessageType: models.MessageTypeText,
		IsModerated: false,
		IsRead:      false,
		IsHidden:    false,
		CreatedAt:   time.Now(),
	}

	err := s.ChatRepo.SaveMessage(chatMessage)
	if err != nil {
		return nil, err
	}

	return chatMessage, nil
}

// SendMediaMessage отправляет сообщение с медиа-файлом
func (s *ChatService) SendMediaMessage(
	senderID, parentID string,
	message, messageType, channel string,
	isPrivate bool, recipientID string,
	isParent bool,
	file *multipart.FileHeader) (*models.ChatMessage, error) {

	// Проверка типа медиа
	if messageType != models.MessageTypeImage &&
		messageType != models.MessageTypeFile &&
		messageType != models.MessageTypeVideo &&
		messageType != models.MessageTypeAudio {
		return nil, errors.New("invalid media type")
	}

	// Текстовое сообщение может быть пустым при отправке медиа

	// Создаем базовое сообщение
	textMsg, err := s.SendMessage(senderID, parentID, message, channel, isPrivate, recipientID, isParent)
	if err != nil {
		return nil, err
	}

	// Сохраняем файл
	filename := filepath.Base(file.Filename)
	fileExt := filepath.Ext(filename)

	// Создаем уникальное имя файла - FIX: Convert uint to string
	uniqueFilename := strconv.FormatUint(uint64(textMsg.ID), 10) + "_" + time.Now().Format("20060102150405") + fileExt

	// Проверяем/создаем директорию для хранения файлов
	uploadDir := "./uploads/chat_media"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return nil, err
	}

	// Путь для сохранения файла
	dst := filepath.Join(uploadDir, uniqueFilename)

	// Сохраняем файл на диск
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	if err != nil {
		return nil, err
	}

	// Формируем URL для доступа к файлу
	mediaURL := "/media/chat/" + uniqueFilename

	// Обновляем информацию о медиа в сообщении
	textMsg.MessageType = messageType
	textMsg.MediaURL = mediaURL
	textMsg.MediaName = filename
	textMsg.MediaSize = file.Size

	// Сохраняем обновленное сообщение
	if err := s.ChatRepo.SaveMessage(textMsg); err != nil {
		// Удаляем файл в случае ошибки
		os.Remove(dst)
		return nil, err
	}

	return textMsg, nil
}

// GetFamilyMessages получает список сообщений для семьи с фильтром по каналу
func (s *ChatService) GetFamilyMessages(userID, parentID, channel string, limit, offset int) ([]models.ChatMessage, error) {
	log.Printf("ChatService.GetFamilyMessages: userID=%s, parentID=%s", userID, parentID)

	// Если пользователь - родитель своей семьи, то разрешаем доступ без проверки
	if userID == parentID {
		log.Printf("User is parent, skipping authorization check")
		// Просто получаем сообщения
		return s.ChatRepo.GetFamilyMessages(parentID, channel, limit, offset)
	}

	// Для детей проверяем принадлежность к семье
	inFamily, err := s.IsChildInFamily(userID, parentID)
	if err != nil {
		log.Printf("Error checking if child in family: %v", err)
		return nil, err
	}

	if !inFamily {
		log.Printf("User %s is not in family %s", userID, parentID)
		return nil, errors.New("unauthorized: user not in this family")
	}

	// После проверки получаем сообщения
	return s.ChatRepo.GetFamilyMessages(parentID, channel, limit, offset)
}

// GetPrivateMessages получает список личных сообщений между двумя пользователями
func (s *ChatService) GetPrivateMessages(userID, parentID, otherUserID string, limit, offset int) ([]models.ChatMessage, error) {
	// Проверка прав доступа
	if userID == parentID {
		// Если запрашивает родитель-владелец семьи - все нормально
	} else {
		// Если запрашивает ребенок, проверяем принадлежность к семье
		inFamily, err := s.IsChildInFamily(userID, parentID)
		if err != nil {
			return nil, err
		}
		if !inFamily {
			return nil, errors.New("unauthorized: user not in this family")
		}
	}

	// Проверяем, что другой пользователь тоже принадлежит к этой семье
	isOtherInFamily := false
	if otherUserID == parentID {
		isOtherInFamily = true
	} else {
		var err error
		isOtherInFamily, err = s.IsChildInFamily(otherUserID, parentID)
		if err != nil {
			return nil, err
		}
	}

	if !isOtherInFamily {
		return nil, errors.New("unauthorized: other user not in this family")
	}

	// Получаем личные сообщения между пользователями
	messages, err := s.ChatRepo.GetPrivateMessages(parentID, userID, otherUserID, limit, offset)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

// MarkMessagesAsRead отмечает сообщения как прочитанные
func (s *ChatService) MarkMessagesAsRead(messageIDs []uint, userID string) error {
	// Здесь также может быть проверка прав доступа к сообщениям
	return s.ChatRepo.MarkAsRead(messageIDs)
}

// DeleteMessage удаляет сообщение (только если пользователь отправитель или родитель)
func (s *ChatService) DeleteMessage(messageID uint, userID string, isParent bool) error {
	// Здесь добавить проверку, что удаляющий - автор сообщения или родитель
	return s.ChatRepo.DeleteMessage(messageID)
}

// ModerateMessage модерирует сообщение (только для родителя)
func (s *ChatService) ModerateMessage(messageID uint, parentID string, isHidden bool) error {
	// Проверка, что запрос от родителя - FIX: Use underscore to ignore unused variable
	_, err := s.ParentRepo.FindByFirebaseUID(parentID)
	if err != nil {
		return errors.New("unauthorized: only parent can moderate messages")
	}

	// Получаем сообщение для проверки, что оно принадлежит к семье этого родителя
	// (в реальной реализации нужно добавить метод в репозиторий)

	return s.ChatRepo.ModerateMessage(messageID, isHidden)
}

// GetUnreadCount получает количество непрочитанных сообщений
func (s *ChatService) GetUnreadCount(parentID, userID, channel string) (int64, error) {
	// Проверяем доступ пользователя к этой семье
	if userID != parentID {
		inFamily, err := s.IsChildInFamily(userID, parentID)
		if err != nil {
			return 0, err
		}
		if !inFamily {
			return 0, errors.New("unauthorized: user not in this family")
		}
	}

	return s.ChatRepo.GetUnreadMessagesCount(parentID, userID, channel)
}

// GetUnreadPrivateCount получает количество непрочитанных личных сообщений
func (s *ChatService) GetUnreadPrivateCount(parentID, userID string) (int64, error) {
	// Проверяем доступ пользователя к этой семье
	if userID != parentID {
		inFamily, err := s.IsChildInFamily(userID, parentID)
		if err != nil {
			return 0, err
		}
		if !inFamily {
			return 0, errors.New("unauthorized: user not in this family")
		}
	}

	return s.ChatRepo.GetUnreadPrivateCount(parentID, userID)
}

// GetChannelsList получает список каналов с сообщениями в семье
func (s *ChatService) GetChannelsList(parentID, userID string) ([]string, error) {
	// Проверяем доступ пользователя к этой семье
	if userID != parentID {
		inFamily, err := s.IsChildInFamily(userID, parentID)
		if err != nil {
			return nil, err
		}
		if !inFamily {
			return nil, errors.New("unauthorized: user not in this family")
		}
	}

	return s.ChatRepo.GetChannelsList(parentID)
}

// GetMessages получает сообщения для семейного чата
func (s *ChatService) GetMessages(parentID string, userID string, limit int) ([]*models.ChatMessage, error) {
	// Проверяем доступ пользователя к этой семье
	if userID != parentID {
		inFamily, err := s.IsChildInFamily(userID, parentID)
		if err != nil {
			return nil, err
		}
		if !inFamily {
			return nil, errors.New("unauthorized: user not in this family")
		}
	}

	// Проверка существования семьи через ParentRepo
	parent, err := s.ParentRepo.FindByFirebaseUID(parentID)
	if err != nil {
		log.Printf("Parent not found: %v", err)
		return nil, fmt.Errorf("parent not found: %w", err)
	}

	// Используем ChatRepo для получения сообщений
	chatMessages, err := s.ChatRepo.GetFamilyMessages(parent.FirebaseUID, "", limit, 0)
	if err != nil {
		log.Printf("Error getting family messages: %v", err)
		return nil, err
	}

	// Преобразуем в нужный формат возврата
	messages := make([]*models.ChatMessage, len(chatMessages))
	for i := range chatMessages {
		messages[i] = &chatMessages[i]
	}

	return messages, nil
}
