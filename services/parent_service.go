package services

import (
	"PinguinMobile/models"
	"encoding/json"
	"errors"

	"firebase.google.com/go/auth"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ParentService struct {
	DB           *gorm.DB
	FirebaseAuth *auth.Client
}

func NewParentService(db *gorm.DB, firebaseAuth *auth.Client) *ParentService {
	return &ParentService{DB: db, FirebaseAuth: firebaseAuth}
}

func (s *ParentService) ReadParent(firebaseUID string) (models.Parent, error) {
	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		return models.Parent{}, err
	}
	return parent, nil
}

func (s *ParentService) UpdateParent(firebaseUID string, input models.Parent) (models.Parent, error) {
	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
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

	if err := s.DB.Save(&parent).Error; err != nil {
		return models.Parent{}, err
	}

	return parent, nil
}

func (s *ParentService) DeleteParent(firebaseUID string) error {
	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		return err
	}
	if err := s.DB.Delete(&parent).Error; err != nil {
		return err
	}
	return nil
}

func (s *ParentService) UnbindChild(parentFirebaseUID, childFirebaseUID string) error {
	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", parentFirebaseUID).First(&parent).Error; err != nil {
		return err
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)
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

	family = append(family[:childIndex], family[childIndex+1:]...)
	familyJson, _ := json.Marshal(family)
	parent.Family = string(familyJson)
	s.DB.Save(&parent)

	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", childFirebaseUID).First(&child).Error; err != nil {
		return err
	}
	child.IsBinded = false
	child.Family = "[]"
	s.DB.Save(&child)

	return nil
}

func (s *ParentService) MonitorChildrenUsage(firebaseUID string) ([]map[string]interface{}, error) {
	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		return nil, err
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)

	var usageData []map[string]interface{}
	for _, member := range family {
		var child models.Child
		if err := s.DB.Where("firebase_uid = ?", member["firebase_uid"]).First(&child).Error; err == nil {
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
	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", parentFirebaseUID).First(&parent).Error; err != nil {
		return nil, err
	}

	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", childFirebaseUID).First(&child).Error; err != nil {
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
