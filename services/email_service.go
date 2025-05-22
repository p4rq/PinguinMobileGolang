package services

import (
	"fmt"
	"math/rand"
	"net/smtp"
	"os"
	"time"
)

type EmailService struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
}

// NewEmailService создает новый экземпляр EmailService
func NewEmailService() *EmailService {
	return &EmailService{
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     os.Getenv("SMTP_PORT"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		FromEmail:    os.Getenv("FROM_EMAIL"),
	}
}

// GenerateVerificationCode создает 6-значный код проверки
func GenerateVerificationCode() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// SendVerificationEmail отправляет код верификации на email
func (s *EmailService) SendVerificationEmail(email, code string) error {
	subject := "Подтверждение Email в Pinguin Mobile"
	body := fmt.Sprintf(`
Здравствуйте!

Благодарим вас за регистрацию в приложении Pinguin Mobile.
Для подтверждения вашего email-адреса, пожалуйста, введите следующий код в приложении:

%s

Код действителен в течение 24 часов.

С уважением,
Команда Pinguin Mobile
`, code)

	message := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s", s.FromEmail, email, subject, body)

	auth := smtp.PlainAuth("", s.SMTPUsername, s.SMTPPassword, s.SMTPHost)
	addr := fmt.Sprintf("%s:%s", s.SMTPHost, s.SMTPPort)

	return smtp.SendMail(addr, auth, s.FromEmail, []string{email}, []byte(message))
}
