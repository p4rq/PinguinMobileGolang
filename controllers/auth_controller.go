package controllers

import (
	"PinguinMobile/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

var authService *services.AuthService

func SetAuthService(service *services.AuthService) {
	authService = service
}

func RegisterParent(c *gin.Context) {
	var input struct {
		Lang     string `json:"lang" binding:"required"`
		Name     string `json:"name" binding:"required"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	if input.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password cannot be empty"})
		return
	}

	parent, token, err := authService.RegisterParent(input.Lang, input.Name, input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": true, "token": token, "data": parent})
}

func LoginParent(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	parent, token, err := authService.LoginParent(input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "token": token, "user": parent})
}

func RegisterChild(c *gin.Context) {
	var input struct {
		Lang string `json:"lang" binding:"required"`
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	// Передаем пустую строку в качестве имени
	child, token, err := authService.RegisterChild(input.Lang, input.Code, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": true, "token": token, "data": child})
}

func TokenVerify(c *gin.Context) {
	var input struct {
		UID string `json:"uid" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	user, err := authService.VerifyToken(input.UID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "user": user})
}

// LoginChild logs in a child using their code
func LoginChild(c *gin.Context) {
	var input struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errors": err.Error()})
		return
	}

	child, token, err := authService.LoginChild(input.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": true, "token": token, "user": child})
}
