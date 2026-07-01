/**
 * Component: SQLite Database Handler
 * Block-UUID: 4416f7cd-d48d-4c25-bfcb-4bc15fd8c249
 * Parent-UUID: 6814805c-5bcb-4732-a017-6c50e93eaebe
 * Version: 1.6.0
 * Description: Handles SQLite database connections using modernc.org/sqlite for pure Go, CGO-free execution.
 * Language: Go
 * Created-at: 2026-04-02T00:02:51.146Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), claude-haiku-4-5-20251001 (v1.2.0), claude-haiku-4-5-20251001 (v1.3.0), claude-haiku-4-5-20251001 (v1.4.0), claude-haiku-4-5-20251001 (v1.5.0), GLM-4.7 (v1.6.0)
 */

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/git"

	_ "modernc.org/sqlite" // Pure Go SQLite driver

	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// OpenDB opens a SQLite database connection with optimized settings for the CLI.
// It enables foreign keys and sets a busy timeout to handle concurrent access gracefully.
func OpenDB(dbPath string) (*sql.DB, error) {
	// Connection string parameters:
	// _pragma=foreign_keys(1): Enforce foreign key constraints
	// _pragma=journal_mode(WAL): Enable Write-Ahead Logging for better concurrency
	// _pragma=busy_timeout(5000): Wait 5 seconds if the database is locked
	connStr := fmt.Sprintf("%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	return openSQLiteConnection(dbPath, connStr)
}

// OpenReadOnlyDB opens a SQLite database connection for read-only query paths.
// It avoids journal_mode changes so concurrent gsc rg/query/tree readers do not
// contend while applying write-oriented PRAGMAs.
func OpenReadOnlyDB(dbPath string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(dbPath)}
	connStr := u.String() + "?mode=ro&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	return openSQLiteConnection(dbPath, connStr)
}

func openSQLiteConnection(dbPath, connStr string) (*sql.DB, error) {
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

// ResolveDBPath determines the correct database path based on the Docker context.
// If a Docker context is active, it prioritizes the Docker data volume.
// Otherwise, it returns the provided native path.
func ResolveDBPath(nativePath string) string {
	// 1. Check for Docker Context
	dctx, err := docker_internal.LoadContext()
	if err == nil && dctx != nil {
		// Context exists: Use Docker data path
		// The database is always named 'chats.sqlite3' in the data directory
		return filepath.Join(dctx.DataHostPath, "chats.sqlite3")
	}

	// 2. No Context: Use Native Path
	return nativePath
}

// ValidateDBExists checks if the database file exists on disk before attempting a connection.
// This prevents the SQLite driver from creating an empty file artifact if the database is missing.
func ValidateDBExists(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s", dbPath)
	}
	return nil
}

// ResolveManifestDBPath constructs the absolute path to a manifest database file within the .gitsense directory.
// It finds the project root and appends the database name with a .db extension.
func ResolveManifestDBPath(dbName string) (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	dbPath := filepath.Join(root, ".gitsense", dbName+".db")
	return dbPath, nil
}

// GetFieldTypes retrieves a map of field names to their types for the specified database.
// This is used by the filter parser to determine how to handle operators (e.g., "=" for lists vs scalars).
func GetFieldTypes(ctx context.Context, dbName string) (map[string]string, error) {
	// 1. Resolve Database Path FIRST
	dbPath, err := ResolveManifestDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 2. THEN Validate Database Exists
	if err := ValidateDBExists(dbPath); err != nil {
		return nil, err
	}

	// 3. Open Database Connection
	database, err := OpenReadOnlyDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer CloseDB(database)

	return GetFieldTypesFromDB(ctx, database)
}

// GetFieldTypesFromDB retrieves field types using an existing database handle.
func GetFieldTypesFromDB(ctx context.Context, database *sql.DB) (map[string]string, error) {
	query := `
		SELECT field_name, field_type
		FROM metadata_fields
		ORDER BY field_name
	`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fieldTypes := make(map[string]string)
	for rows.Next() {
		var name, fieldType string
		if err := rows.Scan(&name, &fieldType); err != nil {
			return nil, err
		}
		fieldTypes[name] = fieldType
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fieldTypes, nil
}
