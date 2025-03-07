package repositories

import "PinguinMobile/models"

type ParentRepository interface {
	FindByFirebaseUID(firebaseUID string) (models.Parent, error)
	FindByEmail(email string) (models.Parent, error)
	FindByCode(code string) (models.Parent, error)
	CountByCode(code string, count *int64) error
	Save(parent models.Parent) error
	DeleteByFirebaseUID(firebaseUID string) error
}
