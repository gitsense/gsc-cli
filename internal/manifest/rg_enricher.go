/**
 * Component: Ripgrep Metadata Enricher
 * Block-UUID: 3c121224-6fd9-48ff-a258-be46a197a4e1
 * Parent-UUID: 11fd1354-41c0-4c66-aa0a-8e0cdd997514
 * Version: 1.3.0
 * Description: Enriches raw ripgrep matches with metadata. Added GetMetadataForFiles to support batch lookups for the dual-pass workflow, returning a map of file paths to metadata results. Refactored all logger calls to use structured Key-Value pairs instead of format strings.
 * Language: Go
 * Created-at: 2026-02-03T07:54:54.354Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package manifest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"gopkg.in/yaml.v3"
)

// EnrichMatches takes raw ripgrep matches and enriches them with metadata from the database.
// It looks up each file in the database and fetches all associated metadata fields.
func EnrichMatches(ctx context.Context, matches []RgMatch, dbName string) ([]EnrichedMatch, error) {
	if len(matches) == 0 {
		return []EnrichedMatch{}, nil
	}

	// 1. Validate Database Exists
	if err := ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve DB Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Enrich each match
	var enriched []EnrichedMatch
	for _, match := range matches {
		enrichedMatch, err := enrichSingleMatch(ctx, database, match)
		if err != nil {
			logger.Warning("Failed to enrich match", "file", match.FilePath, "error", err)
			// Continue with other matches even if one fails
			continue
		}
		enriched = append(enriched, enrichedMatch)
	}

	logger.Info("Enriched matches", "count", len(enriched))
	return enriched, nil
}

// GetMetadataForFiles retrieves metadata for a specific list of file paths.
// It returns a map where the key is the file path and the value is the FileMetadataResult.
// This is optimized for the dual-pass workflow where we have a list of unique files.
func GetMetadataForFiles(ctx context.Context, filePaths []string, dbName string) (map[string]FileMetadataResult, error) {
	result := make(map[string]FileMetadataResult)

	if len(filePaths) == 0 {
		return result, nil
	}

	// 1. Validate Database Exists
	if err := ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve DB Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Initialize result map with "not_found" status for all requested files
	for _, path := range filePaths {
		result[path] = FileMetadataResult{
			FilePath: path,
			Status:   "not_found",
		}
	}

	// 5. Build query to fetch metadata for all files at once
	// We use a placeholder for the IN clause
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

	// 6. Execute Query
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()

	// 7. Process Results
	for rows.Next() {
		var filePath string
		var chatID int
		var fieldName, fieldValue sql.NullString

		if err := rows.Scan(&filePath, &chatID, &fieldName, &fieldValue); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Update the entry in the result map
		if entry, exists := result[filePath]; exists {
			// Mark as found
			entry.Status = "found"
			entry.ChatID = chatID

			// Initialize Fields map if nil
			if entry.Fields == nil {
				entry.Fields = make(map[string]interface{})
			}

			// Add field value if valid
			if fieldName.Valid {
				if fieldValue.Valid {
					entry.Fields[fieldName.String] = fieldValue.String
				} else {
					entry.Fields[fieldName.String] = nil
				}
			}

			// Update map
			result[filePath] = entry
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	logger.Info("Retrieved metadata for files", "count", len(result))
	return result, nil
}

// enrichSingleMatch enriches a single ripgrep match with metadata.
func enrichSingleMatch(ctx context.Context, database *sql.DB, match RgMatch) (EnrichedMatch, error) {
	// 1. Get Chat ID and Metadata for the file
	query := `
		SELECT 
			f.chat_id,
			mf.field_name,
			fm.field_value
		FROM files f
		LEFT JOIN file_metadata fm ON f.file_path = fm.file_path
		LEFT JOIN metadata_fields mf ON fm.field_id = mf.field_id
		WHERE f.file_path = ?
	`

	rows, err := database.QueryContext(ctx, query, match.FilePath)
	if err != nil {
		return EnrichedMatch{}, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()

	var chatID int
	metadata := make(map[string]interface{})

	found := false
	for rows.Next() {
		found = true
		var fieldName, fieldValue sql.NullString
		
		if err := rows.Scan(&chatID, &fieldName, &fieldValue); err != nil {
			return EnrichedMatch{}, fmt.Errorf("failed to scan row: %w", err)
		}

		// Add to metadata map if field name is valid
		if fieldName.Valid {
			if fieldValue.Valid {
				metadata[fieldName.String] = fieldValue.String
			} else {
				metadata[fieldName.String] = nil
			}
		}
	}

	if err := rows.Err(); err != nil {
		return EnrichedMatch{}, err
	}

	if !found {
		// File not found in database, return match without metadata
		logger.Warning("File not found in database", "file", match.FilePath)
		return EnrichedMatch{
			FilePath:   match.FilePath,
			ChatID:     0,
			LineNumber: match.LineNumber,
			Match:      match.MatchText,
			Metadata:   map[string]interface{}{},
		}, nil
	}

	// 2. Construct EnrichedMatch
	return EnrichedMatch{
		FilePath:   match.FilePath,
		ChatID:     chatID,
		LineNumber: match.LineNumber,
		Match:      match.MatchText,
		Metadata:   metadata,
	}, nil
}

// MetadataOutput wraps the metadata results for consistent YAML/JSON formatting.
type MetadataOutput struct {
	Metadata []FileMetadataResult `yaml:"metadata" json:"metadata"`
}

// FormatMetadataYAML formats the metadata map into a YAML string.
// It sorts the results by file path for consistent output.
func FormatMetadataYAML(metadataMap map[string]FileMetadataResult) string {
	if len(metadataMap) == 0 {
		return "metadata: []\n"
	}

	// Convert map to slice and sort by file path
	var results []FileMetadataResult
	for _, result := range metadataMap {
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FilePath < results[j].FilePath
	})

	output := MetadataOutput{
		Metadata: results,
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Sprintf("Error formatting YAML: %v\n", err)
	}

	return string(data)
}

// FormatMetadataJSON formats the metadata map into a JSON string.
// It sorts the results by file path for consistent output.
func FormatMetadataJSON(metadataMap map[string]FileMetadataResult) string {
	if len(metadataMap) == 0 {
		return "{\"metadata\": []}\n"
	}

	// Convert map to slice and sort by file path
	var results []FileMetadataResult
	for _, result := range metadataMap {
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FilePath < results[j].FilePath
	})

	output := MetadataOutput{
		Metadata: results,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v\n", err)
	}

	return string(data)
}
