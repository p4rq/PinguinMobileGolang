package models

type Translation struct {
	ID      uint   `json:"id" gorm:"primaryKey"`
	Key     string `json:"key" gorm:"unique"`
	Russian string `json:"ru"`
	English string `json:"en"`
	Kazakh  string `json:"kz"`
}
