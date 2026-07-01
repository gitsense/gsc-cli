/**
 * Component: Pi Sessions Database
 * Block-UUID: 5d20850f-7d10-4128-b086-bcf030b0d7bd
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Creates and resets the SQLite mirror used by gsc pi sessions.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/db"
)

//go:embed schema.sql
var schemaFS embed.FS

func openMirror(ctx context.Context, dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := database.ExecContext(ctx, "PRAGMA secure_delete = ON"); err != nil {
		db.CloseDB(database)
		return nil, err
	}
	if err := createSchema(ctx, database); err != nil {
		db.CloseDB(database)
		return nil, err
	}
	return database, nil
}

// OpenQueryMirror opens a read-only connection to the Pi sessions database.
// Used by CLI commands that need to query without modifying.
func OpenQueryMirror(dbPath string) (*sql.DB, error) {
	if err := db.ValidateDBExists(dbPath); err != nil {
		return nil, err
	}
	connStr := fmt.Sprintf("%s?_pragma=busy_timeout(5000)", dbPath)
	database, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("failed to ping database at %s: %w", dbPath, err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	return database, nil
}

func createSchema(ctx context.Context, database *sql.DB) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, string(schema)); err != nil {
		return fmt.Errorf("create pi sessions schema: %w", err)
	}
	return nil
}

func ResetDatabase(dbPath string) error {
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
