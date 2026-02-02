/*
 * Component: Manifest Importer
 * Block-UUID: da2e9e09-a2b3-4009-aa5a-135f0df5b73e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Logic to parse a JSON manifest file and import its data into a SQLite database.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// ImportManifest reads a JSON manifest file and imports it into the specified SQLite database.
func ImportManifest(ctx context.Context, jsonPath string, dbName string) error {
	logger.Info(fmt.Sprintf("Starting import from %s to database %s...", jsonPath, dbName))

	// 1. Read and Parse JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest models.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	// 2. Validate Manifest
	if err := validator.ValidateManifest(&manifest); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	// 3. Resolve Database Path
	dbPath, err := path_helper.ResolveDBPath(dbName)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	// 4. Open Database Connection
	db, err := sqlite.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer sqlite.CloseDB(db)

	// 5. Create Schema (if not exists)
	if err := sqlite.CreateSchema(db); err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	// 6. Begin Transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 7. Insert Manifest Info
	if err := insertManifestInfo(tx, &manifest, dbPath); err != nil {
		return err
	}

	// 8. Insert Reference Data (Repositories, Branches, Analyzers, Fields)
	if err := insertReferenceData(tx, &manifest); err != nil {
		return err
	}

	// 9. Insert File Data and Metadata
	if err := insertFileData(ctx, tx, &manifest); err != nil {
		return err
	}

	// 10. Commit Transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 11. Update Registry
	registryPath, err := path_helper.ResolveRegistryPath()
	if err != nil {
		return fmt.Errorf("failed to resolve registry path: %w", err)
	}

	entry := registry.RegistryEntry{
		Name:        manifest.Manifest.Name,
		Database:    dbName,
		Description: manifest.Manifest.Description,
		Tags:        manifest.Manifest.Tags,
		CreatedAt:   manifest.GeneratedAt,
	}

	if err := registry.AddEntry(registryPath, entry); err != nil {
		logger.Error(fmt.Sprintf("Warning: failed to update registry: %v", err))
		// Non-fatal error, data is imported
	}

	logger.Success(fmt.Sprintf("Successfully imported manifest '%s' into database '%s'", manifest.Manifest.Name, dbName))
	return nil
}

// insertManifestInfo inserts the top-level manifest metadata
func insertManifestInfo(tx *sql.Tx, manifest *models.Manifest, sourceFile string) error {
	query := `
		INSERT INTO manifest_info (name, description, tags, version, created_at, source_file)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	
	tagsJSON, _ := json.Marshal(manifest.Manifest.Tags)
	
	_, err := tx.Exec(query, 
		manifest.Manifest.Name,
		manifest.Manifest.Description,
		string(tagsJSON),
		manifest.SchemaVersion,
		manifest.GeneratedAt,
		sourceFile,
	)
	return err
}

// insertReferenceData inserts repositories, branches, analyzers, and field definitions
func insertReferenceData(tx *sql.Tx, manifest *models.Manifest) error {
	// Insert Repositories
	for _, repo := range manifest.Repositories {
		query := `INSERT OR REPLACE INTO repositories (ref, name) VALUES (?, ?)`
		if _, err := tx.Exec(query, repo.Ref, repo.Name); err != nil {
			return fmt.Errorf("failed to insert repository %s: %w", repo.Ref, err)
		}
	}

	// Insert Branches
	for _, branch := range manifest.Branches {
		query := `INSERT OR REPLACE INTO branches (ref, name) VALUES (?, ?)`
		if _, err := tx.Exec(query, branch.Ref, branch.Name); err != nil {
			return fmt.Errorf("failed to insert branch %s: %w", branch.Ref, err)
		}
	}

	// Insert Analyzers
	for _, analyzer := range manifest.Analyzers {
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
			manifest.GeneratedAt,
		); err != nil {
			return fmt.Errorf("failed to insert analyzer %s: %w", analyzer.Ref, err)
		}
	}

	// Insert Metadata Fields
	for _, field := range manifest.Fields {
		// Composite ID: field_name + analyzer_ref
		fieldID := fmt.Sprintf("%s_%s", field.Name, field.AnalyzerRef)
		
		query := `
			INSERT OR REPLACE INTO metadata_fields (field_id, field_ref_id, analyzer_id, field_name, field_display_name, field_type, field_description)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		if _, err := tx.Exec(query, 
			fieldID, 
			field.Ref, 
			field.AnalyzerRef, // Note: In JSON this is ref, in DB we might want full ID or ref. Assuming ref maps to analyzer_ref_id in DB
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
func insertFileData(ctx context.Context, tx *sql.Tx, manifest *models.Manifest) error {
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

	for _, dataRow := range manifest.Data {
		// Insert File
		if _, err := fileStmt.ExecContext(ctx, 
			dataRow.FilePath, 
			dataRow.ChatID, 
			dataRow.Language, 
			manifest.GeneratedAt,
		); err != nil {
			return fmt.Errorf("failed to insert file %s: %w", dataRow.FilePath, err)
		}

		// Insert Metadata Fields
		for fieldRef, value := range dataRow.Fields {
			// We need to find the full field_id based on the fieldRef (e.g., "F0")
			// We have to look this up from the manifest.Fields list
			var fieldDef *models.Field
			for i := range manifest.Fields {
				if manifest.Fields[i].Ref == fieldRef {
					fieldDef = &manifest.Fields[i]
					break
				}
			}

			if fieldDef == nil {
				logger.Error(fmt.Sprintf("Warning: Field ref %s not found in definition for file %s", fieldRef, dataRow.FilePath))
				continue
			}

			// Construct composite field_id
			fieldID := fmt.Sprintf("%s_%s", fieldDef.Name, fieldDef.AnalyzerRef)
			
			// Convert value to string for storage
			valueStr := fmt.Sprintf("%v", value)

			if _, err := metaStmt.ExecContext(ctx, dataRow.FilePath, fieldID, valueStr); err != nil {
				return fmt.Errorf("failed to insert metadata for %s: %w", dataRow.FilePath, err)
			}
		}
	}

	return nil
}
