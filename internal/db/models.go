/**
 * Component: Chat Database Models
 * Block-UUID: b8e2e972-aa89-4b86-b9b8-c298697dd5ed
 * Parent-UUID: 49ca7d56-f7b1-42a9-a81d-03ae696ff784
 * Version: 1.1.0
 * Description: Data structures mapping to the GitSense Chat SQLite schema (chats and messages tables).
 * Language: Go
 * Created-at: 2026-02-19T17:58:56.212Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package db

import (
	"database/sql"
	"time"
)

// Chat represents a record in the 'chats' table.
type Chat struct {
	ID              int64          `json:"id"`
	UUID            string         `json:"uuid"`
	Type            string         `json:"type"`
	Deleted         int            `json:"deleted"`
	Visibility      string         `json:"visibility"`
	Owner           string         `json:"owner"`
	Name            string         `json:"name"`
	ParentID        int64          `json:"parent_id"`
	GroupID         int64          `json:"group_id"`
	PromptID        int64          `json:"prompt_id"`
	MainModel       string         `json:"main_model"`
	Pinned          sql.NullInt64  `json:"pinned"`
	Protected       sql.NullInt64  `json:"protected"`
	OrderWeight     sql.NullInt64  `json:"order_weight"`
	ForkedFromMsgID sql.NullInt64  `json:"forked_from_msg_id"`
	Meta            sql.NullString `json:"meta"`
	IsDefaultName   int            `json:"is_default_name"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	ModifiedAt      sql.NullTime   `json:"modified_at"`
}

// Message represents a record in the 'messages' table.
// Fields use sql.Null types where the database schema allows NULL values.
type Message struct {
	ID                   int64           `json:"id"`
	Type                 string          `json:"type"`
	Deleted              int             `json:"deleted"`
	Visibility           string          `json:"visibility"`
	ChatID               int64           `json:"chat_id"`
	ParentID             int64           `json:"parent_id"`
	Level                int             `json:"level"`
	Sample               sql.NullInt64   `json:"sample"`
	Model                sql.NullString  `json:"model"`
	RealModel            sql.NullString  `json:"real_model"`
	Temperature          sql.NullFloat64 `json:"temperature"`
	TopK                 sql.NullFloat64 `json:"top_k"`
	TopP                 sql.NullFloat64 `json:"top_p"`
	MaxTokens            sql.NullInt64   `json:"max_tokens"`
	Role                 string          `json:"role"`
	Message              sql.NullString  `json:"message"`
	OriginalMessage      sql.NullString  `json:"original_message"`
	CopiedFromMsgID      sql.NullInt64   `json:"copied_from_msg_id"`
	Pinned               sql.NullInt64   `json:"pinned"`
	ChatCompletionStats  sql.NullString  `json:"chat_completion_stats"`
	Meta                 sql.NullString  `json:"meta"`
	BlobID               sql.NullInt64   `json:"blob_id"`
	Priority             sql.NullInt64   `json:"priority"`
	JobID                sql.NullInt64   `json:"job_id"`
	JobAttempts          sql.NullInt64   `json:"job_attempts"`
	JobException         sql.NullString  `json:"job_exception"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
	ModifiedAt           sql.NullTime    `json:"modified_at"`
}

// PublishedManifest represents a record in the 'published_manifests' table.
type PublishedManifest struct {
	ID          int64         `json:"id"`
	UUID        string        `json:"uuid"`
	Owner       string        `json:"owner"`
	Repo        string        `json:"repo"`
	Branch      string        `json:"branch"`
	Database    string        `json:"database"`
	PublishedAt time.Time     `json:"published_at"`
	Deleted     int           `json:"deleted"`
	RootChatID  sql.NullInt64 `json:"root_chat_id"`
	OwnerChatID sql.NullInt64 `json:"owner_chat_id"`
	RepoChatID  sql.NullInt64 `json:"repo_chat_id"`
}

