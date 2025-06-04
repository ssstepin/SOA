package model

import (
	"time"
)

type User struct {
	ID       int    `json:"id" db:"id"`
	Username string `json:"username" db:"username"`
	Mail     string `json:"mail" db:"mail"`
	Password string `json:"password" db:"password_md5"`

	// Поля, которые могут быть NULL
	Phone      *string `json:"phone,omitempty" db:"phone"`
	FirstName  *string `json:"first_name,omitempty" db:"first_name"`
	SecondName *string `json:"second_name,omitempty" db:"second_name"`

	// Метаданные
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

func (u *User) Columns() []string {
	return []string{
		"id",
		"username",
		"mail",
		"phone",
		"first_name",
		"second_name",
		"created_at",
		"updated_at",
	}
}
