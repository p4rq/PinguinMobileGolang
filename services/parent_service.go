package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
	"fmt"

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
		return err
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)
	childIndex := -1
	for i, member := range family {
		fmt.Printf("Checking member: %v\n", member) // Отладочное сообщение
		if member["firebase_uid"] == childFirebaseUID {
			childIndex = i
			break
		}
	}
	if childIndex == -1 {
		return errors.New("child not found in parent's family")
	}

	family = append(family[:childIndex], family[childIndex+1:]...)
	familyJson, _ := json.Marshal(family)
	parent.Family = string(familyJson)
	if err := s.ParentRepo.Save(parent); err != nil {
		return err
	}

	child, err := s.ChildRepo.FindByFirebaseUID(childFirebaseUID)
	if err != nil {
		return err
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
