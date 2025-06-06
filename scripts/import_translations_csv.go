// scripts/import_translations_csv.go
package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"PinguinMobile/models"
)

func main() {
	// Загружаем переменные окружения из .env файла
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Ошибка при загрузке .env файла:", err)
		// Продолжаем работу даже если .env не загружен
	}

	// Определяем путь к CSV файлу относительно текущего рабочего каталога
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Ошибка при получении текущего каталога:", err)
		os.Exit(1)
	}

	csvPath := filepath.Join(dir, "translate2.csv")
	fmt.Printf("Поиск CSV файла по пути: %s\n", csvPath)

	// Открываем CSV файл
	file, err := os.Open(csvPath)
	if err != nil {
		fmt.Println("Ошибка при открытии файла:", err)
		// Пробуем найти файл в подкаталоге scripts
		csvPath = filepath.Join(dir, "scripts", "translate.csv")
		fmt.Printf("Пробуем путь: %s\n", csvPath)

		file, err = os.Open(csvPath)
		if err != nil {
			fmt.Println("Файл не найден. Проверьте путь к CSV файлу.")
			os.Exit(1)
		}
	}
	defer file.Close()
	fmt.Println("CSV файл успешно открыт.")

	// Используем стандартный CSV reader с настройками для многострочных полей
	reader := csv.NewReader(file)
	reader.LazyQuotes = true       // Разрешает кавычки внутри полей
	reader.FieldsPerRecord = -1    // Допускает разное количество полей в строках
	reader.TrimLeadingSpace = true // Удаляет начальные пробелы в полях

	// Читаем все строки CSV файла
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Ошибка при чтении CSV файла:", err)
		os.Exit(1)
	}

	// Подключаемся к базе данных
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "11052004ARAd."
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "Pinguin"
	}

	// Формируем строку подключения к базе данных
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName, dbPort)

	fmt.Println("Подключение к базе данных...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("Ошибка подключения к базе данных:", err)
		os.Exit(1)
	}
	fmt.Println("Успешное подключение к базе данных")

	// Создаем таблицу translations, если она не существует
	err = db.AutoMigrate(&models.Translation{})
	if err != nil {
		fmt.Println("Ошибка при создании таблицы:", err)
		os.Exit(1)
	}
	fmt.Println("Таблица translations проверена/создана")

	// Счетчик импортированных записей
	count := 0

	// Пропускаем заголовок (первая строка)
	header := records[0]
	fmt.Println("Заголовок пропущен:", strings.Join(header, ", "))

	// Обработка строк файла (начиная со второй строки)
	for _, record := range records[1:] {
		// Пропускаем пустые строки
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		// Проверяем наличие достаточного количества полей
		if len(record) < 3 {
			fmt.Println("Строка имеет недостаточно данных:", strings.Join(record, ", "))
			continue
		}

		// Извлекаем и валидируем ID
		idStr := strings.TrimSpace(record[0])
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			fmt.Printf("Ошибка при конвертации ID '%s': %v, строка: %s\n", idStr, err, strings.Join(record, ", "))
			continue
		}

		// Получаем значения полей (с проверкой индексов)
		russian := ""
		english := ""
		kazakh := ""

		if len(record) > 1 {
			russian = cleanQuotes(record[1])
		}
		if len(record) > 2 {
			english = cleanQuotes(record[2])
		}
		if len(record) > 3 {
			kazakh = cleanQuotes(record[3])
		}

		// Создаем запись о переводе
		translation := models.Translation{
			ID:      uint(id),
			Key:     fmt.Sprintf("key_%d", id),
			Russian: russian,
			English: english,
			Kazakh:  kazakh,
		}

		// Сохраняем или обновляем запись в базе данных
		var existing models.Translation
		result := db.First(&existing, id)
		if result.Error != nil {
			// Запись не существует, создаем новую
			if err := db.Create(&translation).Error; err != nil {
				fmt.Printf("Ошибка при создании записи ID=%d: %v\n", id, err)
				continue
			}
			fmt.Printf("Создана новая запись ID=%d\n", id)
		} else {
			// Запись существует, обновляем ее
			if err := db.Model(&existing).Updates(translation).Error; err != nil {
				fmt.Printf("Ошибка при обновлении записи ID=%d: %v\n", id, err)
				continue
			}
			fmt.Printf("Обновлена запись ID=%d\n", id)
		}

		count++
	}

	fmt.Printf("\nИмпорт завершен успешно. Всего обработано: %d записей\n", count)
}

// cleanQuotes удаляет лишние кавычки из строки
func cleanQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		// Заменяем двойные кавычки на одинарные внутри текста
		s = strings.ReplaceAll(s, "\"\"", "\"")
	}
	return s
}
