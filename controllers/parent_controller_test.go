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

// Структуры для мока сервиса родителя
type MockParentService struct {
	mock.Mock
}

// Методы для мока сервиса родителя
func (m *MockParentService) CreateParent(parent models.Parent) (models.Parent, error) {
	args := m.Called(parent)
	return args.Get(0).(models.Parent), args.Error(1)
}

func (m *MockParentService) ReadParent(firebaseUID string) (models.Parent, error) {
	args := m.Called(firebaseUID)
	return args.Get(0).(models.Parent), args.Error(1)
}

func (m *MockParentService) UpdateParent(firebaseUID string, input models.Parent) (models.Parent, error) {
	args := m.Called(firebaseUID, input)
	return args.Get(0).(models.Parent), args.Error(1)
}

func (m *MockParentService) DeleteParent(firebaseUID string) error {
	args := m.Called(firebaseUID)
	return args.Error(0)
}

func (m *MockParentService) AddChildToFamily(parentFirebaseUID string, childInfo models.Child) (string, error) {
	args := m.Called(parentFirebaseUID, childInfo)
	return args.String(0), args.Error(1)
}

func (m *MockParentService) RemoveChildFromFamily(parentFirebaseUID, childFirebaseUID string) error {
	args := m.Called(parentFirebaseUID, childFirebaseUID)
	return args.Error(0)
}

func (m *MockParentService) GetFamily(parentFirebaseUID string) ([]models.Child, error) {
	args := m.Called(parentFirebaseUID)
	return args.Get(0).([]models.Child), args.Error(1)
}

func (m *MockParentService) BlockAppsByTime(parentFirebaseUID, childFirebaseUID string, blocks []models.TimeBlock) error {
	args := m.Called(parentFirebaseUID, childFirebaseUID, blocks)
	return args.Error(0)
}

func (m *MockParentService) UnblockAppsByTime(parentFirebaseUID, childFirebaseUID string, appPackages []string) error {
	args := m.Called(parentFirebaseUID, childFirebaseUID, appPackages)
	return args.Error(0)
}

func (m *MockParentService) GetTimeBlockedApps(parentFirebaseUID, childFirebaseUID string) ([]models.TimeBlock, error) {
	args := m.Called(parentFirebaseUID, childFirebaseUID)
	return args.Get(0).([]models.TimeBlock), args.Error(1)
}

func (m *MockParentService) MonitorChildUsage(parentFirebaseUID, childFirebaseUID string) ([]models.Session, error) {
	args := m.Called(parentFirebaseUID, childFirebaseUID)
	return args.Get(0).([]models.Session), args.Error(1)
}

func (m *MockParentService) MonitorChildrenUsage(parentFirebaseUID string) (map[string][]models.Session, error) {
	args := m.Called(parentFirebaseUID)
	return args.Get(0).(map[string][]models.Session), args.Error(1)
}

// Настройка Gin для тестирования
func setupParentRouter() (*gin.Engine, *MockParentService) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	mockService := new(MockParentService)
	// Сохраняем глобальный доступ к мок-сервису
	parentService = mockService

	// Настройка маршрутов для тестов
	parents := r.Group("/parents")
	{
		parents.GET("/:firebase_uid", ReadParent)
		parents.PUT("/:firebase_uid", UpdateParent)
		parents.DELETE("/:firebase_uid", DeleteParent)
		parents.POST("/:firebase_uid/family", AddChildToFamily)
		parents.DELETE("/:firebase_uid/family/:child_id", RemoveChildFromFamily)
		parents.GET("/:firebase_uid/family", GetFamily)
		parents.POST("/:firebase_uid/block-apps/:child_id", BlockAppsByTime)
		parents.DELETE("/:firebase_uid/block-apps/:child_id", UnblockAppsByTime)
		parents.GET("/:firebase_uid/block-apps/:child_id", GetTimeBlockedApps)
	}

	parentsMonitor := r.Group("/parents/monitor")
	{
		parentsMonitor.POST("/", MonitorChildrenUsage)
		parentsMonitor.POST("/child", MonitorChildUsage)
	}

	return r, mockService
}

// Тесты для контроллера родителя

func TestReadParent(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	firebaseUID := "test-parent-uid"

	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Test Parent",
		Email:       "test@example.com",
		Phone:       "+77777777777",
	}

	// Настройка ожиданий для мока
	mockService.On("ReadParent", firebaseUID).Return(mockParent, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response models.Parent
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, mockParent.Name, response.Name)
	assert.Equal(t, mockParent.Email, response.Email)

	mockService.AssertExpectations(t)
}

