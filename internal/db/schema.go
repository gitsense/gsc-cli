/**
 * Component: Database Schema Definition
 * Block-UUID: 71a843e9-81b0-459f-9460-0f2eac3b171e
 * Parent-UUID: 94bda755-4554-4422-9680-a223ba23b6f8
 * Version: 1.6.0
 * Description: Defines the SQL schema for the GitSense Chat manifest SQLite database. Updated published_manifests table to support full manifest metadata including schema version, generated timestamp, manifest details, and content hash for duplicate detection.
 * Language: Go
 * Created-at: 2026-02-19T17:55:55.207Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0), GLM-4.7 (v1.6.0)
 */


package db

import (
	"database/sql"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/logger"
)

// CreateSchema creates all necessary tables in the database if they don't exist.
// This function should be called after opening a new database connection.
func CreateSchema(db *sql.DB) error {
	logger.Info("Creating database schema...")

	// Table 1: Manifest Information
	// Describes the database itself so agents know if it's relevant
	manifestInfoSQL := `
	CREATE TABLE IF NOT EXISTS manifest_info (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		tags TEXT,
		version TEXT,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		source_file TEXT
	);`

	// Table 2: Repositories
	// Tracks source repositories referenced in the manifest
	repositoriesSQL := `
	CREATE TABLE IF NOT EXISTS repositories (
		ref TEXT PRIMARY KEY,
		name TEXT NOT NULL
	);`

	// Table 3: Branches
	// Tracks git branches referenced in the manifest
	branchesSQL := `
	CREATE TABLE IF NOT EXISTS branches (
		ref TEXT PRIMARY KEY,
		name TEXT NOT NULL
	);`

	// Table 4: Core File Information
	// Note: 'language' column is included here for new databases.
	// An ALTER TABLE statement below ensures it exists for older databases.
	filesSQL := `
	CREATE TABLE IF NOT EXISTS files (
		file_path TEXT PRIMARY KEY,
		chat_id INTEGER NOT NULL UNIQUE,
		language TEXT,
		file_size_bytes INTEGER,
		last_committed TIMESTAMP,
		last_analyzed TIMESTAMP,
		git_hash TEXT,
		is_stale BOOLEAN DEFAULT 0
	);`

	// Table 5: Analyzer Definitions
	analyzersSQL := `
	CREATE TABLE IF NOT EXISTS analyzers (
		analyzer_id TEXT PRIMARY KEY,
		analyzer_ref_id TEXT NOT NULL UNIQUE,
		analyzer_name TEXT,
		analyzer_description TEXT,
		analyzer_version TEXT,
		batch_id TEXT,
		created_at TIMESTAMP
	);`

	// Table 6: Metadata Field Definitions
	metadataFieldsSQL := `
	CREATE TABLE IF NOT EXISTS metadata_fields (
		field_id TEXT PRIMARY KEY,
		field_ref_id TEXT NOT NULL,
		analyzer_id TEXT NOT NULL,
		field_name TEXT NOT NULL,
		field_display_name TEXT,
		field_type TEXT,
		field_description TEXT,
		UNIQUE(analyzer_id, field_name),
		FOREIGN KEY (analyzer_id) REFERENCES analyzers(analyzer_id)
	);`

	// Table 7: Analysis Results
	fileMetadataSQL := `
	CREATE TABLE IF NOT EXISTS file_metadata (
		file_path TEXT NOT NULL,
		field_id TEXT NOT NULL,
		field_value TEXT,
		analysis_confidence REAL DEFAULT 1.0,
		PRIMARY KEY (file_path, field_id),
		FOREIGN KEY (file_path) REFERENCES files(file_path),
		FOREIGN KEY (field_id) REFERENCES metadata_fields(field_id)
	);`

	// Execute table creation
	tables := []string{manifestInfoSQL, repositoriesSQL, branchesSQL, filesSQL, analyzersSQL, metadataFieldsSQL, fileMetadataSQL}
	for _, tableSQL := range tables {
		if _, err := db.Exec(tableSQL); err != nil {
			logger.Error("Failed to create table", "error", err)
			return err
		}
	}

	// Backwards Compatibility: Ensure 'language' column exists
	// This handles databases created before this version.
	// We ignore "duplicate column name" errors as they indicate the column already exists.
	alterLanguageSQL := `ALTER TABLE files ADD COLUMN language TEXT`
	if _, err := db.Exec(alterLanguageSQL); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			logger.Warning("Failed to add language column (may already exist)", "error", err)
		}
	}

	// Backwards Compatibility: Ensure 'updated_at' column exists in manifest_info
	alterUpdatedSQL := `ALTER TABLE manifest_info ADD COLUMN updated_at TIMESTAMP`
	if _, err := db.Exec(alterUpdatedSQL); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			logger.Warning("Failed to add updated_at column (may already exist)", "error", err)
		}
	}

	// Create Indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_metadata_file ON file_metadata(file_path);",
		"CREATE INDEX IF NOT EXISTS idx_metadata_field ON file_metadata(field_id);",
		"CREATE INDEX IF NOT EXISTS idx_analyzer_ref ON analyzers(analyzer_ref_id);",
		"CREATE INDEX IF NOT EXISTS idx_field_ref ON metadata_fields(field_ref_id);",
		"CREATE INDEX IF NOT EXISTS idx_files_language ON files(language);",
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			logger.Error("Failed to create index", "error", err)
			return err
		}
	}

	logger.Success("Database schema created successfully")
	return nil
}

// CreateStatsSchema creates the search_history table and indexes for the stats database.
// This is separate from the manifest schema to avoid bloating the manifest DB.
func CreateStatsSchema(db *sql.DB) error {
	logger.Info("Creating stats database schema...")

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
		analyzed_filter TEXT
	);`

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}

	// Create Indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_search_history_timestamp ON search_history(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_search_history_pattern ON search_history(pattern);",
		"CREATE INDEX IF NOT EXISTS idx_search_history_tool ON search_history(tool_name);",
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}

	logger.Success("Stats database schema created successfully")
	return nil
}

// CreatePublishedManifestsTable creates the table that tracks all published intelligence layers
// and their associated chat UI components in the GitSense Chat database.
func CreatePublishedManifestsTable(db *sql.DB) error {
	logger.Info("Creating published_manifests table...")

	query := `
	CREATE TABLE IF NOT EXISTS published_manifests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT NOT NULL UNIQUE,
		owner TEXT NOT NULL,
		repo TEXT NOT NULL,
		branch TEXT NOT NULL,
		database TEXT NOT NULL,
		schema_version TEXT,
		generated_at TIMESTAMP,
		manifest_name TEXT,
		manifest_description TEXT,
		manifest_tags TEXT,
		repositories TEXT,
		branches TEXT,
		hash TEXT NOT NULL,
		published_at TIMESTAMP NOT NULL,
		deleted INTEGER DEFAULT 0,
		root_chat_id INTEGER,
		owner_chat_id INTEGER,
		repo_chat_id INTEGER,
		FOREIGN KEY (root_chat_id) REFERENCES chats(id),
		FOREIGN KEY (owner_chat_id) REFERENCES chats(id),
		FOREIGN KEY (repo_chat_id) REFERENCES chats(id)
	);`

	if _, err := db.Exec(query); err != nil {
		logger.Error("Failed to create published_manifests table", "error", err)
		return err
	}

	logger.Success("Published manifests table created successfully")
	return nil
}
