/*
 * Component: Contract Events Database Helper
 * Block-UUID: f9eca414-119d-4973-a4e0-2fe08fda2aae
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides helper functions to interact with the contract-level events SQLite database, enabling messaging between the terminal and the web UI.
 * Language: Go
 * Created-at: 2026-03-07T03:31:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
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
type ChatMessagePayload struct {
	Text       string `json:"text"`
	Type       string `json:"type"`       // e.g., "regular"
	Visibility string `json:"visibility"` // e.g., "human-public", "human-only"
}

// GetEventsDBPath resolves the absolute path to the events database for a given contract UUID.
func GetEventsDBPath(uuid string) string {
	gscHome, _ := settings.GetGSCHome(false)
	return filepath.Join(gscHome, "data", "dumps", uuid, "events.sqlite3")
}

// InsertEvent inserts a new event into the contract_events table.
// It handles opening and closing the database connection internally.
func InsertEvent(contractUUID string, eventType string, payload interface{}, source string) error {
	dbPath := GetEventsDBPath(contractUUID)

	// Ensure the database exists (it should be created by initEventsDB in manager.go)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("events database not found for contract %s. Please ensure the contract is active.", contractUUID)
	}

	sqliteDB, err := db.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open events database: %w", err)
	}
	defer db.CloseDB(sqliteDB)

	return InsertEventWithDB(sqliteDB, eventType, payload, source)
}

// InsertEventWithDB inserts an event using an existing database connection.
// This is useful for batch operations or when the connection is already managed.
func InsertEventWithDB(db *sql.DB, eventType string, payload interface{}, source string) error {
	// Marshal payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Prepare SQL
	query := `
		INSERT INTO contract_events (event_type, payload, status, source, created_at)
		VALUES (?, ?, 'pending', ?, ?)
	`

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	// Execute Insert
	_, err = db.Exec(query, eventType, string(payloadJSON), source, now)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	logger.Debug("Event inserted", "type", eventType, "source", source)
	return nil
}
