/*
 * Component: Search Result Enricher
 * Block-UUID: 87706059-d2d6-45aa-9044-8e6480814dfe
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Enriches raw search matches with metadata from the manifest database. Performs batch lookups for efficiency.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// EnrichMatches takes raw search matches and enriches them with metadata from the database.
func EnrichMatches(ctx context.Context, matches []RawMatch, dbName string) ([]MatchResult, error) {
	if len(matches) == 0 {
		return []MatchResult{}, nil
	}

	// 1. Validate Database Exists
	if err := manifest.ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve DB Path
	dbPath, err := manifest.ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Extract unique file paths for batch lookup
	uniquePaths := make(map[string]bool)
	for _, match := range matches {
		uniquePaths[match.FilePath] = true
	}

	filePaths := make([]string, 0, len(uniquePaths))
	for path := range uniquePaths {
		filePaths = append(filePaths, path)
	}

	// 5. Fetch metadata for all files at once
	metadataMap, err := fetchMetadataMap(ctx, database, filePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	// 6. Enrich each match
	var enriched []MatchResult
	for _, match := range matches {
		result := MatchResult{
			FilePath:      match.FilePath,
			LineNumber:    match.LineNumber,
			LineText:      match.LineText,
			ContextBefore: match.ContextBefore,
			ContextAfter:  match.ContextAfter,
		}

		// Attach metadata if found
		if meta, exists := metadataMap[match.FilePath]; exists {
			result.ChatID = meta.ChatID
			result.Metadata = meta.Fields
		} else {
			result.ChatID = 0
			result.Metadata = map[string]interface{}{}
		}

		enriched = append(enriched, result)
	}

	logger.Info("Enriched %d matches", len(enriched))
	return enriched, nil
}

// fileMetadata holds the ChatID and fields for a specific file.
type fileMetadata struct {
	ChatID int
	Fields map[string]interface{}
}

// fetchMetadataMap performs a batch query to retrieve metadata for multiple files.
func fetchMetadataMap(ctx context.Context, database *sql.DB, filePaths []string) (map[string]fileMetadata, error) {
	result := make(map[string]fileMetadata)

	if len(filePaths) == 0 {
		return result, nil
	}

	// Build query with IN clause
	placeholders := make([]string, len(filePaths))
	args := make([]interface{}, len(filePaths))
	for i, path := range filePaths {
		placeholders[i] = "?"
		args[i] = path
	}

	query := fmt.Sprintf(`
		SELECT 
			f.file_path,
			f.chat_id,
			mf.field_name,
			fm.field_value
		FROM files f
		LEFT JOIN file_metadata fm ON f.file_path = fm.file_path
		LEFT JOIN metadata_fields mf ON fm.field_id = mf.field_id
		WHERE f.file_path IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		var filePath string
		var chatID int
		var fieldName, fieldValue sql.NullString

		if err := rows.Scan(&filePath, &chatID, &fieldName, &fieldValue); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Initialize entry if not exists
		if _, exists := result[filePath]; !exists {
			result[filePath] = fileMetadata{
				ChatID: chatID,
				Fields: make(map[string]interface{}),
			}
		}

		// Add field value
		entry := result[filePath]
		if fieldName.Valid {
			if fieldValue.Valid {
				entry.Fields[fieldName.String] = fieldValue.String
			} else {
				entry.Fields[fieldName.String] = nil
			}
		}
		result[filePath] = entry
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
