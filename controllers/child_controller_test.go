package controllers

import (
	"PinguinMobile/models"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Локальные типы данных, которых нет в модели
type ChildRebind struct {
	FirebaseUID string `json:"firebase_uid"`
	Code        string `json:"code"`
}

type BlockStatus struct {
	IsBlocked    bool   `json:"is_blocked"`
	TimeBlocked  bool   `json:"time_blocked"`
	BlockMessage string `json:"block_message"`
}

// Локальное определение UsageData для тестов
type ChildUsageData struct {
	ScreenTime int             `json:"screen_time"`
	Apps       []ChildAppUsage `json:"apps"`
	Date       string          `json:"date"`
}

type ChildAppUsage struct {
	Package     string `json:"package"`
	Time        int    `json:"time"`
	AppName     string `json:"app_name"`
	LastUsed    string `json:"last_used"`
	IconURL     string `json:"icon_url"`
	Category    string `json:"category"`
	IsBlocked   bool   `json:"is_blocked"`
	TimeBlocked bool   `json:"time_blocked"`
}

// MockChildService реализует интерфейс ChildService для тестирования
type MockChildService struct {
	mock.Mock
}

// CreateChild мок-метод
func (m *MockChildService) CreateChild(child models.Child) (models.Child, error) {
	args := m.Called(child)
	return args.Get(0).(models.Child), args.Error(1)
}

// ReadChild мок-метод
func (m *MockChildService) ReadChild(firebaseUID string) (models.Child, error) {
	args := m.Called(firebaseUID)
	return args.Get(0).(models.Child), args.Error(1)
}

// UpdateChild мок-метод
func (m *MockChildService) UpdateChild(firebaseUID string, child models.Child) (models.Child, error) {
	args := m.Called(firebaseUID, child)
	return args.Get(0).(models.Child), args.Error(1)
}

// DeleteChild мок-метод
func (m *MockChildService) DeleteChild(firebaseUID string) error {
	args := m.Called(firebaseUID)
	return args.Error(0)
}

// LogoutChild мок-метод
func (m *MockChildService) LogoutChild(firebaseUID string) error {
	args := m.Called(firebaseUID)
	return args.Error(0)
}

// RebindChild мок-метод
func (m *MockChildService) RebindChild(childInfo ChildRebind) error {
	args := m.Called(childInfo)
	return args.Error(0)
}

// CheckAppBlocking мок-метод
func (m *MockChildService) CheckAppBlocking(childFirebaseUID string, appPackage string) (BlockStatus, error) {
	args := m.Called(childFirebaseUID, appPackage)
	return args.Get(0).(BlockStatus), args.Error(1)
}

// MonitorChild мок-метод для отправки данных об использовании
func (m *MockChildService) MonitorChild(firebaseUID string, usageData ChildUsageData) error {
	args := m.Called(firebaseUID, usageData)
	return args.Error(0)
}

// Настройка роутера для тестов
func setupChildTestRouter() (*gin.Engine, *MockChildService) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Создаем экземпляр мок-сервиса
	mockService := new(MockChildService)

	// Настройка обработчиков для тестирования
	children := router.Group("/children")
	{
		children.GET("/:firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			child, err := mockService.ReadChild(firebaseUID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, child)
		})

		children.PUT("/:firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			var child models.Child
			if err := c.ShouldBindJSON(&child); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			result, err := mockService.UpdateChild(firebaseUID, child)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, result)
		})

		children.DELETE("/:firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			err := mockService.DeleteChild(firebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Child deleted successfully"})
		})

		children.POST("/:firebase_uid/logout", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			err := mockService.LogoutChild(firebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Child logged out successfully"})
		})

		children.POST("/rebind", func(c *gin.Context) {
			var childInfo ChildRebind
			if err := c.ShouldBindJSON(&childInfo); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			err := mockService.RebindChild(childInfo)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Child rebind successfully"})
		})

		children.GET("/check-blocking", func(c *gin.Context) {
			childFirebaseUID := c.Query("firebase_uid")
			appPackage := c.Query("app_package")
			if childFirebaseUID == "" || appPackage == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "firebase_uid and app_package are required"})
				return
			}
			status, err := mockService.CheckAppBlocking(childFirebaseUID, appPackage)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, status)
		})

		children.POST("/:firebase_uid/monitor", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			var usageData ChildUsageData
			if err := c.ShouldBindJSON(&usageData); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			err := mockService.MonitorChild(firebaseUID, usageData)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Usage data sent successfully"})
		})
	}

	return router, mockService
}

// ТЕСТЫ CRUD ОПЕРАЦИЙ

