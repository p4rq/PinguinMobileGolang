package controllers

import (
	"PinguinMobile/models"
	"encoding/json"
	"net/http"

	"firebase.google.com/go/auth"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var DB *gorm.DB
var FirebaseAuth *auth.Client

func SetDB(db *gorm.DB) {
	DB = db
}

func SetFirebaseAuth(authClient *auth.Client) {
	FirebaseAuth = authClient
}
func ReadParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var parent models.Parent
	if err := DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": parent})
}

func UpdateParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var input struct {
		Lang     string `json:"lang"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	var parent models.Parent
	if err := DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
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

	DB.Save(&parent)
	c.JSON(http.StatusOK, gin.H{"message": "Parent updated successfully", "data": parent})
}

func DeleteParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var parent models.Parent
	if err := DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}
	DB.Delete(&parent)
	c.JSON(http.StatusOK, gin.H{"message": "Parent deleted successfully"})
}

func UnbindChild(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string `json:"parentFirebaseUid" binding:"required"`
		ChildFirebaseUID  string `json:"childFirebaseUid" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var parent models.Parent
	if err := DB.Where("firebase_uid = ?", request.ParentFirebaseUID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)
	childIndex := -1
	for i, member := range family {
		if member["firebase_uid"] == request.ChildFirebaseUID {
			childIndex = i
			break
		}
	}
	if childIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found in parent's family"})
		return
	}

	family = append(family[:childIndex], family[childIndex+1:]...)
	familyJson, _ := json.Marshal(family)
	parent.Family = string(familyJson)
	DB.Save(&parent)

	var child models.Child
	if err := DB.Where("firebase_uid = ?", request.ChildFirebaseUID).First(&child).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}
	child.IsBinded = false
	child.Family = "[]"
	DB.Save(&child)

	c.JSON(http.StatusOK, gin.H{"message": "Child unbound successfully"})
}

func MonitorChildrenUsage(c *gin.Context) {
	var input struct {
		FirebaseUID string `json:"firebase_uid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	var parent models.Parent
	if err := DB.Where("firebase_uid = ?", input.FirebaseUID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)

	var usageData []map[string]interface{}
	for _, member := range family {
		var child models.Child
		if err := DB.Where("firebase_uid = ?", member["firebase_uid"]).First(&child).Error; err == nil {
			usageData = append(usageData, map[string]interface{}{
				"child_id":   child.FirebaseUID,
				"name":       child.Name,
				"usage_data": json.Unmarshal([]byte(child.UsageData), &usageData),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "data": usageData})
}

func MonitorChildUsage(c *gin.Context) {
	var input struct {
		ParentFirebaseUID string `json:"parent_firebase_uid" binding:"required"`
		ChildFirebaseUID  string `json:"child_firebase_uid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	var parent models.Parent
	if err := DB.Where("firebase_uid = ?", input.ParentFirebaseUID).First(&parent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	var child models.Child
	if err := DB.Where("firebase_uid = ?", input.ChildFirebaseUID).First(&child).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}

	var usageData map[string]interface{}
	json.Unmarshal([]byte(child.UsageData), &usageData)
	usageData = map[string]interface{}{
		"child_id":   child.FirebaseUID,
		"name":       child.Name,
		"usage_data": usageData,
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "data": usageData})
}
