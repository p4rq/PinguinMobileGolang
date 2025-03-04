package services

import (
	"PinguinMobile/models"
	"encoding/json"
	"time"

	"firebase.google.com/go/auth"
	"gorm.io/gorm"
)

type ChildService struct {
	DB           *gorm.DB
	FirebaseAuth *auth.Client
}

func NewChildService(db *gorm.DB, firebaseAuth *auth.Client) *ChildService {
	return &ChildService{DB: db, FirebaseAuth: firebaseAuth}
}

func (s *ChildService) ReadChild(firebaseUID string) (models.Child, error) {
	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&child).Error; err != nil {
		return models.Child{}, err
	}
	return child, nil
}

func (s *ChildService) UpdateChild(firebaseUID string, input models.Child) (models.Child, error) {
	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&child).Error; err != nil {
		return models.Child{}, err
	}

	child.Lang = input.Lang
	child.Name = input.Name
	child.Gender = input.Gender
	child.Age = input.Age
	child.Birthday = input.Birthday

	if err := s.DB.Save(&child).Error; err != nil {
		return models.Child{}, err
	}

	// Update parent's family JSON
	var familyData map[string]interface{}
	json.Unmarshal([]byte(child.Family), &familyData)
	parentID := uint(familyData["parent_id"].(float64))

	var parent models.Parent
	if err := s.DB.First(&parent, parentID).Error; err == nil {
		var family []map[string]interface{}
		json.Unmarshal([]byte(parent.Family), &family)

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
			})
		}

		familyJSON, _ := json.Marshal(family)
		parent.Family = string(familyJSON)
		s.DB.Save(&parent)
	}

	return child, nil
}

func (s *ChildService) DeleteChild(firebaseUID string) error {
	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&child).Error; err != nil {
		return err
	}
	if err := s.DB.Delete(&child).Error; err != nil {
		return err
	}
	return nil
}

func (s *ChildService) LogoutChild(firebaseUID string) (models.Child, error) {
	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&child).Error; err != nil {
		return models.Child{}, err
	}

	child.IsBinded = false
	if err := s.DB.Save(&child).Error; err != nil {
		return models.Child{}, err
	}

	// Update parent's family JSON
	var familyData map[string]interface{}
	json.Unmarshal([]byte(child.Family), &familyData)
	parentID := uint(familyData["parent_id"].(float64))

	var parent models.Parent
	if err := s.DB.First(&parent, parentID).Error; err == nil {
		var family []map[string]interface{}
		json.Unmarshal([]byte(parent.Family), &family)
		for i, member := range family {
			if uint(member["child_id"].(float64)) == child.ID {
				member["isBinded"] = false
				family[i] = member
				break
			}
		}
		familyJSON, _ := json.Marshal(family)
		parent.Family = string(familyJSON)
		s.DB.Save(&parent)
	}

	return child, nil
}

func (s *ChildService) MonitorChild(firebaseUID string, sessions []models.Session) (models.Child, error) {
	var child models.Child
	if err := s.DB.Where("firebase_uid = ?", firebaseUID).First(&child).Error; err != nil {
		return models.Child{}, err
	}

	var usageData map[string]interface{}
	if child.UsageData != "" {
		json.Unmarshal([]byte(child.UsageData), &usageData)
	} else {
		usageData = make(map[string]interface{})
	}

	if usageData["sessions"] == nil {
		usageData["sessions"] = []interface{}{}
	}

	sessionsData := usageData["sessions"].([]interface{})
	for _, newSession := range sessions {
		merged := false
		for i, existingSession := range sessionsData {
			existingSessionMap := existingSession.(map[string]interface{})
			if existingSessionMap["app"] == newSession.App {
				existingTimestamp, _ := time.Parse(time.RFC3339, existingSessionMap["timestamp"].(string))
				newTimestamp, _ := time.Parse(time.RFC3339, newSession.Timestamp)
				if newTimestamp.Sub(existingTimestamp) < 24*time.Hour {
					existingDuration := existingSessionMap["duration"].(float64)
					newDuration := newSession.Duration
					existingSessionMap["duration"] = existingDuration + newDuration
					sessionsData[i] = existingSessionMap
					merged = true
					break
				}
			}
		}
		if !merged {
			sessionsData = append(sessionsData, map[string]interface{}{
				"app":       newSession.App,
				"duration":  newSession.Duration,
				"timestamp": newSession.Timestamp,
			})
		}
	}
	usageData["sessions"] = sessionsData

	usageDataJSON, _ := json.Marshal(usageData)
	child.UsageData = string(usageDataJSON)
	if err := s.DB.Save(&child).Error; err != nil {
		return models.Child{}, err
	}

	return child, nil
}

func (s *ChildService) RebindChild(childCode, parentFirebaseUID string) (models.Child, error) {
	var child models.Child
	if err := s.DB.Where("code = ?", childCode).First(&child).Error; err != nil {
		return models.Child{}, err
	}

	var parent models.Parent
	if err := s.DB.Where("firebase_uid = ?", parentFirebaseUID).First(&parent).Error; err != nil {
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
	if err := s.DB.Save(&child).Error; err != nil {
		return models.Child{}, err
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)

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
	if err := s.DB.Save(&parent).Error; err != nil {
		return models.Child{}, err
	}

	return child, nil
}
