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

// MockParentService реализует интерфейс ParentService для тестирования
type MockParentService struct {
	mock.Mock
}

// CreateParent мок-метод
func (m *MockParentService) CreateParent(parent models.Parent) (models.Parent, error) {
	args := m.Called(parent)
	return args.Get(0).(models.Parent), args.Error(1)
}

// ReadParent мок-метод
func (m *MockParentService) ReadParent(firebaseUID string) (models.Parent, error) {
	args := m.Called(firebaseUID)
	return args.Get(0).(models.Parent), args.Error(1)
}

// UpdateParent мок-метод
func (m *MockParentService) UpdateParent(firebaseUID string, parent models.Parent) (models.Parent, error) {
	args := m.Called(firebaseUID, parent)
	return args.Get(0).(models.Parent), args.Error(1)
}

// DeleteParent мок-метод
func (m *MockParentService) DeleteParent(firebaseUID string) error {
	args := m.Called(firebaseUID)
	return args.Error(0)
}

// GetFamily мок-метод
func (m *MockParentService) GetFamily(parentFirebaseUID string) ([]models.Child, error) {
	args := m.Called(parentFirebaseUID)
	return args.Get(0).([]models.Child), args.Error(1)
}

// AddChildToFamily мок-метод
func (m *MockParentService) AddChildToFamily(parentFirebaseUID string, child models.Child) (string, error) {
	args := m.Called(parentFirebaseUID, child)
	return args.String(0), args.Error(1)
}

// RemoveChildFromFamily мок-метод
func (m *MockParentService) RemoveChildFromFamily(parentFirebaseUID, childFirebaseUID string) error {
	args := m.Called(parentFirebaseUID, childFirebaseUID)
	return args.Error(0)
}

// BlockAppsByTime мок-метод
func (m *MockParentService) BlockAppsByTime(parentFirebaseUID, childFirebaseUID string, blocks []models.AppTimeBlock) error {
	args := m.Called(parentFirebaseUID, childFirebaseUID, blocks)
	return args.Error(0)
}

// UnblockAppsByTime мок-метод
func (m *MockParentService) UnblockAppsByTime(parentFirebaseUID, childFirebaseUID string, appPackages []string) error {
	args := m.Called(parentFirebaseUID, childFirebaseUID, appPackages)
	return args.Error(0)
}

// GetTimeBlockedApps мок-метод
func (m *MockParentService) GetTimeBlockedApps(parentFirebaseUID, childFirebaseUID string) ([]models.AppTimeBlock, error) {
	args := m.Called(parentFirebaseUID, childFirebaseUID)
	return args.Get(0).([]models.AppTimeBlock), args.Error(1)
}

// Локальные типы данных для тестирования
type UsageData struct {
	ScreenTime int        `json:"screen_time"`
	Apps       []AppUsage `json:"apps"`
	Date       string     `json:"date"`
}

type AppUsage struct {
	Package     string `json:"package"`
	Time        int    `json:"time"`
	AppName     string `json:"app_name"`
	LastUsed    string `json:"last_used"`
	IconURL     string `json:"icon_url"`
	Category    string `json:"category"`
	IsBlocked   bool   `json:"is_blocked"`
	TimeBlocked bool   `json:"time_blocked"`
}

// MonitorChildUsage мок-метод
func (m *MockParentService) MonitorChildUsage(parentFirebaseUID, childFirebaseUID string) (UsageData, error) {
	args := m.Called(parentFirebaseUID, childFirebaseUID)
	return args.Get(0).(UsageData), args.Error(1)
}

// MonitorChildrenUsage мок-метод
func (m *MockParentService) MonitorChildrenUsage(parentFirebaseUID string) (map[string]UsageData, error) {
	args := m.Called(parentFirebaseUID)
	return args.Get(0).(map[string]UsageData), args.Error(1)
}

