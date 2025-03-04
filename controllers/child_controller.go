package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

var childService *services.ChildService

func SetChildService(service *services.ChildService) {
	childService = service
}

func ReadChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	child, err := childService.ReadChild(firebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": true, "data": child})
}

func UpdateChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var input struct {
		Lang     string `json:"lang"`
		Name     string `json:"name"`
		Gender   string `json:"gender"`
		Age      int    `json:"age"`
		Birthday string `json:"birthday"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	child, err := childService.UpdateChild(firebaseUID, models.Child{
		Lang:     input.Lang,
		Name:     input.Name,
		Gender:   input.Gender,
		Age:      input.Age,
		Birthday: input.Birthday,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child updated successfully", "data": child})
}

func DeleteChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	if err := childService.DeleteChild(firebaseUID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Child deleted successfully"})
}

func LogoutChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	child, err := childService.LogoutChild(firebaseUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child logged out successfully", "data": child})
}

func MonitorChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var input struct {
		Sessions []models.Session `json:"sessions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	child, err := childService.MonitorChild(firebaseUID, input.Sessions)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Child not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child usage data updated successfully", "data": child})
}

func RebindChild(c *gin.Context) {
	var request struct {
		ChildCode         string `json:"childCode" binding:"required"`
		ParentFirebaseUID string `json:"parentFirebaseUID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	child, err := childService.RebindChild(request.ChildCode, request.ParentFirebaseUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, child)
}
