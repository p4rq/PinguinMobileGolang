FROM golang:1.18-alpine AS builder

WORKDIR /app

# Копируем модули Go
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Использование минимального образа
FROM alpine:latest

WORKDIR /app

# Устанавливаем зависимости
RUN apk --no-cache add ca-certificates tzdata

# Копируем скомпилированное приложение
COPY --from=builder /app/main .

# Открываем порт
EXPOSE 8000

# Запускаем приложение
CMD ["./main"]