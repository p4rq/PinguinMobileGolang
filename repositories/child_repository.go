package repositories

import "PinguinMobile/models"

type ChildRepository interface {
	FindByFirebaseUID(firebaseUID string) (models.Child, error)
	FindByCode(code string) (models.Child, error)
	CountByCode(code string, count *int64) error
	Save(child models.Child) error
	Delete(child models.Child) error

	// Новые методы для работы с временной блокировкой
	AddTimeBlockedApps(childID uint, timeBlocks []models.AppTimeBlock) error
	RemoveTimeBlockedApps(childID uint, appPackages []string) error
	GetTimeBlockedApps(childID uint) ([]models.AppTimeBlock, error)
}
