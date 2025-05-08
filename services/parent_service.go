package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"

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
