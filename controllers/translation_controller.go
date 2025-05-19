package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetTranslations(c *gin.Context) {
	lang := c.DefaultQuery("lang", "en")

	if translationService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Translation service not initialized",
		})
		return
	}

	translations := translationService.GetAllTranslations(lang)

	c.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"translations": translations,
	})
}
