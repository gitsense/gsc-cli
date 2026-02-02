/*
 * Component: Manifest Importer
 * Block-UUID: 8f00e226-bc54-4646-9416-8adec3b7554f
 * Parent-UUID: da2e9e09-a2b3-4009-aa5a-135f0df5b73e
 * Version: 1.1.0
 * Description: Logic to parse a JSON manifest file and import its data into a SQLite database.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0)
 */


package manifest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// ImportManifest reads a JSON manifest file and imports it into the specified SQLite database.
func ImportManifest(ctx context.Context, jsonPath string, dbName string) error {
	logger.Info("Starting import from %s to database %s...", jsonPath, dbName)

	// 1. Read and Parse JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifestFile ManifestFile
	if err := json.Unmarshal(data, &manifestFile); err != nil {
		return fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	// 2. Validate Manifest
	if err := ValidateManifest(&manifestFile); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	// 3. Resolve Database Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	// 4. Open Database Connection
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseDB(database)

	// 5. Create Schema (if not exists)
	if err := db.CreateSchema(database); err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	// 6. Begin Transaction
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 7. Insert Manifest Info
	if err := insertManifestInfo(tx, &manifestFile, dbPath); err != nil {
		return err
	}

	// 8. Insert Reference Data (Repositories, Branches, Analyzers, Fields)
	if err := insertReferenceData(tx, &manifestFile); err != nil {
		return err
	}

	// 9. Insert File Data and Metadata
	if err := insertFileData(ctx, tx, &manifestFile); err != nil {
		return err
	}

	// 10. Commit Transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 11. Update Registry
	entry := registry.RegistryEntry{
		Name:        manifestFile.Manifest.Name,
		Description: manifestFile.Manifest.Description,
		Tags:        manifestFile.Manifest.Tags,
		Version:     manifestFile.SchemaVersion,
		CreatedAt:   manifestFile.GeneratedAt,
		SourceFile:  jsonPath,
	}

	if err := registry.AddEntry(entry); err != nil {
		logger.Warning("Failed to update registry: %v", err)
		// Non-fatal error, data is imported
	}

	logger.Success("Successfully imported manifest '%s' into database '%s'", manifestFile.Manifest.Name, dbName)
	return nil
}

// insertManifestInfo inserts the top-level manifest metadata
func insertManifestInfo(tx *sql.Tx, manifestFile *ManifestFile, sourceFile string) error {
	query := `
		INSERT INTO manifest_info (name, description, tags, version, created_at, source_file)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	tagsJSON, _ := json.Marshal(manifestFile.Manifest.Tags)

	_, err := tx.Exec(query,
		manifestFile.Manifest.Name,
		manifestFile.Manifest.Description,
		string(tagsJSON),
		manifestFile.SchemaVersion,
		manifestFile.GeneratedAt,
		sourceFile,
	)
	return err
}

// insertReferenceData inserts repositories, branches, analyzers, and field definitions
func insertReferenceData(tx *sql.Tx, manifestFile *ManifestFile) error {
	// Insert Repositories
	for _, repo := range manifestFile.Repositories {
		query := `INSERT OR REPLACE INTO repositories (ref, name) VALUES (?, ?)`
		if _, err := tx.Exec(query, repo.Ref, repo.Name); err != nil {
			return fmt.Errorf("failed to insert repository %s: %w", repo.Ref, err)
		}
	}

	// Insert Branches
	for _, branch := range manifestFile.Branches {
		query := `INSERT OR REPLACE INTO branches (ref, name) VALUES (?, ?)`
		if _, err := tx.Exec(query, branch.Ref, branch.Name); err != nil {
			return fmt.Errorf("failed to insert branch %s: %w", branch.Ref, err)
		}
	}

	// Insert Analyzers
	for _, analyzer := range manifestFile.Analyzers {
		query := `
			INSERT OR REPLACE INTO analyzers (analyzer_id, analyzer_ref_id, analyzer_name, analyzer_description, analyzer_version, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		if _, err := tx.Exec(query,
			analyzer.ID,
			analyzer.Ref,
			analyzer.Name,
			analyzer.Description,
			analyzer.Version,
			manifestFile.GeneratedAt,
		); err != nil {
			return fmt.Errorf("failed to insert analyzer %s: %w", analyzer.Ref, err)
		}
	}

	// Insert Metadata Fields
	for _, field := range manifestFile.Fields {
		// Use field.Ref as the field_id for simplicity and consistency
		query := `
			INSERT OR REPLACE INTO metadata_fields (field_id, field_ref_id, analyzer_id, field_name, field_display_name, field_type, field_description)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		if _, err := tx.Exec(query,
			field.Ref,
			field.Ref,
			field.AnalyzerRef,
			field.Name,
			field.DisplayName,
			field.Type,
			field.Description,
		); err != nil {
			return fmt.Errorf("failed to insert field %s: %w", field.Ref, err)
		}
	}

	return nil
}

// insertFileData inserts file records and their associated metadata values
func insertFileData(ctx context.Context, tx *sql.Tx, manifestFile *ManifestFile) error {
	// Prepare statement for file insertion
	fileStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO files (file_path, chat_id, language, last_analyzed)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer fileStmt.Close()

	// Prepare statement for metadata insertion
	metaStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO file_metadata (file_path, field_id, field_value)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer metaStmt.Close()

	for _, dataRow := range manifestFile.Data {
		// Insert File
		if _, err := fileStmt.ExecContext(ctx,
			dataRow.FilePath,
			dataRow.ChatID,
			dataRow.Language,
			manifestFile.GeneratedAt,
		); err != nil {
			return fmt.Errorf("failed to insert file %s: %w", dataRow.FilePath, err)
		}

		// Insert Metadata Fields
		for fieldRef, value := range dataRow.Fields {
			// Convert value to string for storage
			valueStr := fmt.Sprintf("%v", value)

			if _, err := metaStmt.ExecContext(ctx, dataRow.FilePath, fieldRef, valueStr); err != nil {
				return fmt.Errorf("failed to insert metadata for %s: %w", dataRow.FilePath, err)
			}
		}
	}

	return nil
}
