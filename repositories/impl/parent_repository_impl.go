package impl

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"

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

func (r *ParentRepositoryImpl) Save(parent models.Parent) error {
	return r.DB.Save(&parent).Error
}

func (r *ParentRepositoryImpl) DeleteByFirebaseUID(firebaseUID string) error {
	return r.DB.Where("firebase_uid = ?", firebaseUID).Delete(&models.Parent{}).Error
}
