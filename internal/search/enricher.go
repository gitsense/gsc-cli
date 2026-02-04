/*
 * Component: Search Result Enricher
 * Block-UUID: 571a8435-b2b3-423e-9104-48c75dce5812
 * Parent-UUID: 87706059-d2d6-45aa-9044-8e6480814dfe
 * Version: 2.0.0
 * Description: Enriches raw search matches with metadata from the manifest database. Supports filtering by analyzed status, file patterns, and metadata conditions.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0)
 */


package search

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// EnrichMatches takes raw search matches and enriches them with metadata from the database.
// It applies system filters (analyzed, file path) and metadata filters.
func EnrichMatches(ctx context.Context, matches []RawMatch, dbName string, filters []FilterCondition, analyzedFilter string, filePatterns []string) ([]MatchResult, error) {
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

	// 5. Fetch metadata for all files at once, applying system filters
	// Note: Metadata filters (conditions) are applied in-memory after fetching
	// to handle multi-field logic correctly (e.g., topic=X AND language=Y).
	metadataMap, err := fetchMetadataMap(ctx, database, filePaths, analyzedFilter, filePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	// 6. Enrich each match and apply metadata filters
	var enriched []MatchResult
	for _, match := range matches {
		result := MatchResult{
			FilePath:      match.FilePath,
			LineNumber:    match.LineNumber,
			LineText:      match.LineText,
			ContextBefore: match.ContextBefore,
			ContextAfter:  match.ContextAfter,
		}

		// Check if file exists in metadata map (passed system filters)
		if meta, exists := metadataMap[match.FilePath]; exists {
			result.ChatID = meta.ChatID
			result.Metadata = meta.Fields

			// Apply metadata filters
			if len(filters) > 0 {
				if !checkFilters(meta.Fields, filters) {
					// Filtered out by metadata conditions
					continue
				}
			}
		} else {
			// File did not pass system filters (e.g., analyzed=false but we wanted analyzed=true)
			// Or file simply has no metadata.
			// If analyzedFilter is "false", we expect files with no metadata to be included.
			// fetchMetadataMap handles this logic.
			// If we are here, it means the file was excluded by the SQL query.
			continue
		}

		enriched = append(enriched, result)
	}

	logger.Info("Enriched %d matches (filtered from %d raw matches)", len(enriched), len(matches))
	return enriched, nil
}

// fileMetadata holds the ChatID and fields for a specific file.
type fileMetadata struct {
	ChatID int
	Fields map[string]interface{}
}

// fetchMetadataMap performs a batch query to retrieve metadata for multiple files.
// It applies system filters (analyzed status, file patterns) via SQL.
func fetchMetadataMap(ctx context.Context, database *sql.DB, filePaths []string, analyzedFilter string, filePatterns []string) (map[string]fileMetadata, error) {
	result := make(map[string]fileMetadata)

	if len(filePaths) == 0 {
		return result, nil
	}

	// Build query with IN clause for file paths
	placeholders := make([]string, len(filePaths))
	args := make([]interface{}, len(filePaths))
	for i, path := range filePaths {
		placeholders[i] = "?"
		args[i] = path
	}

	// Build WHERE clause for system filters
	// We pass empty FilterCondition slice because metadata filters are handled in-memory
	whereClause, filterArgs, err := BuildSQLWhereClause([]FilterCondition{}, analyzedFilter, filePatterns)
	if err != nil {
		return nil, err
	}

	// Combine file path IN clause with system filters
	// If whereClause starts with "WHERE", we append " AND file_path IN (...)"
	// If whereClause is empty, we just use "WHERE file_path IN (...)"
	
	baseQuery := `
		SELECT 
			f.file_path,
			f.chat_id,
			mf.field_name,
			fm.field_value
		FROM files f
		LEFT JOIN file_metadata fm ON f.file_path = fm.file_path
		LEFT JOIN metadata_fields mf ON fm.field_id = mf.field_id
	`
	
	finalQuery := baseQuery
	if len(whereClause) > 0 {
		// Replace "WHERE" with "AND" to combine with the IN clause we're about to add
		// Actually, simpler to construct the full WHERE clause here.
		finalQuery += " WHERE f.file_path IN (" + strings.Join(placeholders, ",") + ")"
		finalQuery += " AND " + strings.TrimPrefix(whereClause, "WHERE ")
	} else {
		finalQuery += " WHERE f.file_path IN (" + strings.Join(placeholders, ",") + ")"
	}

	// Append filter args to file path args
	args = append(args, filterArgs...)

	rows, err := database.QueryContext(ctx, finalQuery, args...)
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

// checkFilters verifies if a file's metadata satisfies all filter conditions.
func checkFilters(metadata map[string]interface{}, conditions []FilterCondition) bool {
	for _, cond := range conditions {
		if !checkSingleCondition(metadata, cond) {
			return false
		}
	}
	return true
}

// checkSingleCondition verifies a single filter condition against metadata.
func checkSingleCondition(metadata map[string]interface{}, cond FilterCondition) bool {
	value, exists := metadata[cond.Field]
	
	// Handle exists/!exists
	if cond.Operator == "exists" {
		return exists
	}
	if cond.Operator == "!exists" {
		return !exists
	}

	// If field doesn't exist and operator is not exists/!exists, it fails
	if !exists {
		return false
	}

	// Convert value to string for comparison
	valueStr := fmt.Sprintf("%v", value)
	
	// Case-insensitive comparison for metadata
	valueStr = strings.ToLower(valueStr)
	condValue := strings.ToLower(cond.Value)

	switch cond.Operator {
	case "=":
		// For lists (stored as comma-separated or JSON), check if value contains the target
		// For scalars, check equality
		if strings.Contains(valueStr, ",") {
			// Treat as list
			parts := strings.Split(valueStr, ",")
			for _, part := range parts {
				if strings.TrimSpace(part) == condValue {
					return true
				}
			}
			return false
		}
		return valueStr == condValue

	case "!=":
		if strings.Contains(valueStr, ",") {
			parts := strings.Split(valueStr, ",")
			for _, part := range parts {
				if strings.TrimSpace(part) == condValue {
					return false
				}
			}
			return true
		}
		return valueStr != condValue

	case "in":
		targetValues := strings.Split(condValue, ",")
		if strings.Contains(valueStr, ",") {
			// List in List: check intersection
			sourceParts := strings.Split(valueStr, ",")
			for _, src := range sourceParts {
				for _, tgt := range targetValues {
					if strings.TrimSpace(src) == strings.TrimSpace(tgt) {
						return true
					}
				}
			}
			return false
		}
		// Scalar in List
		for _, tgt := range targetValues {
			if valueStr == strings.TrimSpace(tgt) {
				return true
			}
		}
		return false

	case "not in":
		targetValues := strings.Split(condValue, ",")
		if strings.Contains(valueStr, ",") {
			sourceParts := strings.Split(valueStr, ",")
			for _, src := range sourceParts {
				for _, tgt := range targetValues {
					if strings.TrimSpace(src) == strings.TrimSpace(tgt) {
						return false
					}
				}
			}
			return true
		}
		for _, tgt := range targetValues {
			if valueStr == strings.TrimSpace(tgt) {
				return false
			}
		}
		return true

	case "~":
		return strings.Contains(valueStr, condValue)

	case "!~":
		return !strings.Contains(valueStr, condValue)

	case ">", "<", ">=", "<=":
		// Numeric comparison
		numValue, err1 := strconv.ParseFloat(valueStr, 64)
		numCond, err2 := strconv.ParseFloat(condValue, 64)
		if err1 != nil || err2 != nil {
			return false // Cannot compare non-numbers
		}
		
		switch cond.Operator {
		case ">":
			return numValue > numCond
		case "<":
			return numValue < numCond
		case ">=":
			return numValue >= numCond
		case "<=":
			return numValue <= numCond
		}

	case "range":
		parts := strings.Split(condValue, "..")
		if len(parts) != 2 {
			return false
		}
		min, err1 := strconv.ParseFloat(parts[0], 64)
		max, err2 := strconv.ParseFloat(parts[1], 64)
		numValue, err3 := strconv.ParseFloat(valueStr, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return false
		}
		return numValue >= min && numValue <= max
	}

	return false
}
