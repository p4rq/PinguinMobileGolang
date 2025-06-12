package services

import (
	"PinguinMobile/models"
	"PinguinMobile/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Заменяем определение глобальной переменной jwtKey в начале файла
var jwtKey []byte

func init() {
	// Инициализируем генератор случайных чисел
	rand.Seed(time.Now().UnixNano())

	// Устанавливаем глобальный секретный ключ для JWT
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		secretKey = "your_secret_key" // Дефолтный ключ, если переменная окружения не установлена
	}
	jwtKey = []byte(secretKey)
}

type Claims struct {
	Email       string `json:"email"`
	FirebaseUID string `json:"firebase_uid"` // Переименуем в UserID в будущем
	UserType    string `json:"user_type"`
	jwt.StandardClaims
}

type AuthService struct {
	ParentRepo repositories.ParentRepository
	ChildRepo  repositories.ChildRepository
	DB         *gorm.DB
}

func NewAuthService(parentRepo repositories.ParentRepository, childRepo repositories.ChildRepository) *AuthService {
	return &AuthService{ParentRepo: parentRepo, ChildRepo: childRepo}
}

func (s *AuthService) RegisterParent(lang, name, email, password string) (models.Parent, string, error) {
	existingParent, err := s.ParentRepo.FindByEmail(email)
	if err == nil && existingParent.ID != 0 {
		return models.Parent{}, "", fmt.Errorf("email already exists")
	}
	if password == "" {
		return models.Parent{}, "", errors.New("password cannot be empty")
	}

	// Генерируем уникальный идентификатор пользователя (вместо Firebase UID)
	userID := uuid.New().String()

	// Generate unique 4-digit code
	var code string
	for {
		code = fmt.Sprintf("%04d", 1000+rand.Intn(9000)) // Гарантируем формат 4 цифр
		var count int64
		err := s.ParentRepo.CountByCode(code, &count)
		if err != nil {
			return models.Parent{}, "", err
		}
		if count == 0 {
			break
		}
	}

	// Create user in local database
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.Parent{}, "", err
	}

	// Устанавливаем срок действия кода - 24 часа
	codeExpiresAt := time.Now().Add(24 * time.Hour)

	parent := models.Parent{
		Lang:          lang,
		Name:          name,
		Email:         email,
		Password:      string(hashedPassword),
		Role:          "parent",
		Family:        "[]",
		Code:          code,
		CodeExpiresAt: &codeExpiresAt, // Добавляем срок действия кода
		FirebaseUID:   userID,         // Используем UUID вместо Firebase UID
		EmailVerified: false,          // Email изначально не подтвержден
	}

	if err := s.ParentRepo.Save(parent); err != nil {
		return models.Parent{}, "", err
	}

	// Generate JWT token with additional fields
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email:       email,
		FirebaseUID: userID,   // Используем userID вместо Firebase UID
		UserType:    "parent", // Указываем тип пользователя
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return models.Parent{}, "", err
	}

	return parent, tokenString, nil
}

func (s *AuthService) LoginParent(email, password string) (models.Parent, string, error) {
	parent, err := s.ParentRepo.FindByEmail(email)
	if err != nil {
		return models.Parent{}, "", err
	}

	fmt.Printf("Stored hashed password: %s\n", parent.Password)
	fmt.Printf("Provided password: %s\n", password)

	// Проверка длины пароля
	fmt.Printf("Length of provided password: %d\n", len(password))
	fmt.Printf("Length of stored hashed password: %d\n", len(parent.Password))

	if err := bcrypt.CompareHashAndPassword([]byte(parent.Password), []byte(password)); err != nil {
		fmt.Printf("Password comparison error: %v\n", err)
		return models.Parent{}, "", err
	}
	if parent.CodeExpiresAt == nil || time.Now().After(*parent.CodeExpiresAt) {
		// Генерируем новый код
		updatedParent, err := s.RefreshParentCode(parent.FirebaseUID)
		if err != nil {
			// Логируем ошибку, но продолжаем работу
			fmt.Printf("Failed to refresh parent code: %v", err)
		} else {
			parent = updatedParent
		}
	}
	// Generate JWT token with additional fields
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email:       email,
		FirebaseUID: parent.FirebaseUID, // В будущем переименуем в UserID
		UserType:    "parent",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return models.Parent{}, "", err
	}

	return parent, tokenString, nil
}

