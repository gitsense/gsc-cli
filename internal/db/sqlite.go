/*
 * Component: SQLite Database Handler
 * Block-UUID: 29650eea-03f5-43b3-bd44-041e5cdff755
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Handles SQLite database connections using modernc.org/sqlite for pure Go, CGO-free execution.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"github.com/yourusername/gsc-cli/pkg/logger"
)

// OpenDB opens a SQLite database connection with optimized settings for the CLI.
// It enables foreign keys and sets a busy timeout to handle concurrent access gracefully.
func OpenDB(dbPath string) (*sql.DB, error) {
	// Connection string parameters:
	// _pragma=foreign_keys(1): Enforce foreign key constraints
	// _pragma=journal_mode(WAL): Enable Write-Ahead Logging for better concurrency
	// _timeout=5000: Wait 5 seconds if the database is locked
	connStr := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_timeout=5000", dbPath)

	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		logger.Error("Failed to open database", "path", dbPath, "error", err)
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	// Verify the connection works
	if err := db.Ping(); err != nil {
		logger.Error("Failed to ping database", "path", dbPath, "error", err)
		return nil, fmt.Errorf("failed to ping database at %s: %w", dbPath, err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite works best with a single connection
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)

	logger.Success("Database connection established", "path", dbPath)
	return db, nil
}

// CloseDB closes the database connection gracefully.
func CloseDB(db *sql.DB) error {
	if db == nil {
		return nil
	}

	if err := db.Close(); err != nil {
		logger.Error("Failed to close database", "error", err)
		return fmt.Errorf("failed to close database: %w", err)
	}

	logger.Info("Database connection closed")
	return nil
}
