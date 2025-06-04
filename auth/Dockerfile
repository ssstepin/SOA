# Этап сборки
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем файлы модулей (для кэширования)
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем статический бинарник с правами на исполнение
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /auth main.go && \
    chmod +x /auth

# Этап запуска - используем alpine вместо scratch для простоты
FROM alpine:latest

# Копируем бинарник
COPY --from=builder /auth /auth

COPY /*.pem /

# Копируем SSL-сертификаты (если нужны HTTPS-запросы)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/auth"]