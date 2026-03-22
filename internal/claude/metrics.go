/**
 * Component: Claude Code Metrics Database
 * Block-UUID: 2b2d6072-e280-461a-8822-66cdc33d2e8c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Manages the SQLite database for storing Claude Code CLI usage metrics, including completions and session aggregations.
 * Language: Go
 * Created-at: 2026-03-22T03:44:15.678Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package claude

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// OpenMetricsDB opens or creates the Claude Code metrics database.
func OpenMetricsDB() (*sql.DB, error) {
	gscHome, err := settings.GetGSCHome(true)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	dbPath := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeMetricsDBName)
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	// Connection string with optimizations
	connStr := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_timeout=5000", dbPath)
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping metrics database: %w", err)
	}

	// Initialize Schema
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the necessary tables if they don't exist.
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS completions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_uuid TEXT NOT NULL,
		message_id INTEGER,
		claude_session_id TEXT,
		model TEXT NOT NULL,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		cache_creation_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		duration_ms INTEGER,
		raw_json TEXT,
		exit_code INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		claude_session_id TEXT PRIMARY KEY,
		chat_uuid TEXT NOT NULL,
		first_request_at TIMESTAMP,
		last_request_at TIMESTAMP,
		total_requests INTEGER DEFAULT 0,
		total_input_tokens INTEGER DEFAULT 0,
		total_output_tokens INTEGER DEFAULT 0,
		total_cost_usd REAL DEFAULT 0.0
	);

	CREATE INDEX IF NOT EXISTS idx_completions_chat_uuid ON completions(chat_uuid);
	CREATE INDEX IF NOT EXISTS idx_completions_session_id ON completions(claude_session_id);
	`

	_, err := db.Exec(schema)
	return err
}

// InsertCompletion saves a single completion record to the database.
func InsertCompletion(db *sql.DB, chatUUID string, messageID int64, sessionID string, model string, usage Usage, cost float64, durationMs int, rawJSON string, exitCode int) error {
	query := `
	INSERT INTO completions (
		chat_uuid, message_id, claude_session_id, model,
		input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		cost_usd, duration_ms, raw_json, exit_code
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := db.Exec(
		query,
		chatUUID, messageID, sessionID, model,
		usage.InputTokens, usage.OutputTokens, usage.CacheCreationTokens, usage.CacheReadTokens,
		cost, durationMs, rawJSON, exitCode,
	)

	if err != nil {
		return fmt.Errorf("failed to insert completion: %w", err)
	}

	logger.Debug("Completion metrics saved", "session_id", sessionID, "cost", cost)
	return nil
}

// UpsertSession updates or creates a session aggregate record.
func UpsertSession(db *sql.DB, sessionID string, chatUUID string, usage Usage, cost float64) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	query := `
	INSERT INTO sessions (
		claude_session_id, chat_uuid, first_request_at, last_request_at,
		total_requests, total_input_tokens, total_output_tokens, total_cost_usd
	) VALUES (?, ?, ?, ?, 1, ?, ?, ?)
	ON CONFLICT(claude_session_id) DO UPDATE SET
		last_request_at = excluded.last_request_at,
		total_requests = total_requests + 1,
		total_input_tokens = total_input_tokens + excluded.total_input_tokens,
		total_output_tokens = total_output_tokens + excluded.total_output_tokens,
		total_cost_usd = total_cost_usd + excluded.total_cost_usd
	`

	_, err := db.Exec(
		query,
		sessionID, chatUUID, now, now,
		usage.InputTokens, usage.OutputTokens, cost,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert session: %w", err)
	}

	return nil
}
