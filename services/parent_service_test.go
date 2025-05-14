package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories/mocks"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBlockAppsByTimeWithEmptyDaysOfWeek(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		ID:          2,
		FirebaseUID: childFirebaseUID,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)
	mockChildRepo.On("AddTimeBlockedApps", uint(2), mock.MatchedBy(func(blocks []models.AppTimeBlock) bool {
		// Проверяем, что дни недели заполнены значением по умолчанию
		return len(blocks) == 1 && blocks[0].DaysOfWeek == "1,2,3,4,5,6,7"
	})).Return(nil)

	// Тестовые блокировки с пустым полем дней недели
	blocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "", // Пустое значение должно замениться на все дни недели
		},
	}

	// Вызываем тестируемый метод
	err := parentService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, blocks)

	// Проверяем результат
	assert.NoError(t, err)
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestBlockAppsByTimeChildNotInFamily(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON (другой ребенок)
	familyJSON := `[{"firebase_uid":"another_child_uid"}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		ID:          2,
		FirebaseUID: childFirebaseUID,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Тестовые данные для блокировки
	blocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Вызываем тестируемый метод
	err := parentService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, blocks)

	// Проверяем результат - должна быть ошибка, так как ребенок не в семье родителя
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "child does not belong to this parent")
}

func TestMonitorChildUsage(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "child1"

	// Данные использования для ребенка
	childUsageJSON := `{"screen_time":120, "apps":[{"package":"com.example.app1", "time":60}]}`

	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
	}

	mockChild := models.Child{
		ID:          2,
		FirebaseUID: childFirebaseUID,
		Name:        "Test Child",
		UsageData:   childUsageJSON,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Вызываем тестируемый метод
	usageData, err := parentService.MonitorChildUsage(parentFirebaseUID, childFirebaseUID)

	// Проверяем результат
	assert.NoError(t, err)
	assert.Equal(t, childFirebaseUID, usageData["child_id"])
	assert.Equal(t, "Test Child", usageData["name"])

	// Проверяем данные использования
	childUsage := usageData["usage_data"].(map[string]interface{})
	assert.Equal(t, float64(120), childUsage["screen_time"])
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestMonitorChildUsageParentNotFound(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "nonexistent_parent"
	childFirebaseUID := "child1"

	// Настраиваем ожидания с ошибкой
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(models.Parent{}, errors.New("parent not found"))

	// Вызываем тестируемый метод
	usageData, err := parentService.MonitorChildUsage(parentFirebaseUID, childFirebaseUID)

	// Проверяем результат
	assert.Error(t, err)
	assert.Nil(t, usageData)
	mockParentRepo.AssertExpectations(t)
}

func TestMonitorChildUsageChildNotFound(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "nonexistent_child"

	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(models.Child{}, errors.New("child not found"))

	// Вызываем тестируемый метод
	usageData, err := parentService.MonitorChildUsage(parentFirebaseUID, childFirebaseUID)

	// Проверяем результат
	assert.Error(t, err)
	assert.Nil(t, usageData)
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestBlockAppsParentNotFound(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "nonexistent_parent"
	childFirebaseUID := "child1"

	// Приложения для блокировки
	appsToBlock := []string{
		"com.instagram.android",
		"com.facebook.katana",
	}

	// Настраиваем ожидания с ошибкой
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(models.Parent{}, errors.New("parent not found"))

	// Вызываем тестируемый метод
	err := parentService.BlockApps(parentFirebaseUID, childFirebaseUID, appsToBlock)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "parent not found", err.Error())
	mockParentRepo.AssertExpectations(t)
}

func TestBlockAppsChildNotFound(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "nonexistent_child"

	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
	}

	// Приложения для блокировки
	appsToBlock := []string{
		"com.instagram.android",
		"com.facebook.katana",
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(models.Child{}, errors.New("child not found"))

	// Вызываем тестируемый метод
	err := parentService.BlockApps(parentFirebaseUID, childFirebaseUID, appsToBlock)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "child not found", err.Error())
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestUpdateChild(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	childToUpdate := models.Child{
		ID:          2,
		FirebaseUID: "child1",
		Name:        "Updated Child Name",
		Code:        "5678",
	}

	// Настраиваем ожидания
	mockChildRepo.On("Save", childToUpdate).Return(nil)

	// Вызываем тестируемый метод
	err := parentService.UpdateChild(childToUpdate)

	// Проверяем результат
	assert.NoError(t, err)
	mockChildRepo.AssertExpectations(t)
}

func TestUpdateChildError(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	childToUpdate := models.Child{
		ID:          2,
		FirebaseUID: "child1",
		Name:        "Updated Child Name",
		Code:        "5678",
	}

	// Настраиваем ожидания с ошибкой
	mockChildRepo.On("Save", childToUpdate).Return(errors.New("database error"))

	// Вызываем тестируемый метод
	err := parentService.UpdateChild(childToUpdate)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	mockChildRepo.AssertExpectations(t)
}

func TestBlockAppsByTimeWithParentNotFound(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "nonexistent_parent"
	childFirebaseUID := "child1"

	// Настраиваем ожидания с ошибкой
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(models.Parent{}, errors.New("parent not found"))

	// Тестовые данные для блокировки
	blocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Вызываем тестируемый метод
	err := parentService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, blocks)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "parent not found", err.Error())
	mockParentRepo.AssertExpectations(t)
}

func TestUnblockAppsByTimeWithError(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		ID:          2,
		FirebaseUID: childFirebaseUID,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Ошибка при удалении
	mockChildRepo.On("RemoveTimeBlockedApps", uint(2), []string{"com.instagram.android"}).Return(errors.New("database error"))

	// Тестовые данные для разблокировки
	apps := []string{"com.instagram.android"}

	// Вызываем тестируемый метод
	err := parentService.UnblockAppsByTime(parentFirebaseUID, childFirebaseUID, apps)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestUpdateParentError(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	firebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"

	// Настраиваем ожидания с ошибкой
	mockParentRepo.On("FindByFirebaseUID", firebaseUID).Return(models.Parent{}, errors.New("parent not found"))

	// Вызываем тестируемый метод
	updatedParent, err := parentService.UpdateParent(firebaseUID, models.Parent{})

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, models.Parent{}, updatedParent)
	mockParentRepo.AssertExpectations(t)
}

func TestUpdateParentSaveError(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	firebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"

	existingParent := models.Parent{
		ID:          1,
		FirebaseUID: firebaseUID,
		Name:        "Old Name",
		Email:       "old@test.com",
	}

	updateInput := models.Parent{
		Name:  "New Name",
		Email: "new@test.com",
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", firebaseUID).Return(existingParent, nil)
	mockParentRepo.On("Save", mock.Anything).Return(errors.New("save error"))

	// Вызываем тестируемый метод
	updatedParent, err := parentService.UpdateParent(firebaseUID, updateInput)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "save error", err.Error())
	assert.Equal(t, models.Parent{}, updatedParent)
	mockParentRepo.AssertExpectations(t)
}

func TestDeleteParentError(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	firebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"

	// Настраиваем ожидания с ошибкой
	mockParentRepo.On("DeleteByFirebaseUID", firebaseUID).Return(errors.New("delete error"))

	// Вызываем тестируемый метод
	err := parentService.DeleteParent(firebaseUID)

	// Проверяем результат
	assert.Error(t, err)
	assert.Equal(t, "delete error", err.Error())
	mockParentRepo.AssertExpectations(t)
}

func TestBlockAppsByTimeSuccess(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		ID:              2,
		FirebaseUID:     childFirebaseUID,
		TimeBlockedApps: "",
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Ожидание для добавления блокировок
	mockChildRepo.On("AddTimeBlockedApps", uint(2), mock.AnythingOfType("[]models.AppTimeBlock")).Return(nil)

	// Тестовые данные для блокировки
	blocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Вызываем тестируемый метод
	err := parentService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, blocks)

	// Проверяем результат
	assert.NoError(t, err)
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestUnblockAppsByTime(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"age":0,"birthday":"","child_id":0,"code":"3537","firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1","gender":"","isBinded":true,"lang":"kz","name":"","usage_data":""}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		ID:              2,
		FirebaseUID:     childFirebaseUID,
		TimeBlockedApps: `[{"app_package":"com.instagram.android","start_time":"13:00","end_time":"18:00","days_of_week":"1,2,3,4,5"}]`,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Ожидание для удаления блокировок
	mockChildRepo.On("RemoveTimeBlockedApps", uint(2), []string{"com.instagram.android"}).Return(nil)

	// Тестовые данные для разблокировки
	apps := []string{"com.instagram.android"}

	// Вызываем тестируемый метод
	err := parentService.UnblockAppsByTime(parentFirebaseUID, childFirebaseUID, apps)

	// Проверяем результат
	assert.NoError(t, err)
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestGetTimeBlockedApps(t *testing.T) {
	// Создаем моки для репозиториев
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис с моками
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"age":0,"birthday":"","child_id":0,"code":"3537","firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1","gender":"","isBinded":true,"lang":"kz","name":"","usage_data":""}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		ID:          1,
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		ID:          2,
		FirebaseUID: childFirebaseUID,
	}

	// Ожидаемые блокировки
	expectedBlocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)
	mockChildRepo.On("GetTimeBlockedApps", uint(2)).Return(expectedBlocks, nil)

	// Вызываем тестируемый метод
	blocks, err := parentService.GetTimeBlockedApps(parentFirebaseUID, childFirebaseUID)

	// Проверяем результат
	assert.NoError(t, err)
	assert.Equal(t, expectedBlocks, blocks)
	mockParentRepo.AssertExpectations(t)
	mockChildRepo.AssertExpectations(t)
}

func TestIsChildInFamily(t *testing.T) {
	// Создаем моки
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые случаи
	testCases := []struct {
		name             string
		familyJSON       string
		childFirebaseUID string
		expected         bool
	}{
		{
			name:             "Ребенок найден в семье",
			familyJSON:       `[{"firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"}]`,
			childFirebaseUID: "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1",
			expected:         true,
		},
		{
			name:             "Ребенок не найден в семье",
			familyJSON:       `[{"firebase_uid":"other_child_id"}]`,
			childFirebaseUID: "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1",
			expected:         false,
		},
		{
			name:             "Пустая семья",
			familyJSON:       `[]`,
			childFirebaseUID: "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1",
			expected:         false,
		},
		{
			name:             "Невалидный JSON",
			familyJSON:       `invalid json`,
			childFirebaseUID: "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1",
			expected:         false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parent := models.Parent{
				Family: tc.familyJSON,
			}

			result := parentService.isChildInFamily(parent, tc.childFirebaseUID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBlockAppsByTimeInvalidTimes(t *testing.T) {
	// Создаем моки
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		FirebaseUID: childFirebaseUID,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Тестовые блокировки с невалидным временем
	invalidBlocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "25:00", // Невалидное время
			EndTime:    "18:00",
			DaysOfWeek: "1,2,3,4,5",
		},
	}

	// Вызываем тестируемый метод
	err := parentService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, invalidBlocks)

	// Должна быть ошибка из-за невалидного времени
	assert.Error(t, err)
}

func TestBlockAppsByTimeInvalidDaysOfWeek(t *testing.T) {
	// Создаем моки
	mockParentRepo := new(mocks.ParentRepository)
	mockChildRepo := new(mocks.ChildRepository)

	// Создаем сервис
	parentService := NewParentService(mockParentRepo, mockChildRepo)

	// Тестовые данные
	parentFirebaseUID := "ZEXF4HEyySaGUVUFzUifUsF6rLi2"
	childFirebaseUID := "OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"

	// Тестовое семейство в JSON
	familyJSON := `[{"firebase_uid":"OeLYNPOdTkVhnKihw8Pqns1Q6Ml1"}]`

	// Создаем моки для сущностей
	mockParent := models.Parent{
		FirebaseUID: parentFirebaseUID,
		Family:      familyJSON,
	}

	mockChild := models.Child{
		FirebaseUID: childFirebaseUID,
	}

	// Настраиваем ожидания
	mockParentRepo.On("FindByFirebaseUID", parentFirebaseUID).Return(mockParent, nil)
	mockChildRepo.On("FindByFirebaseUID", childFirebaseUID).Return(mockChild, nil)

	// Тестовые блокировки с невалидными днями недели
	invalidBlocks := []models.AppTimeBlock{
		{
			AppPackage: "com.instagram.android",
			StartTime:  "13:00",
			EndTime:    "18:00",
			DaysOfWeek: "1,8,9", // Невалидные дни недели (8 и 9)
		},
	}

	// Вызываем тестируемый метод
	err := parentService.BlockAppsByTime(parentFirebaseUID, childFirebaseUID, invalidBlocks)

	// Должна быть ошибка из-за невалидных дней недели
	assert.Error(t, err)
}
