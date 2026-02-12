/**
 * Component: Search Statistics Recorder
 * Block-UUID: 240f6828-d42a-4cd2-9f53-905e9b238338
 * Parent-UUID: 7dfc80eb-66b8-43b3-b93c-b39aa4b017d0
 * Version: 1.3.0
 * Description: Records search execution details to a local SQLite database for analytics and Scout intelligence. Updated timestamp generation to use UTC ISO 8601 format with 3-digit milliseconds to match JavaScript's toISOString().
 * Language: Go
 * Created-at: 2026-02-05T20:12:15.422Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0)
 */


package search

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

// RecordSearch saves the search execution details to the stats database.
func RecordSearch(ctx context.Context, searchInfo SearchRecord) error {
	// 1. Resolve Stats DB Path
	dbPath, err := resolveStatsDBPath()
	if err != nil {
		logger.Warning("Failed to resolve stats DB path, skipping stats recording", "error", err)
		return nil // Don't fail the search if stats fail
	}

	// 2. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		logger.Warning("Failed to open stats DB, skipping stats recording", "error", err)
		return nil
	}
	defer db.CloseDB(database)

	// 3. Ensure Schema Exists
	if err := ensureStatsSchema(database); err != nil {
		logger.Warning("Failed to ensure stats schema, skipping stats recording", "error", err)
		return nil
	}

	// 4. Insert Record
	query := `
		INSERT INTO search_history (
			timestamp, pattern, tool_name, tool_version, duration_ms,
			total_matches, total_files, analyzed_files, filters_used,
			database_name, case_sensitive, file_filters, analyzed_filter, requested_fields
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// Format time to match JavaScript's toISOString(): UTC, ISO 8601, 3-digit milliseconds, 'Z' suffix
	timestamp := searchInfo.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z")

	_, err = database.ExecContext(ctx, query,
		timestamp,
		searchInfo.Pattern,
		searchInfo.ToolName,
		searchInfo.ToolVersion,
		searchInfo.DurationMs,
		searchInfo.TotalMatches,
		searchInfo.TotalFiles,
		searchInfo.AnalyzedFiles,
		searchInfo.FiltersUsed,
		searchInfo.DatabaseName,
		searchInfo.CaseSensitive,
		searchInfo.FileFilters,
		searchInfo.AnalyzedFilter,
		searchInfo.RequestedFields,
	)

	if err != nil {
		logger.Warning("Failed to insert search record", "error", err)
		return nil
	}

	logger.Debug("Search stats recorded")
	return nil
}

// resolveStatsDBPath constructs the absolute path to the stats database.
func resolveStatsDBPath() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	// Ensure .gitsense directory exists
	gitsenseDir := filepath.Join(root, settings.GitSenseDir)
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .gitsense directory: %w", err)
	}

	dbPath := filepath.Join(gitsenseDir, "stats.db")
	return dbPath, nil
}

// ensureStatsSchema creates the search_history table and indexes if they don't exist.
func ensureStatsSchema(database *sql.DB) error {
	// Create Table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS search_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP,
		pattern TEXT,
		tool_name TEXT,
		tool_version TEXT,
		duration_ms INTEGER,
		total_matches INTEGER,
		total_files INTEGER,
		analyzed_files INTEGER,
		filters_used TEXT,
		database_name TEXT,
		case_sensitive BOOLEAN,
		file_filters TEXT,
		analyzed_filter TEXT,
		requested_fields TEXT
	);`

	if _, err := database.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create search_history table: %w", err)
	}

	// Create Indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_search_history_timestamp ON search_history(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_search_history_pattern ON search_history(pattern);",
		"CREATE INDEX IF NOT EXISTS idx_search_history_tool ON search_history(tool_name);",
	}

	for _, indexSQL := range indexes {
		if _, err := database.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}
