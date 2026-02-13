/**
 * Component: Manifest Importer
 * Block-UUID: 8a4d2b1c-9e3f-4a5d-8b6c-7d8e9f0a1b2c
 * Parent-UUID: 785d85d4-e9fb-4d3c-a01b-e628eb6562a7
 * Version: 1.13.0
 * Description: Logic to parse a JSON manifest file and import its data into a SQLite database.
 * Language: Go
 * Created-at: 2026-02-11T03:20:06.219Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.1.1), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.6.1), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), Gemini 3 Flash (v1.10.0), Gemini 3 Flash (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0)
 */


package manifest

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/src-d/enry/v2"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ImportManifest reads a JSON manifest file and imports it into the specified SQLite database.
// It implements an atomic swap strategy: imports to a temp file, backs up the existing DB (if any),
// and then renames the temp file to the final name.
func ImportManifest(ctx context.Context, jsonPath string, dbName string, force bool, noBackup bool) error {
	logger.Debug("Starting import", "path", jsonPath)

	// 1. Acquire Lock
	// Prevents concurrent imports from corrupting the registry or database files.
	lockPath, err := ResolveLockPath()
	if err != nil {
		return fmt.Errorf("failed to resolve lock path: %w", err)
	}

	lockFile, err := os.Create(lockPath)
	if err != nil {
		return fmt.Errorf("another import is already in progress (lock file exists): %w", err)
	}
	defer os.Remove(lockPath)
	defer lockFile.Close()

	// 2. Read and Parse JSON
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifestFile ManifestFile
	if err := json.Unmarshal(data, &manifestFile); err != nil {
		return fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	// 3. Validate Manifest
	if err := ValidateManifest(&manifestFile); err != nil {
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	// 4. Resolve Database Name
	// Priority: CLI arg > JSON field > Filename
	if dbName == "" {
		if manifestFile.Manifest.DatabaseName != "" {
			dbName = manifestFile.Manifest.DatabaseName
			logger.Debug("Using database name from manifest", "db", dbName)
		} else {
			// Fallback to filename derivation
			base := filepath.Base(jsonPath)
			dbName = strings.TrimSuffix(base, filepath.Ext(base))
			logger.Debug("Using database name derived from filename", "db", dbName)
		}
	}

	// 5. Pre-flight Check (Registry)
	// Check if database exists in registry to prevent accidental overwrites
	if !force {
		reg, err := registry.LoadRegistry()
		if err != nil {
			return fmt.Errorf("failed to load registry for pre-flight check: %w", err)
		}
		if _, exists := reg.FindEntryByDBName(dbName); exists {
			return fmt.Errorf("database '%s' already exists. Use --force to overwrite", dbName)
		}
	}

	// 6. Resolve Temp Database Path
	tempPath, err := ResolveTempDBPath(dbName)
	if err != nil {
		return fmt.Errorf("failed to resolve temp database path: %w", err)
	}

	// Cleanup temp file on error
	defer func() {
		if err != nil {
			logger.Debug("Cleaning up temp file due to error", "path", tempPath)
			os.Remove(tempPath)
		}
	}()

	// 7. Open Database Connection (Temp)
	database, err := db.OpenDB(tempPath)
	if err != nil {
		return fmt.Errorf("failed to open temp database: %w", err)
	}
	defer db.CloseDB(database)

	// 8. Create Schema (if not exists)
	if err := db.CreateSchema(database); err != nil {
		return fmt.Errorf("failed to create database schema: %w", err)
	}

	// 9. Begin Transaction
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 10. Insert Manifest Info
	// Use current time for updated_at
	if err := insertManifestInfo(tx, &manifestFile, jsonPath, time.Now()); err != nil {
		return err
	}

	// 11. Insert Reference Data (Repositories, Branches, Analyzers, Fields)
	if err := insertReferenceData(tx, &manifestFile); err != nil {
		return err
	}

	// 12. Insert File Data and Metadata
	if err := insertFileData(ctx, tx, &manifestFile); err != nil {
		return err
	}

	// 13. Commit Transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// 14. Close DB explicitly before rename
	db.CloseDB(database)

	// 15. Backup Existing Database (if applicable)
	finalPath, err := ResolveDBPath(dbName)
	if err != nil {
		return fmt.Errorf("failed to resolve final database path: %w", err)
	}

	if !noBackup {
		if _, err := os.Stat(finalPath); err == nil {
			// File exists, perform backup
			logger.Debug("Backing up existing database", "db", dbName)
			if err := backupDatabase(dbName, finalPath); err != nil {
				// If backup fails and --no-backup was not specified, fail the import to prevent data loss
				return fmt.Errorf("backup failed and --no-backup was not specified: %w", err)
			}
		} else {
			logger.Debug("No existing database found, skipping backup", "db", dbName)
		}
	} else {
		logger.Debug("Skipping backup due to --no-backup flag", "db", dbName)
	}

	// 16. Atomic Swap
	logger.Debug("Performing atomic swap", "from", tempPath, "to", finalPath)
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("failed to swap database: %w", err)
	}

	// 17. Update Registry
	// Use the resolved dbName to support CLI overrides (--name flag).
	entry := registry.RegistryEntry{
		ManifestName : manifestFile.Manifest.ManifestName,
		DatabaseName:  dbName,
		Description:   manifestFile.Manifest.Description,
		Tags:          manifestFile.Manifest.Tags,
		Version:       manifestFile.SchemaVersion,
		CreatedAt:     manifestFile.GeneratedAt,
		UpdatedAt:     time.Now(),
		SourceFile:    jsonPath,
	}

	if err := registry.AddEntry(entry); err != nil {
		logger.Warning("Failed to update registry", "error", err)
		// Non-fatal error, data is imported
	}

	logger.Success("Successfully imported manifest", "manifest", manifestFile.Manifest.ManifestName, "db", dbName)
	return nil
}

// insertManifestInfo inserts the top-level manifest metadata
func insertManifestInfo(tx *sql.Tx, manifestFile *ManifestFile, sourceFile string, updatedAt time.Time) error {
	query := `
		INSERT INTO manifest_info (name, description, tags, version, created_at, updated_at, source_file)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	tagsJSON, _ := json.Marshal(manifestFile.Manifest.Tags)

	_, err := tx.Exec(query,
		manifestFile.Manifest.ManifestName,
		manifestFile.Manifest.Description,
		string(tagsJSON),
		manifestFile.SchemaVersion,
		manifestFile.GeneratedAt,
		updatedAt,
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

	// Build a map from analyzer ref to analyzer ID to resolve foreign keys
	refToID := make(map[string]string)
	for _, analyzer := range manifestFile.Analyzers {
		refToID[analyzer.Ref] = analyzer.ID
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
			refToID[field.AnalyzerRef],
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

	// Get project root for resolving file paths during language detection
	root, err := git.FindProjectRoot()
	if err != nil {
		logger.Warning("Failed to find project root for language detection", "error", err)
		// Continue without root, language detection will fail for missing languages
	}

	for _, dataRow := range manifestFile.Data {
		// Determine Language: Trust Upstream -> Detect with enry
		language := dataRow.Language
		if language == "" && root != "" {
			// Fallback detection
			fullPath := filepath.Join(root, dataRow.FilePath)
			detectedLang := detectLanguage(fullPath)
			if detectedLang != "" {
				language = detectedLang
				logger.Debug("Detected language", "file", dataRow.FilePath, "language", language)
			}
		}

		// Insert File
		if _, err := fileStmt.ExecContext(ctx,
			dataRow.FilePath,
			dataRow.ChatID,
			language,
			manifestFile.GeneratedAt,
		); err != nil {
			return fmt.Errorf("failed to insert file %s: %w", dataRow.FilePath, err)
		}

		// Insert Metadata Fields
		for fieldRef, value := range dataRow.Fields {
			var valueStr string

			// Check if value is a slice/array to store as JSON
			val := reflect.ValueOf(value)
			if val.Kind() == reflect.Slice || val.Kind() == reflect.Array {
				jsonData, err := json.Marshal(value)
				if err != nil {
					logger.Warning("Failed to marshal array field to JSON, falling back to string", "field", fieldRef, "error", err)
					valueStr = fmt.Sprintf("%v", value)
				} else {
					valueStr = string(jsonData)
				}
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}

			if _, err := metaStmt.ExecContext(ctx, dataRow.FilePath, fieldRef, valueStr); err != nil {
				return fmt.Errorf("failed to insert metadata for %s: %w", dataRow.FilePath, err)
			}
		}
	}

	return nil
}

// detectLanguage uses enry to detect the language of a file.
// It reads the file content to perform accurate detection.
func detectLanguage(filePath string) string {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		logger.Debug("Failed to read file for language detection", "file", filePath)
		return ""
	}

	// Use enry to detect language
	// enry.GetLanguage returns the language name or empty string if unknown
	lang := enry.GetLanguage(filePath, content)
	
	// enry returns "Text" for unknown text files, we might want to treat that as empty or keep it
	// depending on requirements. For now, we keep what enry returns.
	if lang == "" {
		return ""
	}
	
	return lang
}

// backupDatabase creates a compressed backup of the database and its registry metadata.
func backupDatabase(dbName string, dbPath string) error {
	backupDir, err := ResolveBackupDir()
	if err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102-150405")
	
	// 1. Backup Database File
	dbBackupName := fmt.Sprintf("%s.%s.db.gz", dbName, timestamp)
	dbBackupPath := filepath.Join(backupDir, dbBackupName)
	
	if err := compressFile(dbPath, dbBackupPath); err != nil {
		return fmt.Errorf("failed to compress database: %w", err)
	}
	logger.Debug("Database backed up", "path", dbBackupPath)

	// 2. Backup Registry Metadata
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry for metadata backup: %w", err)
	}

	entry, exists := reg.FindEntryByDBName(dbName)
	if !exists {
		// This shouldn't happen if we are backing up an existing DB, but handle gracefully
		logger.Warning("Could not find registry entry for backup", "db", dbName)
	} else {
		regBackupName := fmt.Sprintf("%s.%s.registry.json", dbName, timestamp)
		regBackupPath := filepath.Join(backupDir, regBackupName)
		
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal registry entry: %w", err)
		}
		
		if err := os.WriteFile(regBackupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write registry backup: %w", err)
		}
		logger.Debug("Registry metadata backed up", "path", regBackupPath)
	}

	// 3. Rotate Backups
	if err := rotateBackups(backupDir, dbName); err != nil {
		logger.Warning("Failed to rotate backups", "error", err)
	}

	return nil
}

// compressFile compresses a source file to a destination gzip file.
func compressFile(srcPath, destPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	gz := gzip.NewWriter(dest)
	defer gz.Close()

	if _, err := io.Copy(gz, src); err != nil {
		return err
	}

	return nil
}

// rotateBackups ensures only the most recent MaxBackups are kept.
func rotateBackups(backupDir, dbName string) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	// Filter files belonging to this database
	var files []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Match pattern: dbname.timestamp.db.gz or dbname.timestamp.registry.json
		// We only care about the DB files for rotation count, or we can count pairs.
		// Simplest is to count DB files.
		if strings.HasPrefix(entry.Name(), dbName+".") && strings.HasSuffix(entry.Name(), ".db.gz") {
			files = append(files, entry)
		}
	}

	// If we have more than MaxBackups, delete the oldest
	if len(files) > settings.MaxBackups {
		// Sort by modification time (oldest first)
		sort.Slice(files, func(i, j int) bool {
			infoI, _ := files[i].Info()
			infoJ, _ := files[j].Info()
			return infoI.ModTime().Before(infoJ.ModTime())
		})

		// Delete oldest files (and their corresponding registry json)
		toDelete := len(files) - settings.MaxBackups
		for i := 0; i < toDelete; i++ {
			oldFile := files[i]
			oldPath := filepath.Join(backupDir, oldFile.Name())
			
			// Delete DB backup
			if err := os.Remove(oldPath); err != nil {
				logger.Warning("Failed to delete old backup", "path", oldPath, "error", err)
			}

			// Delete corresponding registry backup
			regName := strings.TrimSuffix(oldFile.Name(), ".db.gz") + ".registry.json"
			regPath := filepath.Join(backupDir, regName)
			if _, err := os.Stat(regPath); err == nil {
				if err := os.Remove(regPath); err != nil {
					logger.Warning("Failed to delete old registry backup", "path", regPath, "error", err)
				}
			}
		}
	}

	return nil
}
