package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"PinguinMobile/models"
)

func main() {
	// Определяем путь к CSV файлу
	dir, err := os.Getwd()
	if err != nil {
		fmt.Println("Ошибка при получении текущего каталога:", err)
		os.Exit(1)
	}

	csvPath := filepath.Join(dir, "translatev2.csv")
	fmt.Printf("Поиск CSV файла по пути: %s\n", csvPath)

	// Открываем CSV файл
	file, err := os.Open(csvPath)
	if err != nil {
		fmt.Println("Ошибка при открытии файла:", err)
		// Пробуем другие пути
		alternatePaths := []string{
			filepath.Join(dir, "scripts", "translate.csv"),
			filepath.Join(dir, "..", "translate.csv"),
		}

		for _, path := range alternatePaths {
			fmt.Printf("Пробуем путь: %s\n", path)
			file, err = os.Open(path)
			if err == nil {
				csvPath = path
				break
			}
		}

		if err != nil {
			fmt.Println("Файл не найден. Проверьте путь к CSV файлу.")
			os.Exit(1)
		}
	}
	defer file.Close()
	fmt.Println("CSV файл успешно открыт:", csvPath)

	// Жестко закодированные параметры для Render
	dbUser := "pinguin_user"
	dbPassword := "AYaZRsxBvKCVBMPdctqXJEiaRWNW88Wf"
	dbHost := "dpg-d0i9cbbuibrs73a2uqq0-a.singapore-postgres.render.com"
	dbPort := "5432"
	dbName := "pinguin_mobile"
	dbSSLMode := "require"

	// Формируем строку подключения
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	fmt.Println("Подключение к базе данных Render...")
	fmt.Println("Используется строка подключения:", dsn)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println("Ошибка подключения к базе данных:", err)
		os.Exit(1)
	}
	fmt.Println("Успешное подключение к базе данных Render")

	// Создаем таблицу translations, если она не существует
	err = db.AutoMigrate(&models.Translation{})
	if err != nil {
		fmt.Println("Ошибка при создании таблицы:", err)
		os.Exit(1)
	}
	fmt.Println("Таблица translations проверена/создана")

	// Считываем CSV файл
	scanner := bufio.NewScanner(file)

	// Пропускаем заголовок
	if scanner.Scan() {
		fmt.Println("Заголовок пропущен:", scanner.Text())
	}

	count := 0

	// Обработка строк файла
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		parts := parseCSVLine(line)
		if len(parts) < 4 {
			fmt.Println("Строка имеет недостаточно данных:", line)
			continue
		}

		idStr := parts[0]
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			fmt.Printf("Ошибка при конвертации ID '%s': %v\n", idStr, err)
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

		// Сохраняем или обновляем запись
		var existing models.Translation
		result := db.First(&existing, id)
		if result.Error != nil {
			if err := db.Create(&translation).Error; err != nil {
				fmt.Printf("Ошибка при создании записи ID=%d: %v\n", id, err)
				continue
			}
			fmt.Printf("Создана новая запись ID=%d: %s\n", id, translation.Russian)
		} else {
			if err := db.Model(&existing).Updates(translation).Error; err != nil {
				fmt.Printf("Ошибка при обновлении записи ID=%d: %v\n", id, err)
				continue
			}
			fmt.Printf("Обновлена запись ID=%d: %s\n", id, translation.Russian)
		}

		count++
	}

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
