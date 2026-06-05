/**
 * Component: Ripgrep Metadata Enricher
 * Block-UUID: 3440fc09-843c-4114-80f6-b2169700d488
 * Parent-UUID: 8d218b5a-5ee5-445a-8306-d557d2c71bae
 * Version: 1.4.6
 * Description: Enriches raw ripgrep matches with metadata. Added GetMetadataForFiles to support batch lookups for the dual-pass workflow, returning a map of file paths to metadata results. Refactored all logger calls to use structured Key-Value pairs instead of format strings.
 * Language: Go
 * Created-at: 2026-04-02T15:25:51.888Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), claude-haiku-4-5-20251001 (v1.4.0), GLM-4.7 (v1.4.1), GLM-4.7 (v1.4.2), claude-haiku-4-5-20251001 (v1.4.3), GLM-4.7 (v1.4.4), GLM-4.7 (v1.4.5), GLM-4.7 (v1.4.6)
 */


package manifest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/search"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"gopkg.in/yaml.v3"
)

// EnrichMatches takes raw ripgrep matches and enriches them with metadata from the database.
// It looks up each file in the database and fetches all associated metadata fields.
func EnrichMatches(ctx context.Context, matches []RgMatch, dbName string) ([]EnrichedMatch, error) {
	if len(matches) == 0 {
		return []EnrichedMatch{}, nil
	}

	// 1. Resolve DB Path
	dbPath, err := db.ResolveManifestDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 2. Validate Database Exists
	if err := db.ValidateDBExists(dbPath); err != nil {
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

// EnrichMatchesWithFilters enriches matches and applies metadata filters.
// This version supports filtering by metadata fields and analyzing status.
func EnrichMatchesWithFilters(ctx context.Context, matches []RgMatch, dbName string, filters []search.FilterCondition, analyzedFilter string, filePatterns []string, requestedFields []string, cwdOffset string) ([]EnrichedMatch, map[string]bool, int, error) {
	if len(matches) == 0 {
		return []EnrichedMatch{}, make(map[string]bool), 0, nil
	}

	// 1. Resolve DB Path
	dbPath, err := db.ResolveManifestDBPath(dbName)
	if err != nil {
		return nil, nil, 0, err
	}

	// 2. Validate Database Exists
	if err := db.ValidateDBExists(dbPath); err != nil {
		return nil, nil, 0, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, nil, 0, err
	}
	defer db.CloseDB(database)
	
	// 4. Retrieve field types for proper filter handling
	fieldTypes, err := db.GetFieldTypes(ctx, dbName)
	if err != nil {
		// Continue with empty map if field types unavailable
		fieldTypes = make(map[string]string)
	}
	
	// 5. Build WHERE clause from filters
	whereClause, whereArgs, err := search.BuildSQLWhereClauseWithTypes(filters, analyzedFilter, filePatterns, fieldTypes)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to build WHERE clause: %w", err)
	}
	
	// 6. Build query to fetch metadata for all matching files
	// We need to join files with file_metadata and metadata_fields
	query := `
		SELECT 
			f.file_path,
			f.chat_id,
			mf.field_name,
			fm.field_value
		FROM files f
		LEFT JOIN file_metadata fm ON f.file_path = fm.file_path
		LEFT JOIN metadata_fields mf ON fm.field_id = mf.field_id
	`
	
	// Add WHERE clause if we have conditions
	if whereClause != "" {
		query += " " + whereClause
	}
	
	// 7. Execute Query
	rows, err := database.QueryContext(ctx, query, whereArgs...)
	if err != nil {
		logger.Error("SQL query failed", "query", query, "args", whereArgs, "error", err)
		return nil, nil, 0, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()
	
	// 8. Process Results
	// We'll build a map of file_path -> metadata
	fileMetadataMap := make(map[string]map[string]interface{})
	availableFields := make(map[string]bool)
	
	for rows.Next() {
		var filePath string
		var chatID int
		var fieldName, fieldValue sql.NullString
		
		if err := rows.Scan(&filePath, &chatID, &fieldName, &fieldValue); err != nil {
			return nil, nil, 0, fmt.Errorf("failed to scan row: %w", err)
		}
		
		// Initialize metadata map for this file if not exists
		if _, exists := fileMetadataMap[filePath]; !exists {
			fileMetadataMap[filePath] = make(map[string]interface{})
		}
		
		// Add field value if valid
		if fieldName.Valid {
			availableFields[fieldName.String] = true
			if fieldValue.Valid {
				fileMetadataMap[filePath][fieldName.String] = fieldValue.String
			} else {
				fileMetadataMap[filePath][fieldName.String] = nil
			}
		}
	}
	
	if err := rows.Err(); err != nil {
		return nil, nil, 0, err
	}
	
	// 9. Enrich matches with metadata
	var enriched []EnrichedMatch
	matchesOutsideScope := 0
	
	for _, match := range matches {
		metadata, found := fileMetadataMap[match.FilePath]
		if !found {
			// File not in query results (filtered out or not in database)
			matchesOutsideScope++
			continue
		}
		
		// Get chat ID from metadata or default to 0
		chatID := 0
		if chatIDVal, ok := metadata["chat_id"].(int); ok {
			chatID = chatIDVal
		}
		
		enrichedMatch := EnrichedMatch{
			FilePath:   match.FilePath,
			ChatID:     chatID,
			LineNumber: match.LineNumber,
			Match:      match.MatchText,
			Metadata:   metadata,
		}
		
		enriched = append(enriched, enrichedMatch)
	}
	
	logger.Info("Enriched matches with filters", "total", len(matches), "enriched", len(enriched), "outside_scope", matchesOutsideScope)
	return enriched, availableFields, matchesOutsideScope, nil
}

// GetMetadataForFiles retrieves metadata for a specific list of file paths.
// It returns a map where the key is the file path and the value is the FileMetadataResult.
// This is optimized for the dual-pass workflow where we have a list of unique files.
func GetMetadataForFiles(ctx context.Context, filePaths []string, dbName string) (map[string]FileMetadataResult, error) {
	result := make(map[string]FileMetadataResult)

	if len(filePaths) == 0 {
		return result, nil
	}

	// 1. Resolve DB Path
	dbPath, err := db.ResolveManifestDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 2. Validate Database Exists
	if err := db.ValidateDBExists(dbPath); err != nil {
		return nil, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Early return if no file paths to query
	// This prevents building an invalid SQL query with an empty IN clause
	if len(filePaths) == 0 {
		return result, nil
	}

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
