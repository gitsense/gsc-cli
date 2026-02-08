/**
 * Component: Chat Database Models
 * Block-UUID: 49ca7d56-f7b1-42a9-a81d-03ae696ff784
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Data structures mapping to the GitSense Chat SQLite schema (chats and messages tables).
 * Language: Go
 * Created-at: 2026-02-08T04:11:52.031Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package db

import (
	"database/sql"
	"time"
)

// Chat represents a record in the 'chats' table.
type Chat struct {
	ID        int64     `json:"id"`
	UUID      string    `json:"uuid"`
	MainModel string    `json:"main_model"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a record in the 'messages' table.
// Fields use sql.Null types where the database schema allows NULL values.
type Message struct {
	ID              int64           `json:"id"`
	Type            string          `json:"type"`
	Deleted         int             `json:"deleted"`
	Visibility      string          `json:"visibility"`
	ChatID          int64           `json:"chat_id"`
	ParentID        int64           `json:"parent_id"`
	Level           int             `json:"level"`
	Model           sql.NullString  `json:"model"`
	RealModel       sql.NullString  `json:"real_model"`
	Temperature     sql.NullFloat64 `json:"temperature"`
	Role            string          `json:"role"`
	Message         sql.NullString  `json:"message"`
	OriginalMessage sql.NullString  `json:"original_message"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ModifiedAt      sql.NullTime    `json:"modified_at"`
}
