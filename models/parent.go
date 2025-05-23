package models

import "time"

type Parent struct {
	ID                 uint       `json:"id" gorm:"primary_key"`
	Lang               string     `json:"lang"`
	Name               string     `json:"name"`
	Family             string     `json:"family"`
	Email              string     `json:"email"`
	Password           string     `json:"-"`
	FirebaseUID        string     `json:"firebase_uid"`
	Role               string     `json:"role"`
	Code               string     `json:"code" gorm:"size:4"`
	CodeExpiresAt      *time.Time `json:"code_expires_at"`
	EmailVerified      bool       `json:"email_verified"`
	VerificationCode   string     `json:"-" gorm:"column:verification_code"`
	EmailCodeExpiresAt time.Time  `json:"-" gorm:"column:email_code_expires_at"`

	PasswordResetCode          string    `json:"-" gorm:"size:10"`
	PasswordResetCodeExpiresAt time.Time `json:"-"`
}

func (p *Parent) IsCodeValid() bool {
	return p.Code != "" && p.CodeExpiresAt != nil && time.Now().Before(*p.CodeExpiresAt)
}

// RefreshCode обновляет код привязки со сроком действия 24 часа
func (p *Parent) RefreshCode(code string) {
	p.Code = code
	expiresAt := time.Now().Add(24 * time.Hour)
	p.CodeExpiresAt = &expiresAt
}