// Настройка роутера для тестов
func setupParentTestRouter() (*gin.Engine, *MockParentService) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	// Создаем экземпляр мок-сервиса
	mockService := new(MockParentService)

	// Настройка обработчиков для тестирования
	parents := router.Group("/parents")
	{
		parents.POST("", func(c *gin.Context) {
			var parent models.Parent
			if err := c.ShouldBindJSON(&parent); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			result, err := mockService.CreateParent(parent)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, result)
		})

		parents.GET("/:firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			parent, err := mockService.ReadParent(firebaseUID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, parent)
		})

		parents.PUT("/:firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			var parent models.Parent
			if err := c.ShouldBindJSON(&parent); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			result, err := mockService.UpdateParent(firebaseUID, parent)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, result)
		})

		parents.DELETE("/:firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			err := mockService.DeleteParent(firebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Parent deleted successfully"})
		})

		// Семейные маршруты
		parents.GET("/:firebase_uid/family", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			children, err := mockService.GetFamily(firebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"children": children})
		})

		parents.POST("/:firebase_uid/family", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			var child models.Child
			if err := c.ShouldBindJSON(&child); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			code, err := mockService.AddChildToFamily(firebaseUID, child)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": code})
		})

		parents.DELETE("/:firebase_uid/family/:child_firebase_uid", func(c *gin.Context) {
			firebaseUID := c.Param("firebase_uid")
			childFirebaseUID := c.Param("child_firebase_uid")
			err := mockService.RemoveChildFromFamily(firebaseUID, childFirebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Child removed successfully"})
		})

		// Маршруты для блокировки приложений по времени
		parents.GET("/:firebase_uid/block-apps/:child_firebase_uid", func(c *gin.Context) {
			parentFirebaseUID := c.Param("firebase_uid")
			childFirebaseUID := c.Param("child_firebase_uid")
			blocks, err := mockService.GetTimeBlockedApps(parentFirebaseUID, childFirebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"blocks": blocks})
		})

		parents.POST("/:firebase_uid/block-apps/:child_firebase_uid", func(c *gin.Context) {
			parentFirebaseUID := c.Param("firebase_uid")
			childFirebaseUID := c.Param("child_firebase_uid")

			var blocks []models.AppTimeBlock
			if err := c.ShouldBindJSON(&blocks); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			err := mockService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, blocks)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Apps blocked successfully"})
		})

		parents.DELETE("/:firebase_uid/block-apps/:child_firebase_uid", func(c *gin.Context) {
			parentFirebaseUID := c.Param("firebase_uid")
			childFirebaseUID := c.Param("child_firebase_uid")

			var appPackages []string
			if err := c.ShouldBindJSON(&appPackages); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			err := mockService.UnblockAppsByTime(parentFirebaseUID, childFirebaseUID, appPackages)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "Apps unblocked successfully"})
		})
	}

	// Маршруты для мониторинга
	monitor := router.Group("/parents/monitor")
	{
		monitor.POST("/child", func(c *gin.Context) {
			var request struct {
				ParentFirebaseUID string `json:"parent_firebase_uid"`
				ChildFirebaseUID  string `json:"child_firebase_uid"`
			}
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			usage, err := mockService.MonitorChildUsage(request.ParentFirebaseUID, request.ChildFirebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"usage": usage})
		})

		monitor.POST("", func(c *gin.Context) {
			var request struct {
				ParentFirebaseUID string `json:"parent_firebase_uid"`
			}
			if err := c.ShouldBindJSON(&request); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			usage, err := mockService.MonitorChildrenUsage(request.ParentFirebaseUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"usage": usage})
		})
	}

	return router, mockService
}

// ТЕСТЫ CRUD ОПЕРАЦИЙ

