package models

import "time"

// AppTimeBlock представляет собой структуру для хранения информации о временной блокировке приложения
type AppTimeBlock struct {
	ID           int64     `json:"id" gorm:"primaryKey"`
	AppPackage   string    `json:"app_package"`
	StartTime    string    `json:"start_time"`
	EndTime      string    `json:"end_time"`
	DaysOfWeek   string    `json:"days_of_week"`
	IsOneTime    bool      `json:"is_one_time"`
	OneTimeEndAt time.Time `json:"one_time_end_at,omitempty"`
	Duration     string    `json:"duration,omitempty"`
	BlockName    string    `json:"block_name,omitempty"`
	IsPermanent  bool      `json:"is_permanent,omitempty"` // Добавленное поле

}

// TempBlockRequest представляет запрос на временную одноразовую блокировку
type TempBlockRequest struct {
	ChildFirebaseUID string   `json:"child_firebase_uid" binding:"required"`
	AppPackages      []string `json:"app_packages" binding:"required"`
	DurationMins     int      `json:"duration_mins" binding:"required"` // Потенциальная проблема
	BlockName        string   `json:"block_name,omitempty"`
}
