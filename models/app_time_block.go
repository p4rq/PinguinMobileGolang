package models

// AppTimeBlock представляет собой структуру для хранения информации о временной блокировке приложения
type AppTimeBlock struct {
	AppPackage string `json:"app_package"`  // Идентификатор приложения
	StartTime  string `json:"start_time"`   // Время начала блокировки (формат "HH:MM")
	EndTime    string `json:"end_time"`     // Время окончания блокировки (формат "HH:MM")
	DaysOfWeek string `json:"days_of_week"` // Дни недели для блокировки (формат "1,2,3,4,5,6,7", где 1 - понедельник)
}
