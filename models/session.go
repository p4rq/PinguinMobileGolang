package models

type Session struct {
	App       string  `json:"app"`
	Duration  float64 `json:"duration"`
	Timestamp string  `json:"timestamp"`
}
