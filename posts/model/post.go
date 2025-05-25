package model

import "time"

type Post struct {
	ID     int32  `json:"id"`
	UserId int32  `json:"user_id"`
	Text   string `json:"text"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