func (s *AuthService) RegisterChild(lang, code, name string) (models.Child, string, error) {
	parent, err := s.ParentRepo.FindByCode(code)
	if err != nil {
		return models.Child{}, "", errors.New("invalid parent code")
	}

	// Проверяем срок действия кода родителя
	if parent.CodeExpiresAt == nil || time.Now().After(*parent.CodeExpiresAt) {
		return models.Child{}, "", errors.New("parent code has expired")
	}

	// Генерируем уникальный ID для ребенка (вместо Firebase)
	userID := uuid.New().String()

	// Generate unique code for the child
	var childCode string
	for {
		childCode = fmt.Sprintf("%04d", 1000+rand.Intn(9000)) // Гарантируем формат 4 цифр
		var count int64
		err := s.ChildRepo.CountByCode(childCode, &count)
		if err != nil {
			return models.Child{}, "", err
		}
		if count == 0 {
			break
		}
	}

	// Create user in local database
	familyData := map[string]interface{}{
		"parent_id":           parent.ID,
		"parent_name":         parent.Name,
		"parent_email":        parent.Email,
		"parent_firebase_uid": parent.FirebaseUID,
	}
	familyJSON, _ := json.Marshal(familyData)

	child := models.Child{
		Lang:        lang,
		Name:        name, // Используем переданное имя
		Family:      string(familyJSON),
		FirebaseUID: userID, // Используем UUID вместо Firebase UID
		IsBinded:    true,
		Code:        childCode,
		Role:        "child",
	}

	if err := s.ChildRepo.Save(child); err != nil {
		return models.Child{}, "", err
	}

	// Update parent's family field
	var family []map[string]interface{}
	json.Unmarshal([]byte(parent.Family), &family)
	family = append(family, map[string]interface{}{
		"child_id":     child.ID,
		"name":         child.Name,
		"lang":         child.Lang,
		"firebase_uid": child.FirebaseUID,
		"isBinded":     child.IsBinded,
		"usage_data":   child.UsageData,
		"gender":       child.Gender,
		"age":          child.Age,
		"birthday":     child.Birthday,
		"code":         child.Code,
	})
	familyJson, _ := json.Marshal(family)
	parent.Family = string(familyJson)

	// Generate new unique 4-digit code for the parent
	var newCode string
	for {
		newCode = fmt.Sprintf("%04d", 1000+rand.Intn(9000)) // Гарантируем формат 4 цифр
		var count int64
		err := s.ParentRepo.CountByCode(newCode, &count)
		if err != nil {
			return models.Child{}, "", err
		}
		if count == 0 {
			break
		}
	}
	parent.Code = newCode
	codeExpiresAt := time.Now().Add(24 * time.Hour)
	parent.CodeExpiresAt = &codeExpiresAt

	if err := s.ParentRepo.Save(parent); err != nil {
		return models.Child{}, "", err
	}

	// Generate JWT token with additional fields
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email:       child.FirebaseUID, // В случае с ребенком email может не быть, используем FirebaseUID
		FirebaseUID: userID,            // Используем userID
		UserType:    "child",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return models.Child{}, "", err
	}

	return child, tokenString, nil
}

// LoginChild authenticates a child using their code and returns a JWT token
func (s *AuthService) LoginChild(code string) (models.Child, string, error) {
	child, err := s.ChildRepo.FindByCode(code)
	if err != nil {
		return models.Child{}, "", errors.New("invalid code")
	}

	// Generate JWT token with additional fields
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email:       child.FirebaseUID, // Child doesn't have email, use FirebaseUID
		FirebaseUID: child.FirebaseUID,
		UserType:    "child",
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return models.Child{}, "", err
	}

	// Check if child is already bound to a parent
	if child.IsBinded {
		// Update the child's status if needed
		if !child.IsBinded {
			child.IsBinded = true
			if err := s.ChildRepo.Save(child); err != nil {
				return models.Child{}, "", err
			}
		}
	}

	return child, tokenString, nil
}

func (s *AuthService) VerifyToken(uid string) (interface{}, error) {
	var parent models.Parent
	var child models.Child

	parent, err := s.ParentRepo.FindByFirebaseUID(uid)
	if err == nil {
		return parent, nil
	}

	child, err = s.ChildRepo.FindByFirebaseUID(uid)
	if err == nil {
		return child, nil
	}

	return nil, errors.New("user not found")
}

func (s *AuthService) RefreshParentCode(firebaseUID string) (models.Parent, error) {
	// Находим родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return models.Parent{}, err
	}

	// Генерируем новый уникальный код
	var newCode string
	for {
		newCode = fmt.Sprintf("%04d", 1000+rand.Intn(9000))
		var count int64
		err := s.ParentRepo.CountByCode(newCode, &count)
		if err != nil {
			return models.Parent{}, err
		}
		if count == 0 {
			break
		}
	}

	// Устанавливаем новый код со сроком действия 24 часа
	parent.Code = newCode
	codeExpiresAt := time.Now().Add(24 * time.Hour)
	parent.CodeExpiresAt = &codeExpiresAt

	// Сохраняем изменения
	if err := s.ParentRepo.Save(parent); err != nil {
		return models.Parent{}, err
	}

	return parent, nil
}

