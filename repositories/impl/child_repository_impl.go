package impl

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"

	"gorm.io/gorm"
)

type ChildRepositoryImpl struct {
	DB *gorm.DB
}

func NewChildRepository(db *gorm.DB) repositories.ChildRepository {
	return &ChildRepositoryImpl{DB: db}
}

func (r *ChildRepositoryImpl) FindByFirebaseUID(firebaseUID string) (models.Child, error) {
	var child models.Child
	if err := r.DB.Where("firebase_uid = ?", firebaseUID).First(&child).Error; err != nil {
		return models.Child{}, err
	}
	return child, nil
}

func (r *ChildRepositoryImpl) FindByCode(code string) (models.Child, error) {
	var child models.Child
	if err := r.DB.Where("code = ?", code).First(&child).Error; err != nil {
		return models.Child{}, err
	}
	return child, nil
}

func (r *ChildRepositoryImpl) CountByCode(code string, count *int64) error {
	return r.DB.Model(&models.Child{}).Where("code = ?", code).Count(count).Error
}

func (r *ChildRepositoryImpl) Save(child models.Child) error {
	return r.DB.Save(&child).Error
}

func (r *ChildRepositoryImpl) Delete(child models.Child) error {
	return r.DB.Delete(&child).Error
}

// AddTimeBlockedApps добавляет временные блокировки для приложений
func (r *ChildRepositoryImpl) AddTimeBlockedApps(childID uint, timeBlocks []models.AppTimeBlock) error {
	var child models.Child
	if err := r.DB.First(&child, childID).Error; err != nil {
		return err
	}

	// Десериализуем существующие блокировки
	var existingBlocks []models.AppTimeBlock
	if child.TimeBlockedApps != "" {
		if err := json.Unmarshal([]byte(child.TimeBlockedApps), &existingBlocks); err != nil {
			return err
		}
	}

	// Создаем карту существующих блокировок для быстрого поиска
	existingMap := make(map[string]bool)
	for _, block := range existingBlocks {
		existingMap[block.AppPackage] = true
	}

	// Обновляем существующие блокировки или добавляем новые
	var updatedBlocks []models.AppTimeBlock

	// Сначала добавим все существующие блокировки, которые не будут обновлены
	for _, block := range existingBlocks {
		shouldInclude := true
		for _, newBlock := range timeBlocks {
			if block.AppPackage == newBlock.AppPackage {
				shouldInclude = false
				break
			}
		}
		if shouldInclude {
			updatedBlocks = append(updatedBlocks, block)
		}
	}

	// Затем добавим все новые блокировки
	updatedBlocks = append(updatedBlocks, timeBlocks...)

	// Сериализуем обратно в JSON
	blocksJSON, err := json.Marshal(updatedBlocks)
	if err != nil {
		return err
	}

	// Обновляем запись в базе данных
	return r.DB.Model(&child).Update("time_blocked_apps", string(blocksJSON)).Error
}

// RemoveTimeBlockedApps удаляет временные блокировки для указанных приложений
func (r *ChildRepositoryImpl) RemoveTimeBlockedApps(childID uint, appPackages []string) error {
	var child models.Child
	if err := r.DB.First(&child, childID).Error; err != nil {
		return err
	}

	// Если нет временных блокировок, нечего удалять
	if child.TimeBlockedApps == "" {
		return nil
	}

	// Десериализуем существующие блокировки
	var existingBlocks []models.AppTimeBlock
	if err := json.Unmarshal([]byte(child.TimeBlockedApps), &existingBlocks); err != nil {
		return err
	}

	// Создаем карту приложений для удаления
	toRemove := make(map[string]bool)
	for _, pkg := range appPackages {
		toRemove[pkg] = true
	}

	// Отфильтровываем блокировки для удаления
	var updatedBlocks []models.AppTimeBlock
	for _, block := range existingBlocks {
		if !toRemove[block.AppPackage] {
			updatedBlocks = append(updatedBlocks, block)
		}
	}

	// Сериализуем обратно в JSON
	blocksJSON, err := json.Marshal(updatedBlocks)
	if err != nil {
		return err
	}

	// Обновляем запись в базе данных
	return r.DB.Model(&child).Update("time_blocked_apps", string(blocksJSON)).Error
}

// GetTimeBlockedApps возвращает список временных блокировок для ребенка
func (r *ChildRepositoryImpl) GetTimeBlockedApps(childID uint) ([]models.AppTimeBlock, error) {
	var child models.Child
	if err := r.DB.First(&child, childID).Error; err != nil {
		return nil, err
	}

	// Если нет временных блокировок, возвращаем пустой список
	if child.TimeBlockedApps == "" {
		return []models.AppTimeBlock{}, nil
	}

	// Десериализуем и возвращаем блокировки
	var blocks []models.AppTimeBlock
	if err := json.Unmarshal([]byte(child.TimeBlockedApps), &blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}
