package models

import "time"

type Session struct {
	App       string    `json:"app"`
	Duration  int       `json:"duration"`
	Timestamp time.Time `json:"timestamp"`
}
