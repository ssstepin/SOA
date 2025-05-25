package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

type Postgres struct {
	db *sql.DB
}

// New создает новое подключение к Postgres
func New(connectionString string) (*Postgres, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL")
	return &Postgres{db: db}, nil
}

// Close закрывает подключение к БД
func (p *Postgres) Close() error {
	return p.db.Close()
}

// DB возвращает соединение с БД
func (p *Postgres) DB() *sql.DB {
	return p.db
}
