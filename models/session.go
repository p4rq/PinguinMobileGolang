package models

import "time"

type Session struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Activity  string    `json:"activity"`
}
