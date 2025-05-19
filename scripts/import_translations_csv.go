// scripts/import_translations_csv.go
package main

import (
	"bufio"
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

	csvPath := filepath.Join(dir, "translate.csv")
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

	// Вместо стандартного CSV reader используем построчное чтение и ручной парсинг
	scanner := bufio.NewScanner(file)

	// Пропускаем заголовок (первая строка)
	if scanner.Scan() {
		fmt.Println("Заголовок пропущен:", scanner.Text())
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

	// Обработка строк файла
	for scanner.Scan() {
		line := scanner.Text()
		// Пропускаем пустые строки
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Ручной парсинг строки с учетом кавычек
		parts := parseCSVLine(line)
		if len(parts) < 4 {
			fmt.Println("Строка имеет недостаточно данных:", line)
			continue
		}

		// Извлекаем и валидируем ID
		idStr := parts[0]
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			fmt.Printf("Ошибка при конвертации ID '%s': %v, строка: %s\n", idStr, err, line)
			continue
		}

		// Создаем запись о переводе
		translation := models.Translation{
			ID:      uint(id),
			Key:     fmt.Sprintf("key_%d", id),
			Russian: cleanQuotes(parts[1]),
			English: cleanQuotes(parts[2]),
			Kazakh:  cleanQuotes(parts[3]),
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
			fmt.Printf("Создана новая запись ID=%d: %s\n", id, translation.Russian)
		} else {
			// Запись существует, обновляем ее
			if err := db.Model(&existing).Updates(translation).Error; err != nil {
				fmt.Printf("Ошибка при обновлении записи ID=%d: %v\n", id, err)
				continue
			}
			fmt.Printf("Обновлена запись ID=%d: %s\n", id, translation.Russian)
		}

		count++
	}

	// Проверяем наличие ошибок сканирования
	if err := scanner.Err(); err != nil {
		fmt.Println("Ошибка при чтении файла:", err)
	}

	fmt.Printf("\nИмпорт завершен успешно. Всего обработано: %d записей\n", count)
}

// parseCSVLine разбирает строку CSV с учетом всех возможных форматов
func parseCSVLine(line string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	doubleQuoteEscape := false

	for i := 0; i < len(line); i++ {
		char := line[i]

		// Обрабатываем кавычки
		if char == '"' {
			// Проверяем на экранированные кавычки
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				current.WriteByte('"')
				i++ // Пропускаем следующую кавычку
				continue
			}
			inQuotes = !inQuotes
			doubleQuoteEscape = (i > 0 && i+1 < len(line) && line[i-1] == '"' && line[i+1] == '"')
			if doubleQuoteEscape {
				current.WriteByte('"')
			}
			continue
		}

		// Если встречаем запятую вне кавычек, это разделитель
		if char == ',' && !inQuotes {
			result = append(result, current.String())
			current.Reset()
			continue
		}

		// В остальных случаях добавляем символ к текущему полю
		current.WriteByte(char)
	}

	// Добавляем последнее поле
	result = append(result, current.String())
	return result
}

// cleanQuotes удаляет лишние кавычки из строки
func cleanQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}