func (s *AuthService) IsParentCodeValid(code string) (bool, error) {
	parent, err := s.ParentRepo.FindByCode(code)
	if err != nil {
		return false, err
	}

	// Проверяем срок действия кода
	if parent.CodeExpiresAt == nil || time.Now().After(*parent.CodeExpiresAt) {
		return false, nil
	}

	return true, nil
}

func (s *AuthService) EnsureValidParentCode(firebaseUID string) (models.Parent, error) {
	// Находим родителя
	parent, err := s.ParentRepo.FindByFirebaseUID(firebaseUID)
	if err != nil {
		return models.Parent{}, err
	}

	// Проверяем, истек ли срок действия кода
	if parent.CodeExpiresAt == nil || time.Now().After(*parent.CodeExpiresAt) {
		// Код истек, обновляем его
		return s.RefreshParentCode(firebaseUID)
	}

	// Код действителен, возвращаем родителя без изменений
	return parent, nil
}

// GenerateToken создает JWT токен для пользователя по его UID
func (s *AuthService) GenerateToken(userID string) (string, error) {
	// Проверяем тип пользователя (родитель или ребенок)
	parentExists, _ := s.ParentRepo.FindByFirebaseUID(userID)
	userType := "parent"

	if parentExists.ID == 0 {
		childExists, _ := s.ChildRepo.FindByFirebaseUID(userID)
		if childExists.ID != 0 {
			userType = "child"
		}
	}

	// Создаем токен с использованием той же структуры Claims
	expirationTime := time.Now().Add(24 * time.Hour * 7) // 7 дней
	claims := &Claims{
		FirebaseUID: userID,
		UserType:    userType,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// VerifyEmail подтверждает email пользователя
func (s *AuthService) VerifyEmail(email, code string) error {
	parent, err := s.ParentRepo.FindByEmail(email)
	if err != nil {
		return errors.New("parent not found")
	}

	// Проверяем код верификации
	if parent.Code != code {
		return errors.New("invalid verification code")
	}

	// Проверяем срок действия кода
	if parent.CodeExpiresAt == nil || time.Now().After(*parent.CodeExpiresAt) {
		return errors.New("verification code expired")
	}

	// Устанавливаем флаг подтверждения email
	parent.EmailVerified = true

	// Сохраняем изменения в базе данных
	if err := s.ParentRepo.Save(parent); err != nil {
		return err
	}

	return nil
}

// SendPasswordResetCode отправляет код для сброса пароля
func (s *AuthService) SendPasswordResetCode(email string) error {
	parent, err := s.ParentRepo.FindByEmail(email)
	if err != nil {
		return errors.New("parent not found")
	}

	// Генерируем новый код
	resetCode := fmt.Sprintf("%04d", 1000+rand.Intn(9000))
	codeExpiresAt := time.Now().Add(1 * time.Hour) // Код действителен 1 час

	// Используем поле Code вместо ResetCode
	parent.Code = resetCode
	parent.CodeExpiresAt = &codeExpiresAt

	// Сохраняем изменения
	if err := s.ParentRepo.Save(parent); err != nil {
		return err
	}

	// TODO: Отправить email с кодом сброса пароля
	// Вместо Firebase используйте свой сервис отправки email

	return nil
}

// ResetPassword сбрасывает пароль с помощью кода
func (s *AuthService) ResetPassword(email, code, newPassword string) error {
	parent, err := s.ParentRepo.FindByEmail(email)
	if err != nil {
		return errors.New("parent not found")
	}

	// Проверяем код сброса (используем поле Code)
	if parent.Code != code {
		return errors.New("invalid reset code")
	}

	// Проверяем срок действия кода
	if parent.CodeExpiresAt == nil || time.Now().After(*parent.CodeExpiresAt) {
		return errors.New("reset code expired")
	}

	// Хешируем новый пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Обновляем пароль и генерируем новый код (чтобы старый не использовался повторно)
	parent.Password = string(hashedPassword)
	newCode := fmt.Sprintf("%04d", 1000+rand.Intn(9000))
	parent.Code = newCode
	codeExpiresAt := time.Now().Add(24 * time.Hour)
	parent.CodeExpiresAt = &codeExpiresAt

	// Сохраняем изменения
	if err := s.ParentRepo.Save(parent); err != nil {
		return err
	}

	return nil
}
