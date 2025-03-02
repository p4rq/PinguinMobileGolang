package models

type Parent struct {
	ID          uint   `json:"id" gorm:"primary_key"`
	Lang        string `json:"lang"`
	Name        string `json:"name"`
	Family      string `json:"family"`
	Email       string `json:"email"`
	Password    string `json:"-"`
	FirebaseUID string `json:"firebase_uid"`
	Role        string `json:"role"`
	Code        string `json:"code"`
	FamilyCode  string `json:"family_code"`
}
