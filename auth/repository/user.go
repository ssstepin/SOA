package repository

import (
	"auth/storage"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Masterminds/squirrel"
	"log"
	"strings"

	"auth/model"
)

type UserRepository struct {
	postgres storage.Postgres
	sb       squirrel.StatementBuilderType
}

type UserRepositoryError error

var (
	ErrUserNotFound      UserRepositoryError = errors.New("user not found")
	ErrUsernameTaken     UserRepositoryError = errors.New("username already taken")
	ErrEmailTaken        UserRepositoryError = errors.New("email already taken")
	ErrInvalidUpdateData UserRepositoryError = errors.New("invalid user update data")
)

func NewUserRepository(postgres storage.Postgres) *UserRepository {
	return &UserRepository{
		postgres: postgres,
		sb:       squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (s *UserRepository) CreateUser(user *model.User) error {
	query, args, err := s.sb.Insert("users").
		Columns("username", "mail", "password_md5", "phone", "first_name", "second_name").
		Values(user.Username, user.Mail, user.Password, user.Phone, user.FirstName, user.SecondName).
		Suffix("RETURNING id, created_at, updated_at").
		ToSql()

	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	err = s.postgres.DB().QueryRow(query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "users_username_key") {
			return ErrUsernameTaken
		}
		if strings.Contains(err.Error(), "users_mail_key") {
			return ErrEmailTaken
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("Created user: %v, pass: %v", user, user.Password)

	return nil
}

func (r *UserRepository) GetUserByUsername(username string) (*model.User, error) {
	query, args, err := r.sb.Select(
		"id", "username", "mail", "password_md5",
		"phone", "first_name", "second_name",
		"created_at", "updated_at",
	).
		From("users").
		Where(squirrel.Eq{"username": username}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	user := &model.User{}
	err = r.postgres.DB().QueryRow(query, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Mail,
		&user.Password,
		&user.Phone,
		&user.FirstName,
		&user.SecondName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return user, nil
}

func (r *UserRepository) GetUserById(id int) (*model.User, error) {
	query, args, err := r.sb.Select(
		"id", "username", "mail", "password_md5",
		"phone", "first_name", "second_name",
		"created_at", "updated_at",
	).
		From("users").
		Where(squirrel.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	user := &model.User{}
	err = r.postgres.DB().QueryRow(query, args...).Scan(
		&user.ID,
		&user.Username,
		&user.Mail,
		&user.Password,
		&user.Phone,
		&user.FirstName,
		&user.SecondName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return user, nil
}
