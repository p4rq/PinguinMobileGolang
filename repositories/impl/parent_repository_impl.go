package impl

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"fmt"

	"gorm.io/gorm"
)

type ParentRepositoryImpl struct {
	DB *gorm.DB
}

func NewParentRepository(db *gorm.DB) repositories.ParentRepository {
	return &ParentRepositoryImpl{DB: db}
}

func (r *ParentRepositoryImpl) FindByFirebaseUID(firebaseUID string) (models.Parent, error) {
	var parent models.Parent
	if err := r.DB.Where("firebase_uid = ?", firebaseUID).First(&parent).Error; err != nil {
		return models.Parent{}, err
	}
	return parent, nil
}

func (r *ParentRepositoryImpl) FindByEmail(email string) (models.Parent, error) {
	var parent models.Parent
	if err := r.DB.Where("email = ?", email).First(&parent).Error; err != nil {
		return models.Parent{}, err
	}
	return parent, nil
}

func (r *ParentRepositoryImpl) FindByCode(code string) (models.Parent, error) {
	var parent models.Parent
	if err := r.DB.Where("code = ?", code).First(&parent).Error; err != nil {
		return models.Parent{}, err
	}
	return parent, nil
}

func (r *ParentRepositoryImpl) CountByCode(code string, count *int64) error {
	return r.DB.Model(&models.Parent{}).Where("code = ?", code).Count(count).Error
}

func (r *ParentRepositoryImpl) Save(parent models.Parent) error {
	return r.DB.Save(&parent).Error
}

func (r *ParentRepositoryImpl) DeleteByFirebaseUID(firebaseUID string) error {
	return r.DB.Where("firebase_uid = ?", firebaseUID).Delete(&models.Parent{}).Error
}
func (r *ParentRepositoryImpl) Delete(id uint) error {
	// Проверяем, существует ли запись
	var parent models.Parent
	if err := r.DB.First(&parent, id).Error; err != nil {
		return fmt.Errorf("parent not found: %w", err)
	}

	// Выполняем удаление
	if err := r.DB.Delete(&parent).Error; err != nil {
		return fmt.Errorf("failed to delete parent: %w", err)
	}

	return nil
}
