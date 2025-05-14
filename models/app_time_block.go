package models

import "time"

// AppTimeBlock представляет собой структуру для хранения информации о временной блокировке приложения
type AppTimeBlock struct {
	AppPackage   string    `json:"app_package"`               // Идентификатор приложения
	StartTime    string    `json:"start_time"`                // Время начала блокировки (формат "HH:MM")
	EndTime      string    `json:"end_time"`                  // Время окончания блокировки (формат "HH:MM")
	DaysOfWeek   string    `json:"days_of_week"`              // Дни недели для блокировки (формат "1,2,3,4,5,6,7", где 1 - понедельник)
	IsOneTime    bool      `json:"is_one_time,omitempty"`     // Флаг для одноразовой блокировки
	OneTimeEndAt time.Time `json:"one_time_end_at,omitempty"` // Время окончания одноразовой блокировки
	Duration     string    `json:"duration,omitempty"`        // Текстовое представление продолжительности
}

// TempBlockRequest представляет запрос на временную одноразовую блокировку
type TempBlockRequest struct {
	ChildFirebaseUID string   `json:"child_firebase_uid" binding:"required"`
	AppPackages      []string `json:"app_packages" binding:"required"`
	DurationHours    float64  `json:"duration_hours" binding:"required,min=0.5,max=24"`
}
