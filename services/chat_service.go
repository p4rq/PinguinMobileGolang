package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
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

// Проверяет, принадлежит ли ребенок к семье родителя
func (s *ChatService) isChildInFamily(childFirebaseUID, parentFirebaseUID string) (bool, error) {
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

// Добавьте этот метод в ChatService

// IsChildInFamily проверяет, принадлежит ли ребенок к семье родителя
func (s *ChatService) IsChildInFamily(childFirebaseUID, parentFirebaseUID string) (bool, error) {
	return s.isChildInFamily(childFirebaseUID, parentFirebaseUID)
}

// SendMessage отправляет новое сообщение
func (s *ChatService) SendMessage(senderID, parentID, message, messageType string, isParent bool) (*models.ChatMessage, error) {
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
		inFamily, err := s.isChildInFamily(senderID, parentID)
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

	chatMessage := &models.ChatMessage{
		ParentID:    parentID,
		SenderID:    senderID,
		SenderType:  senderType,
		SenderName:  senderName,
		Message:     message,
		MessageType: messageType,
		IsRead:      false,
		CreatedAt:   time.Now(),
	}

	err := s.ChatRepo.SaveMessage(chatMessage)
	if err != nil {
		return nil, err
	}

	return chatMessage, nil
}

// GetFamilyMessages получает список сообщений для семьи
func (s *ChatService) GetFamilyMessages(userID, parentID string, limit, offset int) ([]models.ChatMessage, error) {
	// Проверка, что запрашивающий пользователь имеет доступ к этой семье
	if userID == parentID {
		// Если запрашивает родитель-владелец семьи - все нормально
	} else {
		// Если запрашивает ребенок, проверяем принадлежность к семье
		inFamily, err := s.isChildInFamily(userID, parentID)
		if err != nil {
			return nil, err
		}
		if !inFamily {
			return nil, errors.New("unauthorized: user not in this family")
		}
	}

	messages, err := s.ChatRepo.GetFamilyMessages(parentID, limit, offset)
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

// DeleteMessage удаляет сообщение (только если пользователь отправитель)
func (s *ChatService) DeleteMessage(messageID uint, userID string) error {
	// Здесь должна быть проверка, что удаляющий - автор сообщения
	return s.ChatRepo.DeleteMessage(messageID)
}

// GetUnreadCount получает количество непрочитанных сообщений
func (s *ChatService) GetUnreadCount(parentID, userID string) (int64, error) {
	// Проверяем доступ пользователя к этой семье
	if userID != parentID {
		inFamily, err := s.isChildInFamily(userID, parentID)
		if err != nil {
			return 0, err
		}
		if !inFamily {
			return 0, errors.New("unauthorized: user not in this family")
		}
	}

	return s.ChatRepo.GetUnreadMessagesCount(parentID, userID)
}