func TestCreateParent(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	newParent := models.Parent{
		FirebaseUID: "test-uid-123",
		Name:        "Test User",
		Email:       "test@example.com",
		Lang:        "ru",
	}

	// Ожидаемый результат с ID
	createdParent := newParent
	createdParent.ID = 1

	// Настраиваем ожидание для мок-сервиса
	mockService.On("CreateParent", mock.AnythingOfType("models.Parent")).Return(createdParent, nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(newParent)
	req := httptest.NewRequest(http.MethodPost, "/parents", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Parent
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, createdParent.ID, response.ID)
	assert.Equal(t, createdParent.Name, response.Name)
	assert.Equal(t, createdParent.Email, response.Email)

	mockService.AssertExpectations(t)
}

func TestReadParent(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "test-uid-123"
	parent := models.Parent{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Test User",
		Email:       "test@example.com",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("ReadParent", firebaseUID).Return(parent, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Parent
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, parent.ID, response.ID)
	assert.Equal(t, parent.Name, response.Name)
	assert.Equal(t, parent.Email, response.Email)

	mockService.AssertExpectations(t)
}

func TestReadParentNotFound(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "non-existent-uid"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("ReadParent", firebaseUID).Return(models.Parent{}, errors.New("parent not found"))

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")

	mockService.AssertExpectations(t)
}

func TestUpdateParent(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "test-uid-123"
	updateData := models.Parent{
		Name:  "Updated Name",
		Email: "updated@example.com",
	}

	updatedParent := models.Parent{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Updated Name",
		Email:       "updated@example.com",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("UpdateParent", firebaseUID, mock.AnythingOfType("models.Parent")).Return(updatedParent, nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(updateData)
	req := httptest.NewRequest(http.MethodPut, "/parents/"+firebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response models.Parent
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, updatedParent.Name, response.Name)
	assert.Equal(t, updatedParent.Email, response.Email)

	mockService.AssertExpectations(t)
}

func TestDeleteParent(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "test-uid-123"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("DeleteParent", firebaseUID).Return(nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodDelete, "/parents/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Equal(t, "Parent deleted successfully", response["message"])

	mockService.AssertExpectations(t)
}

// ТЕСТЫ СЕМЕЙНЫХ ОПЕРАЦИЙ

func TestGetFamily(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "test-uid-123"
	family := []models.Child{
		{
			ID:          1,
			FirebaseUID: "child-uid-1",
			Name:        "Child One",
			Age:         10,
			Gender:      "female",
			Birthday:    "2013-05-15",
		},
		{
			ID:          2,
			FirebaseUID: "child-uid-2",
			Name:        "Child Two",
			Age:         8,
			Gender:      "male",
			Birthday:    "2015-03-20",
		},
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("GetFamily", firebaseUID).Return(family, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+firebaseUID+"/family", nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Children []models.Child `json:"children"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Children))
	assert.Equal(t, family[0].Name, response.Children[0].Name)
	assert.Equal(t, family[1].Name, response.Children[1].Name)

	mockService.AssertExpectations(t)
}

func TestAddChildToFamily(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "test-uid-123"
	childInfo := models.Child{
		Name:     "Test Child",
		Age:      10,
		Gender:   "male",
		Birthday: "2013-05-15",
	}

	// Код подключения, который должен вернуть сервис
	connectCode := "ABC123"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("AddChildToFamily", firebaseUID, mock.AnythingOfType("models.Child")).Return(connectCode, nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(childInfo)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+firebaseUID+"/family", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Code string `json:"code"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, connectCode, response.Code)

	mockService.AssertExpectations(t)
}

func TestRemoveChildFromFamily(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "child-uid-1"

	// Настраиваем ожидание для мок-сервиса
	mockService.On("RemoveChildFromFamily", parentFirebaseUID, childFirebaseUID).Return(nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodDelete, "/parents/"+parentFirebaseUID+"/family/"+childFirebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "message")
	assert.Equal(t, "Child removed successfully", response["message"])

	mockService.AssertExpectations(t)
}

// ТЕСТЫ БЛОКИРОВКИ ПРИЛОЖЕНИЙ

func TestGetTimeBlockedApps(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "child-uid-1"

	blockedApps := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
		{
			AppPackage: "com.facebook.katana",
			StartTime:  "20:00",
			EndTime:    "22:00",
			DaysOfWeek: "1,2,3,4,5,6,7",
		},
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("GetTimeBlockedApps", parentFirebaseUID, childFirebaseUID).Return(blockedApps, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Blocks []models.AppTimeBlock `json:"blocks"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Blocks))
	assert.Equal(t, blockedApps[0].AppPackage, response.Blocks[0].AppPackage)
	assert.Equal(t, blockedApps[1].AppPackage, response.Blocks[1].AppPackage)

	mockService.AssertExpectations(t)
}

func TestBlockAppsByTime(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "child-uid-1"

	blocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("BlockAppsByTime", parentFirebaseUID, childFirebaseUID, mock.AnythingOfType("[]models.AppTimeBlock")).Return(nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(blocks)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, bytes.NewBuffer(requestBody))
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
	assert.Equal(t, "Apps blocked successfully", response["message"])

	mockService.AssertExpectations(t)
}

func TestUnblockAppsByTime(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "child-uid-1"

	appPackages := []string{"com.instagram.android", "com.facebook.katana"}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("UnblockAppsByTime", parentFirebaseUID, childFirebaseUID, appPackages).Return(nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(appPackages)
	req := httptest.NewRequest(http.MethodDelete, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, bytes.NewBuffer(requestBody))
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
	assert.Equal(t, "Apps unblocked successfully", response["message"])

	mockService.AssertExpectations(t)
}

// ТЕСТЫ МОНИТОРИНГА

func TestMonitorChildUsage(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "child-uid-1"

	usageData := UsageData{
		ScreenTime: 120,
		Date:       "2023-05-15",
		Apps: []AppUsage{
			{
				Package: "com.instagram.android",
				AppName: "Instagram",
				Time:    45,
			},
			{
				Package: "com.facebook.katana",
				AppName: "Facebook",
				Time:    30,
			},
		},
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("MonitorChildUsage", parentFirebaseUID, childFirebaseUID).Return(usageData, nil)

	// Создаем тестовый запрос
	requestData := struct {
		ParentFirebaseUID string `json:"parent_firebase_uid"`
		ChildFirebaseUID  string `json:"child_firebase_uid"`
	}{
		ParentFirebaseUID: parentFirebaseUID,
		ChildFirebaseUID:  childFirebaseUID,
	}

	requestBody, _ := json.Marshal(requestData)
	req := httptest.NewRequest(http.MethodPost, "/parents/monitor/child", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Usage UsageData `json:"usage"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, usageData.ScreenTime, response.Usage.ScreenTime)
	assert.Equal(t, usageData.Date, response.Usage.Date)
	assert.Equal(t, 2, len(response.Usage.Apps))
	assert.Equal(t, "Instagram", response.Usage.Apps[0].AppName)
	assert.Equal(t, "Facebook", response.Usage.Apps[1].AppName)

	mockService.AssertExpectations(t)
}

func TestMonitorChildrenUsage(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"

	usageDataMap := map[string]UsageData{
		"child-uid-1": {
			ScreenTime: 120,
			Date:       "2023-05-15",
			Apps: []AppUsage{
				{
					Package: "com.instagram.android",
					AppName: "Instagram",
					Time:    45,
				},
			},
		},
		"child-uid-2": {
			ScreenTime: 90,
			Date:       "2023-05-15",
			Apps: []AppUsage{
				{
					Package: "com.facebook.katana",
					AppName: "Facebook",
					Time:    30,
				},
			},
		},
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("MonitorChildrenUsage", parentFirebaseUID).Return(usageDataMap, nil)

	// Создаем тестовый запрос
	requestData := struct {
		ParentFirebaseUID string `json:"parent_firebase_uid"`
	}{
		ParentFirebaseUID: parentFirebaseUID,
	}

	requestBody, _ := json.Marshal(requestData)
	req := httptest.NewRequest(http.MethodPost, "/parents/monitor", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Usage map[string]UsageData `json:"usage"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Usage))
	assert.Equal(t, 120, response.Usage["child-uid-1"].ScreenTime)
	assert.Equal(t, 90, response.Usage["child-uid-2"].ScreenTime)

	mockService.AssertExpectations(t)
}

// ТЕСТЫ ОШИБОК

func TestCreateParentError(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	invalidParent := models.Parent{
		// Отсутствует FirebaseUID
		Name:  "Test User",
		Email: "test@example.com",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("CreateParent", mock.AnythingOfType("models.Parent")).Return(models.Parent{}, errors.New("missing required firebase_uid"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(invalidParent)
	req := httptest.NewRequest(http.MethodPost, "/parents", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(t)
}

func TestAddChildToFamilyError(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	firebaseUID := "test-uid-123"
	childInfo := models.Child{
		Name:     "Test Child",
		Age:      10,
		Gender:   "male",
		Birthday: "2013-05-15",
	}

	// Настраиваем ожидание для мок-сервиса
	mockService.On("AddChildToFamily", firebaseUID, mock.AnythingOfType("models.Child")).Return("", errors.New("failed to add child"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(childInfo)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+firebaseUID+"/family", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(t)
}

func TestBlockAppsByTimeError(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "child-uid-1"

	blocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Настраиваем ожидание для мок-сервиса с возвратом ошибки
	mockService.On("BlockAppsByTime", parentFirebaseUID, childFirebaseUID, mock.AnythingOfType("[]models.AppTimeBlock")).Return(errors.New("failed to block apps"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(blocks)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, bytes.NewBuffer(requestBody))
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
	assert.Equal(t, "failed to block apps", response["error"])

	mockService.AssertExpectations(t)
}

func TestMonitorChildUsageError(t *testing.T) {
	router, mockService := setupParentTestRouter()

	// Тестовые данные
	parentFirebaseUID := "test-uid-123"
	childFirebaseUID := "non-existent-child"

	// Настраиваем ожидание для мок-сервиса с возвратом ошибки
	mockService.On("MonitorChildUsage", parentFirebaseUID, childFirebaseUID).Return(UsageData{}, errors.New("child not found"))

	// Создаем тестовый запрос
	requestData := struct {
		ParentFirebaseUID string `json:"parent_firebase_uid"`
		ChildFirebaseUID  string `json:"child_firebase_uid"`
	}{
		ParentFirebaseUID: parentFirebaseUID,
		ChildFirebaseUID:  childFirebaseUID,
	}

	requestBody, _ := json.Marshal(requestData)
	req := httptest.NewRequest(http.MethodPost, "/parents/monitor/child", bytes.NewBuffer(requestBody))
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
	assert.Equal(t, "child not found", response["error"])

	mockService.AssertExpectations(t)
}

func TestInvalidJsonInput(t *testing.T) {
	router, _ := setupParentTestRouter()

	// Создаем некорректный JSON запрос
	invalidJson := []byte(`{"name": "Test", email: "invalid-json"}`)
	req := httptest.NewRequest(http.MethodPost, "/parents", bytes.NewBuffer(invalidJson))
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
