package services

import (
	"PinguinMobile/models"
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"firebase.google.com/go/auth"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var jwtKey = []byte("your_secret_key")

type Claims struct {
	Email string `json:"email"`
	jwt.StandardClaims
}

type AuthService struct {
	DB           *gorm.DB
	FirebaseAuth *auth.Client
}

func NewAuthService(db *gorm.DB, firebaseAuth *auth.Client) *AuthService {
	return &AuthService{DB: db, FirebaseAuth: firebaseAuth}
}

func (s *AuthService) RegisterParent(lang, name, email, password string) (models.Parent, string, error) {
	// Register user in Firebase
	params := (&auth.UserToCreate{}).
		Email(email).
		Password(password).
		DisplayName(name)

	createdUser, err := s.FirebaseAuth.CreateUser(context.Background(), params)
	if err != nil {
		return models.Parent{}, "", err
	}
	firebaseUid := createdUser.UID

	// Generate unique 4-digit code
	var code string
	for {
		code = strconv.Itoa(1000 + rand.Intn(9000))
		var count int64
		s.DB.Model(&models.Parent{}).Where("code = ?", code).Count(&count)
		if count == 0 {
			break
		}
	}

	// Create user in local database
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	parent := models.Parent{
		Lang:        lang,
		Name:        name,
		Email:       email,
		Password:    string(hashedPassword),
		Role:        "parent",
		Family:      "[]",
		Code:        code,
		FirebaseUID: firebaseUid,
	}

	if result := s.DB.Create(&parent); result.Error != nil {
		return models.Parent{}, "", result.Error
	}

	// Generate JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: email,
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
	var parent models.Parent
	if err := s.DB.Where("email = ?", email).First(&parent).Error; err != nil {
		return models.Parent{}, "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(parent.Password), []byte(password)); err != nil {
		return models.Parent{}, "", err
	}

	// Generate JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: email,
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
	var parent models.Parent
	if err := s.DB.Where("code = ?", code).First(&parent).Error; err != nil {
		return models.Child{}, "", err
	}

	// Register user in Firebase
	params := (&auth.UserToCreate{}).
		DisplayName(name)

	createdUser, err := s.FirebaseAuth.CreateUser(context.Background(), params)
	if err != nil {
		return models.Child{}, "", err
	}
	firebaseUid := createdUser.UID

	// Generate unique code for the child
	var childCode string
	for {
		childCode = strconv.Itoa(1000 + rand.Intn(9000))
		var count int64
		s.DB.Model(&models.Child{}).Where("code = ?", childCode).Count(&count)
		if count == 0 {
			break
		}
	}

	// Create user in local database
	child := models.Child{
		Lang:        lang,
		Name:        name,
		Family:      `{"parent_id":` + strconv.Itoa(int(parent.ID)) + `}`,
		FirebaseUID: firebaseUid,
		IsBinded:    true,
		Code:        childCode,
	}

	if result := s.DB.Create(&child); result.Error != nil {
		return models.Child{}, "", result.Error
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
		newCode = strconv.Itoa(1000 + rand.Intn(9000))
		var count int64
		s.DB.Model(&models.Parent{}).Where("code = ?", newCode).Count(&count)
		if count == 0 {
			break
		}
	}
	parent.Code = newCode
	s.DB.Save(&parent)

	// Generate JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Email: child.FirebaseUID,
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

func (s *AuthService) VerifyToken(uid string) (interface{}, error) {
	var parent models.Parent
	var child models.Child

	if err := s.DB.Where("firebase_uid = ?", uid).First(&parent).Error; err == nil {
		return parent, nil
	}

	if err := s.DB.Where("firebase_uid = ?", uid).First(&child).Error; err == nil {
		return child, nil
	}

	return nil, errors.New("user not found")
}
