package controllers

import (
	"PinguinMobile/models"
	"PinguinMobile/services"
	"encoding/json"
	"fmt"
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

	// Получаем дополнительные параметры от клиента, если нужно
	var request struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&request) // Игнорируем ошибки, так как параметры необязательные

	// Вызываем сервис для выхода ребенка с отправкой уведомления родителю
	child, err := childService.LogoutChild(firebaseUID, request.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, child)
}

// MonitorChild обрабатывает получение данных об использовании устройства ребенком
func MonitorChild(c *gin.Context) {
	firebaseUID := c.Param("firebase_uid")

	var input struct {
		UsageData json.RawMessage `json:"usage_data" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Логируем входные данные для отладки
	fmt.Printf("Received usage data for child %s: %s\n", firebaseUID, string(input.UsageData))

	// Обрабатываем данные с учетом новой логики (кумулятивные данные за день)
	err := parentService.MonitorChildWithDailyData(firebaseUID, input.UsageData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Child usage monitored successfully"})
}

func RebindChild(c *gin.Context) {
	var request struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	child, err := childService.RebindChild(request.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, child)
}
