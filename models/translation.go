package models

import "time"

type Translation struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Key           string    `json:"key" gorm:"unique"`
	Russian       string    `json:"ru"`
	English       string    `json:"en"`
	Kazakh        string    `json:"kz"`
	LastUpdatedAt time.Time `json:"last_updated_at" gorm:"autoUpdateTime"` // Добавляем поле для отслеживания времени обновления
}
