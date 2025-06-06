package main

import (
	"encoding/csv"
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

	csvPath := filepath.Join(dir, "translate2.csv")
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

	// Обработка тройных кавычек
	if len(s) >= 6 && strings.HasPrefix(s, "\"\"\"") && strings.HasSuffix(s, "\"\"\"") {
		s = s[3 : len(s)-3]
	} else if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		// Обычные кавычки
		s = s[1 : len(s)-1]
	}

	// Замена двойных кавычек на одинарные
	s = strings.ReplaceAll(s, "\"\"", "\"")
	return s
}
