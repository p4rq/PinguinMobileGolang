package repositories

import "PinguinMobile/models"

type ParentRepository interface {
	FindByFirebaseUID(firebaseUID string) (models.Parent, error)
	Save(parent models.Parent) error
	DeleteByFirebaseUID(firebaseUID string) error
}
