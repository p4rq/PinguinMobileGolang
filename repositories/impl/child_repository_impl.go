package impl

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"

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

func (r *ChildRepositoryImpl) Save(child models.Child) error {
	return r.DB.Save(&child).Error
}
func (r *ChildRepositoryImpl) Delete(child models.Child) error {
	return r.DB.Delete(&child).Error
}
