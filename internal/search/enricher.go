/**
 * Component: Search Result Enricher
 * Block-UUID: 21ddbd40-3b85-4859-979b-89412a29d417
 * Parent-UUID: 5a96db41-a3d3-498d-99f2-a00865a47e23
 * Version: 2.7.0
 * Description: Enriches raw search matches with metadata from the manifest database. Supports filtering by analyzed status, file patterns, and metadata conditions. Refactored SQL query construction in fetchMetadataMap for clarity and correctness. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted routine Info logs to Debug level to enable quiet-by-default behavior. Updated checkSingleCondition to support querying array fields stored as JSON strings.
 * Language: Go
 * Created-at: 2026-02-06T04:07:02.769Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0), Gemini 3 Flash (v2.5.0), Gemini 3 Flash (v2.6.0), Gemini 3 Flash (v2.6.1), Gemini 3 Flash (v2.7.0)
 */


package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// EnrichMatches takes raw search matches and enriches them with metadata from the database.
// It applies system filters (analyzed, file path) and metadata filters.
func EnrichMatches(ctx context.Context, matches []RawMatch, dbName string, filters []FilterCondition, analyzedFilter string, filePatterns []string, requestedFields []string, cwdOffset string) ([]MatchResult, []string, error) {
	if len(matches) == 0 {
		return []MatchResult{}, []string{}, nil
	}

	// 1. Validate Database Exists
	if err := manifest.ValidateDBExists(dbName); err != nil {
		return nil, nil, err
	}

	// 2. Resolve DB Path
	dbPath, err := manifest.ResolveDBPath(dbName)
	if err != nil {
		return nil, nil, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.CloseDB(database)

	// 4. Extract unique file paths for batch lookup
	uniquePaths := make(map[string]bool)
	for _, match := range matches {
		// Normalize path to be relative to repo root for DB lookup
		dbPath := filepath.Join(cwdOffset, match.FilePath)
		uniquePaths[dbPath] = true
	}

	filePaths := make([]string, 0, len(uniquePaths))
	for path := range uniquePaths {
		filePaths = append(filePaths, path)
	}

	// 5. Fetch metadata for all files at once, applying system filters
	// Note: Metadata filters (conditions) are applied in-memory after fetching
	// to handle multi-field logic correctly (e.g., topic=X AND language=Y).
	metadataMap, availableFields, err := fetchMetadataMap(ctx, database, filePaths, analyzedFilter, filePatterns, requestedFields, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	// 6. Enrich each match and apply metadata filters
	var enriched []MatchResult
	for _, match := range matches {
		// Normalize path for lookup
		dbPath := filepath.Join(cwdOffset, match.FilePath)

		result := MatchResult{
			FilePath:      dbPath, // Use normalized path for the result
			LineNumber:    match.LineNumber,
			LineText:      match.LineText,
			ContextBefore: match.ContextBefore,
			ContextAfter:  match.ContextAfter,
			Submatches:    match.Submatches,
		}

		// Check if file exists in metadata map (passed system filters)
		if meta, exists := metadataMap[dbPath]; exists {
			result.ChatID = meta.ChatID
			result.Metadata = meta.Fields

			// Apply metadata filters
			if len(filters) > 0 {
				if !checkFilters(meta.Fields, filters) {
					// Filtered out by metadata conditions
					continue
				}
			}

			// Apply field projection (prune fields not requested)
			if len(requestedFields) > 0 {
				pruned := make(map[string]interface{})
				for _, rf := range requestedFields {
					if val, ok := result.Metadata[rf]; ok {
						pruned[rf] = val
					}
				}
				result.Metadata = pruned
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

	logger.Debug("Enriched matches", "count", len(enriched), "filtered_from", len(matches))
	return enriched, availableFields, nil
}

// fileMetadata holds the ChatID and fields for a specific file.
type fileMetadata struct {
	ChatID int
	Fields map[string]interface{}
}

// fetchMetadataMap performs a batch query to retrieve metadata for multiple files.
// It applies system filters (analyzed status, file patterns) via SQL.
func fetchMetadataMap(ctx context.Context, database *sql.DB, filePaths []string, analyzedFilter string, filePatterns []string, requestedFields []string, filters []FilterCondition) (map[string]fileMetadata, []string, error) {
	result := make(map[string]fileMetadata)
	var availableFields []string

	if len(filePaths) == 0 {
		return result, availableFields, nil
	}

	// 1. Get all available fields in the database for discovery
	fieldRows, err := database.QueryContext(ctx, "SELECT field_name FROM metadata_fields ORDER BY field_name")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query available fields: %w", err)
	}
	defer fieldRows.Close()
	for fieldRows.Next() {
		var fn string
		if err := fieldRows.Scan(&fn); err == nil {
			availableFields = append(availableFields, fn)
		}
	}

	// 2. Determine which fields we MUST fetch (requested + those needed for filters)
	fetchList := make(map[string]bool)
	for _, f := range requestedFields {
		fetchList[f] = true
	}
	for _, f := range filters {
		fetchList[f.Field] = true
	}

	// Build query with IN clause for file paths
	placeholders := make([]string, len(filePaths))
	args := make([]interface{}, len(filePaths))
	for i, path := range filePaths {
		placeholders[i] = "?"
		args[i] = path
	}

	// Build WHERE clause for file paths
	whereClause := fmt.Sprintf("f.file_path IN (%s)", strings.Join(placeholders, ","))
	
	// Add Analyzed Filter (System Filter)
	if analyzedFilter == "true" {
		whereClause += " AND f.chat_id IS NOT NULL"
	} else if analyzedFilter == "false" {
		whereClause += " AND f.chat_id IS NULL"
	}

	// Add File Path Filters (System Filter - OR logic)
	if len(filePatterns) > 0 {
		var fileParts []string
		for _, pattern := range filePatterns {
			// Convert glob pattern to SQL LIKE
			// "internal/*" -> "internal/%"
			// "pkg/auth/*" -> "pkg/auth/%"
			sqlPattern := strings.ReplaceAll(pattern, "*", "%")
			fileParts = append(fileParts, "f.file_path LIKE ?")
			args = append(args, sqlPattern)
		}
		if len(fileParts) > 0 {
			whereClause += " AND (" + strings.Join(fileParts, " OR ") + ")"
		}
	}

	// Add Field Projection to SQL if we have a specific list to fetch
	if len(fetchList) > 0 {
		var fieldPlaceholders []string
		for f := range fetchList {
			fieldPlaceholders = append(fieldPlaceholders, "?")
			args = append(args, f)
		}
		whereClause += fmt.Sprintf(" AND mf.field_name IN (%s)", strings.Join(fieldPlaceholders, ","))
	}

	// Construct the final query
	// We join files, file_metadata, and metadata_fields to get all field values
	baseQuery := `
		SELECT 
			f.file_path,
			f.chat_id,
			mf.field_name,
			fm.field_value
		FROM files f
		LEFT JOIN file_metadata fm ON f.file_path = fm.file_path
		LEFT JOIN metadata_fields mf ON fm.field_id = mf.field_id
		WHERE ` + whereClause

	rows, err := database.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()

	// Process results
	for rows.Next() {
		var filePath string
		var chatID sql.NullInt64
		var fieldName, fieldValue sql.NullString

		if err := rows.Scan(&filePath, &chatID, &fieldName, &fieldValue); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Initialize entry if not exists
		if _, exists := result[filePath]; !exists {
			cID := 0
			if chatID.Valid {
				cID = int(chatID.Int64)
			}
			result[filePath] = fileMetadata{
				ChatID: cID,
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
		return nil, nil, err
	}

	return result, availableFields, nil
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

	// Attempt to parse as JSON array first
	var jsonArray []interface{}
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", value)), &jsonArray); err == nil {
		// Successfully parsed as JSON array
		return checkArrayCondition(jsonArray, cond)
	}

	// Fallback to scalar logic
	switch cond.Operator {
	case "=":
		return valueStr == condValue

	case "!=":
		return valueStr != condValue

	case "in":
		targetValues := strings.Split(condValue, ",")
		for _, tgt := range targetValues {
			if valueStr == strings.TrimSpace(tgt) {
				return true
			}
		}
		return false

	case "not in":
		targetValues := strings.Split(condValue, ",")
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

// checkArrayCondition handles comparison logic for JSON array fields.
func checkArrayCondition(array []interface{}, cond FilterCondition) bool {
	condValue := strings.ToLower(cond.Value)

	switch cond.Operator {
	case "=":
		// Check if condValue exists in the array
		for _, item := range array {
			if strings.ToLower(fmt.Sprintf("%v", item)) == condValue {
				return true
			}
		}
		return false

	case "!=":
		// Check if condValue does NOT exist in the array
		for _, item := range array {
			if strings.ToLower(fmt.Sprintf("%v", item)) == condValue {
				return false
			}
		}
		return true

	case "in":
		// Check if any of the comma-separated values in condValue exist in the array
		targetValues := strings.Split(condValue, ",")
		for _, item := range array {
			itemStr := strings.ToLower(fmt.Sprintf("%v", item))
			for _, tgt := range targetValues {
				if itemStr == strings.TrimSpace(tgt) {
					return true
				}
			}
		}
		return false

	case "not in":
		// Check if NONE of the comma-separated values in condValue exist in the array
		targetValues := strings.Split(condValue, ",")
		for _, item := range array {
			itemStr := strings.ToLower(fmt.Sprintf("%v", item))
			for _, tgt := range targetValues {
				if itemStr == strings.TrimSpace(tgt) {
					return false
				}
			}
		}
		return true

	case "~":
		// Check if any element in the array contains condValue
		for _, item := range array {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", item)), condValue) {
				return true
			}
		}
		return false

	case "!~":
		// Check if NO element in the array contains condValue
		for _, item := range array {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", item)), condValue) {
				return false
			}
		}
		return true

	default:
		// Operators like >, <, range are not supported for arrays in this context
		return false
	}
}
