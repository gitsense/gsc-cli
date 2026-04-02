/**
 * Component: Search Result Enricher
 * Block-UUID: 866f3047-8ef6-4f86-a8d3-e432255e5e13
 * Parent-UUID: 55498844-209d-491e-bd4f-169e50f26e11
 * Version: 3.2.0
 * Description: Exported CheckFilters, CheckSingleCondition, and CheckArrayCondition to support semantic filtering in the 'gsc tree' command.
 * Language: Go
 * Created-at: 2026-04-02T01:58:18.639Z
 * Authors: GLM-4.7 (v1.0.0), ..., Gemini 3 Flash (v2.9.0), Gemini 3 Flash (v3.0.0), claude-haiku-4-5-20251001 (v3.1.0), GLM-4.7 (v3.2.0)
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

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// EnrichMatches takes raw search matches and enriches them with metadata from the database.
// It applies system filters (analyzed, file path) and metadata filters.
func EnrichMatches(ctx context.Context, matches []RawMatch, dbName string, filters []FilterCondition, analyzedFilter string, filePatterns []string, requestedFields []string, cwdOffset string) ([]MatchResult, []string, int, error) {
	if len(matches) == 0 {
		return []MatchResult{}, []string{}, 0, nil
	}

	// 1. Validate Database Exists
	if err := db.ValidateDBExists(dbName); err != nil {
		return nil, nil, 0, err
	}

	// 2. Resolve DB Path
	dbPath, err := db.ResolveManifestDBPath(dbName)
	if err != nil {
		return nil, nil, 0, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, nil, 0, err
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
	metadataMap, availableFields, err := fetchMetadataMap(ctx, database, filePaths, analyzedFilter, filePatterns, requestedFields, filters)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to fetch metadata: %w", err)
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
				if !CheckFilters(meta.Fields, filters) {
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
			continue
		}

		enriched = append(enriched, result)
	}

	logger.Debug("Enriched matches", "count", len(enriched), "filtered_from", len(matches))
	return enriched, availableFields, len(matches) - len(enriched), nil
}

// FileMetadata holds the ChatID and fields for a specific file.
type FileMetadata struct {
	ChatID int
	Fields map[string]interface{}
}

// FetchMetadataMap is a public wrapper around fetchMetadataMap that handles DB connection.
func FetchMetadataMap(ctx context.Context, dbPath string, filePaths []string, analyzedFilter string, filePatterns []string, requestedFields []string, filters []FilterCondition) (map[string]FileMetadata, []string, error) {
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.CloseDB(database)

	return fetchMetadataMap(ctx, database, filePaths, analyzedFilter, filePatterns, requestedFields, filters)
}

// fetchMetadataMap performs a batch query to retrieve metadata for multiple files.
func fetchMetadataMap(ctx context.Context, database *sql.DB, filePaths []string, analyzedFilter string, filePatterns []string, requestedFields []string, filters []FilterCondition) (map[string]FileMetadata, []string, error) {
	result := make(map[string]FileMetadata)
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

	// 2.5. Get field types for filter processing
	fieldTypesQuery := `SELECT field_name, field_type FROM metadata_fields`
	fieldTypesRows, err := database.QueryContext(ctx, fieldTypesQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query field types: %w", err)
	}
	defer fieldTypesRows.Close()
	fieldTypes := make(map[string]string)
	for fieldTypesRows.Next() {
		var name, fieldType string
		if err := fieldTypesRows.Scan(&name, &fieldType); err != nil {
			return nil, nil, fmt.Errorf("failed to scan field type: %w", err)
		}
		fieldTypes[name] = fieldType
	}
	if err := fieldTypesRows.Err(); err != nil {
		return nil, nil, err
	}

	// 3. Determine which fields we MUST fetch
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

	// Build WHERE clause using the new array-aware filter builder
	whereClause, filterArgs, err := BuildSQLWhereClauseWithTypes(filters, analyzedFilter, filePatterns, fieldTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build WHERE clause: %w", err)
	}
	
	// Add file path IN clause
	filePathClause := fmt.Sprintf("f.file_path IN (%s)", strings.Join(placeholders, ","))
	if whereClause != "" {
		whereClause = filePathClause + " AND " + whereClause
	} else {
		whereClause = filePathClause
	}
	
	// Add field name filter
	if len(fetchList) > 0 {
		var fieldPlaceholders []string
		for f := range fetchList {
			fieldPlaceholders = append(fieldPlaceholders, "?")
			args = append(args, f)
		}
		whereClause += fmt.Sprintf(" AND mf.field_name IN (%s)", strings.Join(fieldPlaceholders, ","))
	}
	
	// Combine args: file paths + filter args
	args = append(args, filterArgs...)

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

	for rows.Next() {
		var filePath string
		var chatID sql.NullInt64
		var fieldName, fieldValue sql.NullString

		if err := rows.Scan(&filePath, &chatID, &fieldName, &fieldValue); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if _, exists := result[filePath]; !exists {
			cID := 0
			if chatID.Valid {
				cID = int(chatID.Int64)
			}
			result[filePath] = FileMetadata{
				ChatID: cID,
				Fields: make(map[string]interface{}),
			}
		}

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

// CheckFilters verifies if a file's metadata satisfies all filter conditions.
func CheckFilters(metadata map[string]interface{}, conditions []FilterCondition) bool {
	for _, cond := range conditions {
		if !CheckSingleCondition(metadata, cond) {
			return false
		}
	}
	return true
}

// CheckSingleCondition verifies a single filter condition against metadata.
func CheckSingleCondition(metadata map[string]interface{}, cond FilterCondition) bool {
	value, exists := metadata[cond.Field]
	
	if cond.Operator == "exists" {
		return exists
	}
	if cond.Operator == "!exists" {
		return !exists
	}

	if !exists {
		return false
	}

	valueStr := fmt.Sprintf("%v", value)
	valueStr = strings.ToLower(valueStr)
	condValue := strings.ToLower(cond.Value)

	var jsonArray []interface{}
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", value)), &jsonArray); err == nil {
		return CheckArrayCondition(jsonArray, cond)
	}

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
		numValue, err1 := strconv.ParseFloat(valueStr, 64)
		numCond, err2 := strconv.ParseFloat(condValue, 64)
		if err1 != nil || err2 != nil {
			return false
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

// CheckArrayCondition handles comparison logic for JSON array fields.
func CheckArrayCondition(array []interface{}, cond FilterCondition) bool {
	condValue := strings.ToLower(cond.Value)

	switch cond.Operator {
	case "=":
		for _, item := range array {
			if strings.ToLower(fmt.Sprintf("%v", item)) == condValue {
				return true
			}
		}
		return false
	case "!=":
		for _, item := range array {
			if strings.ToLower(fmt.Sprintf("%v", item)) == condValue {
				return false
			}
		}
		return true
	case "in":
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
		for _, item := range array {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", item)), condValue) {
				return true
			}
		}
		return false
	case "!~":
		for _, item := range array {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", item)), condValue) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
