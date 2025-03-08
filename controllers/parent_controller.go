package controllers

import (
	"PinguinMobile/services"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

var parentService *services.ParentService

func SetParentService(service *services.ParentService) {
	parentService = service
}

func ReadParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	parent, err := parentService.ReadParent(firebaseUID)
	if err != nil {
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

	parent, err := parentService.ReadParent(firebaseUID)
	if err != nil {
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

	updatedParent, err := parentService.UpdateParent(firebaseUID, parent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Parent updated successfully", "data": updatedParent})
}

func DeleteParent(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	if err := parentService.DeleteParent(firebaseUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
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

	err := parentService.UnbindChild(request.ParentFirebaseUID, request.ChildFirebaseUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	parent, err := parentService.ReadParent(request.ParentFirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child unbound successfully", "parent": parent})
}

func MonitorChildrenUsage(c *gin.Context) {
	var input struct {
		FirebaseUID string `json:"firebase_uid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	parent, err := parentService.ReadParent(input.FirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)

	var usageData []map[string]interface{}
	for _, member := range family {
		child, err := parentService.ReadChild(member["firebase_uid"].(string))
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

	_, err := parentService.ReadParent(input.ParentFirebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Parent not found"})
		return
	}

	child, err := parentService.ReadChild(input.ChildFirebaseUID)
	if err != nil {
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
func BlockApps(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string   `json:"parentFirebaseUid" binding:"required"`
		ChildFirebaseUID  string   `json:"childFirebaseUid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := parentService.BlockApps(request.ParentFirebaseUID, request.ChildFirebaseUID, request.Apps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Apps blocked successfully"})
}

func UnblockApps(c *gin.Context) {
	var request struct {
		ParentFirebaseUID string   `json:"parentFirebaseUid" binding:"required"`
		ChildFirebaseUID  string   `json:"childFirebaseUid" binding:"required"`
		Apps              []string `json:"apps" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := parentService.UnblockApps(request.ParentFirebaseUID, request.ChildFirebaseUID, request.Apps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked successfully"})
}
