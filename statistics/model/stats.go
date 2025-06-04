package model

import (
	"time"
)

type Event struct {
	EventDate   time.Time
	EventTime   time.Time
	EventType   string // "like", "view", "comment"
	UserID      uint32
	PostID      uint32
	CommentText *string
}

type PostStats struct {
	PostID   uint32
	Likes    uint32
	Views    uint32
	Comments uint32
	Count    uint32 // Для топов
}

type UserStats struct {
	UserID uint32
	Count  uint32
}

type DayStats struct {
	Date     time.Time
	Likes    uint32
	Views    uint32
	Comments uint32
}
