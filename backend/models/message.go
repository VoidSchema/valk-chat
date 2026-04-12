package models

import (
	"time"
)

type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
}