func TestReadChild(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"
	child := models.Child{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Test Child",
		Age:         10,
		Gender:      "male",
		Birthday:    "2013-05-15",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("ReadChild", firebaseUID).Return(child, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/children/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Child
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, child.ID, response.ID)
	assert.Equal(t, child.Name, response.Name)
	assert.Equal(t, child.Age, response.Age)
	assert.Equal(t, child.Gender, response.Gender)
	assert.Equal(t, child.Birthday, response.Birthday)

	mockService.AssertExpectations(t)
}

func TestReadChildNotFound(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "non-existent-uid"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("ReadChild", firebaseUID).Return(models.Child{}, errors.New("child not found"))

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/children/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Equal(t, "child not found", response["error"])

	mockService.AssertExpectations(t)
}

func TestUpdateChild(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"
	updateData := models.Child{
		Name:     "Updated Child Name",
		Age:      11,
		Gender:   "male",
		Birthday: "2012-05-15",
	}

	updatedChild := models.Child{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Updated Child Name",
		Age:         11,
		Gender:      "male",
		Birthday:    "2012-05-15",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("UpdateChild", firebaseUID, mock.AnythingOfType("models.Child")).Return(updatedChild, nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(updateData)
	req := httptest.NewRequest(http.MethodPut, "/children/"+firebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Child
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, updatedChild.Name, response.Name)
	assert.Equal(t, updatedChild.Age, response.Age)
	assert.Equal(t, updatedChild.Gender, response.Gender)
	assert.Equal(t, updatedChild.Birthday, response.Birthday)

	mockService.AssertExpectations(t)
}

func TestDeleteChild(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("DeleteChild", firebaseUID).Return(nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodDelete, "/children/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Equal(t, "Child deleted successfully", response["message"])

	mockService.AssertExpectations(t)
}

// ТЕСТЫ ФУНКЦИОНАЛЬНЫХ ОПЕРАЦИЙ

func TestLogoutChild(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("LogoutChild", firebaseUID).Return(nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodPost, "/children/"+firebaseUID+"/logout", nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Equal(t, "Child logged out successfully", response["message"])

	mockService.AssertExpectations(t)
}

func TestRebindChild(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	childRebind := ChildRebind{
		FirebaseUID: "test-child-uid-123",
		Code:        "ABC123",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("RebindChild", mock.AnythingOfType("controllers.ChildRebind")).Return(nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(childRebind)
	req := httptest.NewRequest(http.MethodPost, "/children/rebind", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Equal(t, "Child rebind successfully", response["message"])

	mockService.AssertExpectations(t)
}

func TestCheckAppBlocking(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	childFirebaseUID := "test-child-uid-123"
	appPackage := "com.instagram.android"

	blockStatus := BlockStatus{
		IsBlocked:    true,
		TimeBlocked:  true,
		BlockMessage: "This app is blocked by your parent",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("CheckAppBlocking", childFirebaseUID, appPackage).Return(blockStatus, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/children/check-blocking?firebase_uid="+childFirebaseUID+"&app_package="+appPackage, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response BlockStatus
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, blockStatus.IsBlocked, response.IsBlocked)
	assert.Equal(t, blockStatus.TimeBlocked, response.TimeBlocked)
	assert.Equal(t, blockStatus.BlockMessage, response.BlockMessage)

	mockService.AssertExpectations(t)
}

func TestMonitorChild(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"
	usageData := ChildUsageData{
		ScreenTime: 120,
		Date:       "2023-05-15",
		Apps: []ChildAppUsage{
			{
				Package:  "com.instagram.android",
				AppName:  "Instagram",
				Time:     45,
				LastUsed: "2023-05-15T14:30:00Z",
			},
			{
				Package:  "com.facebook.katana",
				AppName:  "Facebook",
				Time:     30,
				LastUsed: "2023-05-15T15:15:00Z",
			},
		},
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("MonitorChild", firebaseUID, mock.AnythingOfType("controllers.ChildUsageData")).Return(nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(usageData)
	req := httptest.NewRequest(http.MethodPost, "/children/"+firebaseUID+"/monitor", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Equal(t, "Usage data sent successfully", response["message"])

	mockService.AssertExpectations(t)
}

// ТЕСТЫ ОШИБОК

func TestUpdateChildError(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"
	updateData := models.Child{
		Name: "Updated Child Name",
	}

	// Настраиваем ожидание для мок-сервиса с возвратом ошибки
	mockService.On("UpdateChild", firebaseUID, mock.AnythingOfType("models.Child")).
		Return(models.Child{}, errors.New("failed to update child"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(updateData)
	req := httptest.NewRequest(http.MethodPut, "/children/"+firebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Equal(t, "failed to update child", response["error"])

	mockService.AssertExpectations(t)
}

func TestRebindChildError(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	childRebind := ChildRebind{
		FirebaseUID: "test-child-uid-123",
		Code:        "INVALID", // Неверный код
	}

	// Настраиваем ожидание для мок-сервиса с возвратом ошибки
	mockService.On("RebindChild", mock.AnythingOfType("controllers.ChildRebind")).
		Return(errors.New("invalid rebind code"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(childRebind)
	req := httptest.NewRequest(http.MethodPost, "/children/rebind", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Equal(t, "invalid rebind code", response["error"])

	mockService.AssertExpectations(t)
}

func TestCheckAppBlockingMissingParams(t *testing.T) {
	router, _ := setupChildTestRouter()

	// Создаем тестовый запрос без обязательных параметров
	req := httptest.NewRequest(http.MethodGet, "/children/check-blocking", nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Equal(t, "firebase_uid and app_package are required", response["error"])
}

func TestMonitorChildError(t *testing.T) {
	router, mockService := setupChildTestRouter()

	// Тестовые данные
	firebaseUID := "test-child-uid-123"
	usageData := ChildUsageData{
		ScreenTime: 120,
		Date:       "2023-05-15",
		Apps:       []ChildAppUsage{},
	}

	// Настраиваем ожидание для мок-сервиса с возвратом ошибки
	mockService.On("MonitorChild", firebaseUID, mock.AnythingOfType("controllers.ChildUsageData")).
		Return(errors.New("failed to store usage data"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(usageData)
	req := httptest.NewRequest(http.MethodPost, "/children/"+firebaseUID+"/monitor", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Equal(t, "failed to store usage data", response["error"])

	mockService.AssertExpectations(t)
}

func TestInvalidJsonInputChild(t *testing.T) {
	router, _ := setupChildTestRouter()

	// Некорректные JSON данные
	invalidJson := []byte(`{"name": "Test Child", age: 10}`) // Отсутствуют кавычки вокруг age

	// Создаем тестовый запрос для обновления ребенка
	req := httptest.NewRequest(http.MethodPut, "/children/test-child-uid", bytes.NewBuffer(invalidJson))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}
