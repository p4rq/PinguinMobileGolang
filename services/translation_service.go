package services

import (
	"PinguinMobile/models"
	"sync"

	"gorm.io/gorm"
)

type TranslationService struct {
	DB               *gorm.DB
	translationCache map[string]map[string]string
	mutex            sync.RWMutex
}

func NewTranslationService(db *gorm.DB) *TranslationService {
	return &TranslationService{
		DB:               db,
		translationCache: make(map[string]map[string]string),
	}
}

func (s *TranslationService) GetAllTranslations(lang string) map[string]string {
	// Проверяем кэш
	s.mutex.RLock()
	if cached, exists := s.translationCache[lang]; exists {
		s.mutex.RUnlock()
		return cached
	}
	s.mutex.RUnlock()

	// Загружаем переводы из БД
	var translations []models.Translation
	s.DB.Find(&translations)

	result := make(map[string]string)
	for _, t := range translations {
		var translation string
		switch lang {
		case "ru":
			translation = t.Russian
		case "en":
			translation = t.English
		case "kz":
			translation = t.Kazakh
		default:
			translation = t.English
		}
		result[t.Key] = translation
	}

	// Сохраняем в кэш
	s.mutex.Lock()
	s.translationCache[lang] = result
	s.mutex.Unlock()

	return result
}
