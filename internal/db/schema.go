/*
 * Component: Database Schema Definition
 * Block-UUID: 8e0becf6-a27b-492a-b50d-8309946458e4
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the SQL schema for the GitSense manifest SQLite database, including tables for manifest info, files, analyzers, fields, and metadata.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package db

import (
	"database/sql"

	"github.com/yourusername/gsc-cli/pkg/logger"
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
		source_file TEXT
	);`

	// Table 2: Core File Information
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

	// Table 3: Analyzer Definitions
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

	// Table 4: Metadata Field Definitions
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

	// Table 5: Analysis Results
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
	tables := []string{manifestInfoSQL, filesSQL, analyzersSQL, metadataFieldsSQL, fileMetadataSQL}
	for _, tableSQL := range tables {
		if _, err := db.Exec(tableSQL); err != nil {
			logger.Error("Failed to create table", "error", err)
			return err
		}
	}

	// Create Indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_metadata_file ON file_metadata(file_path);",
		"CREATE INDEX IF NOT EXISTS idx_metadata_field ON file_metadata(field_id);",
		"CREATE INDEX IF NOT EXISTS idx_analyzer_ref ON analyzers(analyzer_ref_id);",
		"CREATE INDEX IF NOT EXISTS idx_field_ref ON metadata_fields(field_ref_id);",
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
