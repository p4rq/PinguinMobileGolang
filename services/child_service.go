package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
	"time"

	"firebase.google.com/go/auth"
)

type ChildService struct {
	ChildRepo    repositories.ChildRepository
	ParentRepo   repositories.ParentRepository
	FirebaseAuth *auth.Client
}

func NewChildService(childRepo repositories.ChildRepository, parentRepo repositories.ParentRepository, firebaseAuth *auth.Client) *ChildService {
	return &ChildService{ChildRepo: childRepo, ParentRepo: parentRepo, FirebaseAuth: firebaseAuth}
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

func (s *ChildService) LogoutChild(firebaseUID string) (models.Child, error) {
	child, err := s.ChildRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return models.Child{}, err
	}

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
			if uint(member["child_id"].(float64)) == child.ID {
				member["isBinded"] = false
				family[i] = member
				break
			}
		}

		familyJSON, _ := json.Marshal(family)
		parent.Family = string(familyJSON)
		s.ParentRepo.Save(parent)
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
