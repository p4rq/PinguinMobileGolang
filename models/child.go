package models

type Child struct {
	ID              uint   `json:"id" gorm:"primary_key"`
	Role            string `json:"role"`
	Lang            string `json:"lang"`
	Name            string `json:"name"`
	Family          string `json:"family"`
	FirebaseUID     string `json:"firebase_uid"`
	IsBinded        bool   `json:"is_binded"`
	UsageData       string `json:"usage_data"`
	Gender          string `json:"gender"`
	Age             int    `json:"age"`
	Birthday        string `json:"birthday"`
	Code            string `json:"code"`
	BlockedApps     string `json:"blocked_apps"` // Новое поле для хранения заблокированных приложений
	TimeBlockedApps string // Новое поле для хранения временных блокировок в формате JSON

}
