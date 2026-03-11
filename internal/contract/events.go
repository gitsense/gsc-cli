/*
 * Component: Contract Events Database Helper
 * Block-UUID: 1c947add-f763-413e-9bdd-f8abb52cf2bd
 * Parent-UUID: 44c7ed6b-27af-4941-81cb-bc6c5f44d5a0
 * Version: 1.4.0
 * Description: Added MessageID field to ChatMessagePayload to support the 'chat_message_posted' event type, which carries the ID of the message inserted by the backend.
 * Language: Go
 * Created-at: 2026-03-07T04:11:57.272Z
 * Authors: Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
 */


package contract

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ChatMessagePayload represents the data structure for a chat message event.
// It supports appending new messages as well as manipulating existing messages
// relative to a reference message ID (e.g., replace, insert before, insert after).
// It also supports the 'chat_message_posted' event type, which includes the ID
// of the message that was successfully inserted into the database.
type ChatMessagePayload struct {
	Text               string `json:"text"`
	Type               string `json:"type"`             // e.g., "regular"
	Visibility         string `json:"visibility"`       // e.g., "human-public", "human-only"
	NoConfirmation     bool   `json:"no_confirmation"`  // If true, bypass the UI confirmation modal
	ReferenceMessageID int64  `json:"reference_message_id,omitempty"` // The ID of the message to target for operations
	Replace            bool   `json:"replace,omitempty"`       // If true, replace the reference message
	InsertBefore       bool   `json:"insert_before,omitempty"` // If true, insert before the reference message
	InsertAfter        bool   `json:"insert_after,omitempty"`  // If true, insert after the reference message
	MessageID          int64  `json:"message_id,omitempty"`    // The ID of the message inserted by the backend (for 'chat_message_posted')
}

// GetEventsDBPath resolves the absolute path to the events database for a given contract UUID.
func GetEventsDBPath(uuid string) string {
	gscHome, _ := settings.GetGSCHome(false)
	return filepath.Join(gscHome, "data", "dumps", uuid, "events.sqlite3")
}

// InsertEvent inserts a new event into the contract_events table.
func InsertEvent(contractUUID string, chatID int64, eventType string, payload interface{}, source string, expiresAt time.Time) error {
	dbPath := GetEventsDBPath(contractUUID)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("events database not found for contract %s", contractUUID)
	}

	sqliteDB, err := db.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open events database: %w", err)
	}
	defer db.CloseDB(sqliteDB)

	return InsertEventWithDB(sqliteDB, chatID, eventType, payload, source, expiresAt)
}

// InsertEventWithDB inserts an event using an existing database connection.
func InsertEventWithDB(db *sql.DB, chatID int64, eventType string, payload interface{}, source string, expiresAt time.Time) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	query := `
		INSERT INTO contract_events (chat_id, event_type, payload, status, source, expires_at, created_at)
		VALUES (?, ?, ?, 'pending', ?, ?, ?)
	`

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	expiry := expiresAt.UTC().Format("2006-01-02T15:04:05.000Z")

	_, err = db.Exec(query, chatID, eventType, string(payloadJSON), source, expiry, now)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	logger.Debug("Event inserted", "type", eventType, "chat_id", chatID, "expires_at", expiry)
	return nil
}
