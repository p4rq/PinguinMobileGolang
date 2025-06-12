package config

import (
	"PinguinMobile/models"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"google.golang.org/api/option"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB
var FirebaseAuth *auth.Client

func InitDatabase() {
	// Получаем значения из environment variables
	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	port := os.Getenv("DB_PORT")

	// Используем значение из DB_SSLMODE или "require" для Render
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		if strings.Contains(host, "render.com") {
			sslmode = "require"
		} else {
			sslmode = "disable"
		}
	}

	// Отладочный вывод
	log.Printf("Connecting to database: host=%s user=%s dbname=%s port=%s sslmode=%s",
		host, user, dbname, port, sslmode)

	// Формируем строку подключения
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Almaty",
		host, user, password, dbname, port, sslmode)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	log.Println("Successfully connected to database!")

	DB.AutoMigrate(&models.Parent{}, &models.Child{})
}

func InitFirebase() {
	opt := option.WithCredentialsFile(os.Getenv("FIREBASE_CREDENTIALS_PATH"))
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	authClient, err := app.Auth(context.Background())
	if err != nil {
		log.Fatalf("error getting Auth client: %v\n", err)
	}

	FirebaseAuth = authClient
}
