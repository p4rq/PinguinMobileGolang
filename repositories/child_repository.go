package repositories

import "PinguinMobile/models"

type ChildRepository interface {
	FindByFirebaseUID(firebaseUID string) (models.Child, error)
	FindByCode(code string) (models.Child, error)
	Save(child models.Child) error
	Delete(child models.Child) error
}