func TestReadParentNotFound(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	firebaseUID := "nonexistent-uid"

	// Настройка ожиданий для мока
	mockService.On("ReadParent", firebaseUID).Return(models.Parent{}, errors.New("parent not found"))

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusNotFound, w.Code)

	mockService.AssertExpectations(t)
}

func TestUpdateParent(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	firebaseUID := "test-parent-uid"
	updateData := models.Parent{
		Name:  "Updated Name",
		Email: "updated@example.com",
	}

	updatedParent := models.Parent{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Updated Name",
		Email:       "updated@example.com",
		Phone:       "+77777777777",
	}

	// Настройка ожиданий для мока
	mockService.On("UpdateParent", firebaseUID, mock.Anything).Return(updatedParent, nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(updateData)
	req := httptest.NewRequest(http.MethodPut, "/parents/"+firebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response models.Parent
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, updatedParent.Name, response.Name)
	assert.Equal(t, updatedParent.Email, response.Email)

	mockService.AssertExpectations(t)
}

func TestDeleteParent(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	firebaseUID := "test-parent-uid"

	// Настройка ожиданий для мока
	mockService.On("DeleteParent", firebaseUID).Return(nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodDelete, "/parents/"+firebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	mockService.AssertExpectations(t)
}

func TestAddChildToFamily(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childInfo := models.Child{
		Name:     "Test Child",
		Age:      10,
		Gender:   "male",
		Birthday: "2013-01-01",
	}

	// Настройка ожиданий для мока
	mockService.On("AddChildToFamily", parentFirebaseUID, mock.Anything).Return("1234", nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(childInfo)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+parentFirebaseUID+"/family", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response struct {
		Code string `json:"code"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, "1234", response.Code)

	mockService.AssertExpectations(t)
}

func TestRemoveChildFromFamily(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	// Настройка ожиданий для мока
	mockService.On("RemoveChildFromFamily", parentFirebaseUID, childFirebaseUID).Return(nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodDelete, "/parents/"+parentFirebaseUID+"/family/"+childFirebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	mockService.AssertExpectations(t)
}

func TestGetFamily(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"

	mockFamily := []models.Child{
		{
			ID:          1,
			FirebaseUID: "child-1",
			Name:        "Child One",
			Age:         10,
			Gender:      "female",
			Birthday:    "2013-01-01",
		},
		{
			ID:          2,
			FirebaseUID: "child-2",
			Name:        "Child Two",
			Age:         8,
			Gender:      "male",
			Birthday:    "2015-05-05",
		},
	}

	// Настройка ожиданий для мока
	mockService.On("GetFamily", parentFirebaseUID).Return(mockFamily, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+parentFirebaseUID+"/family", nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response struct {
		Children []models.Child `json:"children"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Children))
	assert.Equal(t, mockFamily[0].Name, response.Children[0].Name)
	assert.Equal(t, mockFamily[1].Name, response.Children[1].Name)

	mockService.AssertExpectations(t)
}

func TestBlockAppsByTime(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	blocks := []models.TimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Настройка ожиданий для мока
	mockService.On("BlockAppsByTime", parentFirebaseUID, childFirebaseUID, mock.Anything).Return(nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(blocks)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	mockService.AssertExpectations(t)
}

func TestUnblockAppsByTime(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	appPackages := []string{"com.instagram.android", "com.facebook.katana"}

	// Настройка ожиданий для мока
	mockService.On("UnblockAppsByTime", parentFirebaseUID, childFirebaseUID, appPackages).Return(nil)

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(appPackages)
	req := httptest.NewRequest(http.MethodDelete, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	mockService.AssertExpectations(t)
}

func TestGetTimeBlockedApps(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	blocks := []models.TimeBlock{
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

	// Настройка ожиданий для мока
	mockService.On("GetTimeBlockedApps", parentFirebaseUID, childFirebaseUID).Return(blocks, nil)

	// Создаем тестовый запрос
	req := httptest.NewRequest(http.MethodGet, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response struct {
		Blocks []models.TimeBlock `json:"blocks"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Blocks))
	assert.Equal(t, blocks[0].AppPackage, response.Blocks[0].AppPackage)
	assert.Equal(t, blocks[1].AppPackage, response.Blocks[1].AppPackage)

	mockService.AssertExpectations(t)
}

func TestMonitorChildUsage(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	mockUsageSessions := []models.Session{
		{
			AppPackage:  "com.instagram.android",
			StartTime:   "2023-05-14T13:00:00Z",
			EndTime:     "2023-05-14T14:00:00Z",
			DurationMin: 60,
		},
		{
			AppPackage:  "com.facebook.katana",
			StartTime:   "2023-05-14T15:00:00Z",
			EndTime:     "2023-05-14T15:30:00Z",
			DurationMin: 30,
		},
	}

	// Настройка ожиданий для мока
	mockService.On("MonitorChildUsage", parentFirebaseUID, childFirebaseUID).Return(mockUsageSessions, nil)

	// Создаем данные для запроса
	requestData := struct {
		ParentFirebaseUID string `json:"parent_firebase_uid"`
		ChildFirebaseUID  string `json:"child_firebase_uid"`
	}{
		ParentFirebaseUID: parentFirebaseUID,
		ChildFirebaseUID:  childFirebaseUID,
	}

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(requestData)
	req := httptest.NewRequest(http.MethodPost, "/parents/monitor/child", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response struct {
		Sessions []models.Session `json:"sessions"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.Sessions))
	assert.Equal(t, mockUsageSessions[0].AppPackage, response.Sessions[0].AppPackage)
	assert.Equal(t, mockUsageSessions[1].AppPackage, response.Sessions[1].AppPackage)

	mockService.AssertExpectations(t)
}

func TestMonitorChildrenUsage(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"

	mockUsageData := map[string][]models.Session{
		"child-1": {
			{
				AppPackage:  "com.instagram.android",
				StartTime:   "2023-05-14T13:00:00Z",
				EndTime:     "2023-05-14T14:00:00Z",
				DurationMin: 60,
			},
		},
		"child-2": {
			{
				AppPackage:  "com.facebook.katana",
				StartTime:   "2023-05-14T15:00:00Z",
				EndTime:     "2023-05-14T15:45:00Z",
				DurationMin: 45,
			},
		},
	}

	// Настройка ожиданий для мока
	mockService.On("MonitorChildrenUsage", parentFirebaseUID).Return(mockUsageData, nil)

	// Создаем данные для запроса
	requestData := struct {
		ParentFirebaseUID string `json:"parent_firebase_uid"`
	}{
		ParentFirebaseUID: parentFirebaseUID,
	}

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(requestData)
	req := httptest.NewRequest(http.MethodPost, "/parents/monitor/", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	// Проверяем тело ответа
	var response struct {
		UsageData map[string][]models.Session `json:"usage_data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(response.UsageData))
	assert.Equal(t, 1, len(response.UsageData["child-1"]))
	assert.Equal(t, 1, len(response.UsageData["child-2"]))
	assert.Equal(t, mockUsageData["child-1"][0].AppPackage, response.UsageData["child-1"][0].AppPackage)
	assert.Equal(t, mockUsageData["child-2"][0].AppPackage, response.UsageData["child-2"][0].AppPackage)

	mockService.AssertExpectations(t)
}

// Тесты для обработки ошибок

func TestAddChildToFamilyError(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childInfo := models.Child{
		Name:     "Test Child",
		Age:      10,
		Gender:   "male",
		Birthday: "2013-01-01",
	}

	// Настройка ожиданий для мока
	mockService.On("AddChildToFamily", parentFirebaseUID, mock.Anything).Return("", errors.New("failed to add child"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(childInfo)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+parentFirebaseUID+"/family", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(t)
}

func TestBlockAppsByTimeError(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	blocks := []models.TimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Настройка ожиданий для мока
	mockService.On("BlockAppsByTime", parentFirebaseUID, childFirebaseUID, mock.Anything).
		Return(errors.New("failed to block apps"))

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(blocks)
	req := httptest.NewRequest(http.MethodPost, "/parents/"+parentFirebaseUID+"/block-apps/"+childFirebaseUID, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(t)
}

func TestMonitorChildUsageError(t *testing.T) {
	r, mockService := setupParentRouter()

	// Тестовые данные
	parentFirebaseUID := "test-parent-uid"
	childFirebaseUID := "test-child-uid"

	// Настройка ожиданий для мока
	mockService.On("MonitorChildUsage", parentFirebaseUID, childFirebaseUID).
		Return([]models.Session{}, errors.New("child not found"))

	// Создаем данные для запроса
	requestData := struct {
		ParentFirebaseUID string `json:"parent_firebase_uid"`
		ChildFirebaseUID  string `json:"child_firebase_uid"`
	}{
		ParentFirebaseUID: parentFirebaseUID,
		ChildFirebaseUID:  childFirebaseUID,
	}

	// Создаем тестовый запрос
	requestBody, _ := json.Marshal(requestData)
	req := httptest.NewRequest(http.MethodPost, "/parents/monitor/child", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Выполняем запрос
	r.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockService.AssertExpectations(t)
}
