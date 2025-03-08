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
	c.JSON(http.StatusOK, child)
}

func UpdateChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	var input models.Child
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	child, err := childService.UpdateChild(firebaseUID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, child)
}

func DeleteChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	if err := childService.DeleteChild(firebaseUID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Child deleted successfully"})
}

func LogoutChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")
	child, err := childService.LogoutChild(firebaseUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, child)
}

func MonitorChild(c *gin.Context) {
	var request struct {
		Sessions []models.Session `json:"sessions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	childFirebaseUID := c.Param("firebase_uid")
	err := childService.MonitorChild(childFirebaseUID, request.Sessions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child usage monitored successfully"})
}

func RebindChild(c *gin.Context) {
	var request struct {
		Code              string `json:"code" binding:"required"`
		ParentFirebaseUID string `json:"parentFirebaseUID" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	child, err := childService.RebindChild(request.Code, request.ParentFirebaseUID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, child)
}
