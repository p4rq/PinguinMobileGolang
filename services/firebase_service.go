package services

import (
	"context"
	"fmt"
	"os"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var (
	authClient *auth.Client
	authOnce   sync.Once
	authErr    error
)

// GetAuthClient возвращает инициализированный экземпляр Firebase Auth Client
func GetAuthClient() (*auth.Client, error) {
	authOnce.Do(func() {
		ctx := context.Background()

		// Получаем путь к файлу учетных данных из переменной окружения
		credPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
		if credPath == "" {
			authErr = fmt.Errorf("FIREBASE_CREDENTIALS_PATH not set")
			return
		}

		// Создаем опцию с учетными данными
		opt := option.WithCredentialsFile(credPath)

		// Инициализируем Firebase App
		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			authErr = fmt.Errorf("error initializing Firebase app: %w", err)
			return
		}

		// Получаем Auth клиент
		authClient, authErr = app.Auth(ctx)
	})

	return authClient, authErr
}

// DeleteFirebaseUser удаляет пользователя из Firebase Auth по его UID
func DeleteFirebaseUser(uid string) error {
	client, err := GetAuthClient()
	if err != nil {
		return err
	}

	ctx := context.Background()
	return client.DeleteUser(ctx, uid)
}
