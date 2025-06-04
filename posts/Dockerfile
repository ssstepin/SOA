# Этап сборки
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем файлы модулей (для кэширования)
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем статический бинарник с правами на исполнение
RUN go build -ldflags="-w -s" -o /posts main.go && \
    chmod +x /posts

# Этап запуска - используем alpine вместо scratch для простоты
FROM alpine:latest

# Копируем бинарник
COPY --from=builder /posts /posts

# Копируем SSL-сертификаты (если нужны HTTPS-запросы)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/posts"]