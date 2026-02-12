/**
 * Component: Simple Query Executor
 * Block-UUID: 0de5c5a0-21f2-4aca-af00-2a7185139326
 * Parent-UUID: cec62867-448b-4f54-acd4-f8a10af853fc
 * Version: 1.8.0
 * Description: Executes simple value-matching queries and hierarchical list operations. Updated GetListResult to support the '--all' flag, which populates a nested hierarchy of databases and their fields. Refactored listAllDatabases to correctly map command-line slugs (Name) and human-friendly display names (Label) for improved ergonomics.
 * Language: Go
 * Created-at: 2026-02-11T07:35:30.010Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.7.1), GLM-4.7 (v1.7.2), Gemini 3 Flash (v1.8.0)
 */


package manifest

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// ExecuteSimpleQuery performs a simple value-matching query against the database.
// It supports comma-separated values for OR logic.
func ExecuteSimpleQuery(ctx context.Context, dbName string, fieldName string, value string) ([]QueryResult, error) {
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

	// 4. Parse Values (comma-separated for OR logic)
	values := strings.Split(value, ",")

	// 5. Build Query
	// We need to find files where the specified field matches any of the values
	// The field_name is stored in metadata_fields, and the value is in file_metadata
	// We need to join tables to get the field_id first, then match values
	
	// Step 5a: Get field_id and field_type for the field name
	var fieldID, fieldType string
	fieldQuery := `SELECT field_id, field_type FROM metadata_fields WHERE field_name = ? LIMIT 1`
	err = database.QueryRowContext(ctx, fieldQuery, fieldName).Scan(&fieldID, &fieldType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("field '%s' not found in database '%s'", fieldName, dbName)
		}
		return nil, fmt.Errorf("failed to query field: %w", err)
	}

	// Step 5b: Query files matching the values
	// Strategy depends on field_type:
	// - If "array" or "list": Use json_each to search within the JSON array string.
	// - Otherwise: Use standard IN clause for scalar matching.
	
	args := make([]interface{}, 0, len(values)+1)
	args = append(args, fieldID)

	var conditions []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if strings.Contains(v, "*") {
			pattern := strings.ReplaceAll(v, "*", "%")
			if fieldType == "array" || fieldType == "list" {
				conditions = append(conditions, "json_each.value LIKE ?")
			} else {
				conditions = append(conditions, "fm.field_value LIKE ?")
			}
			args = append(args, pattern)
		} else {
			if fieldType == "array" || fieldType == "list" {
				conditions = append(conditions, "json_each.value = ?")
			} else {
				conditions = append(conditions, "fm.field_value = ?")
			}
			args = append(args, v)
		}
	}

	whereClause := strings.Join(conditions, " OR ")
	var query string

	if fieldType == "array" || fieldType == "list" {
		query = fmt.Sprintf("SELECT f.file_path, f.chat_id FROM files f INNER JOIN file_metadata fm ON f.file_path = fm.file_path WHERE fm.field_id = ? AND EXISTS (SELECT 1 FROM json_each(fm.field_value) WHERE %s)", whereClause)
	} else {
		query = fmt.Sprintf("SELECT f.file_path, f.chat_id FROM files f INNER JOIN file_metadata fm ON f.file_path = fm.file_path WHERE fm.field_id = ? AND (%s)", whereClause)
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var results []QueryResult
	for rows.Next() {
		var r QueryResult
		if err := rows.Scan(&r.FilePath, &r.ChatID); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	logger.Info("Query executed successfully", "database", dbName, "field", fieldName, "results", len(results))
	return results, nil
}

// GetListResult performs hierarchical discovery based on the provided arguments.
// It implements the "Discovery Dashboard" logic:
// - If fieldName is empty: Returns both Databases and Fields (if dbName is set).
// - If fieldName is set: Returns unique values for that field.
// - If all is true: Returns a full map of all databases and their fields.
func GetListResult(ctx context.Context, dbName string, fieldName string, all bool) (*ListResult, error) {
	result := &ListResult{
		ActiveDatabase: dbName,
		Hints:          []string{},
	}

	// Level 1 & 2: Discovery (Databases and/or Fields)
	if fieldName == "" {
		result.Level = "discovery"
		
		// Always get databases
		dbs, err := listAllDatabases(ctx)
		if err != nil {
			return nil, err
		}
		result.Databases = dbs

		// Handle --all flag: Populate fields for every database
		if all {
			for i := range result.Databases {
				fields, err := listFieldsInDatabase(ctx, result.Databases[i].Name)
				if err != nil {
					logger.Warning("Failed to list fields for database in --all view", "db", result.Databases[i].Name, "error", err)
					continue
				}
				result.Databases[i].Fields = fields
			}
			result.Hints = append(result.Hints, "Use 'gsc query list --db <name>' to see fields for a specific database.")
			result.Hints = append(result.Hints, "Use 'gsc query list --db <name> <field>' to see unique values for a specific field.")
			return result, nil
		}

		// Standard Discovery Dashboard: If a database is active, also get its fields
		if dbName != "" {
			fields, err := listFieldsInDatabase(ctx, dbName)
			if err != nil {
				return nil, err
			}
			result.Fields = fields
			result.Hints = append(result.Hints, fmt.Sprintf("Use 'gsc query list --db %s <field>' to see unique values for a specific field.", dbName))
			result.Hints = append(result.Hints, "Use 'gsc query list --db <name>' to see fields for a different database.")
		} else {
			result.Hints = append(result.Hints, "Use 'gsc query list --all' to see the full intelligence map.")
			result.Hints = append(result.Hints, "Use 'gsc query list --db <name>' to see available fields, or 'gsc config use <name>' to set a default database.")
		}

		return result, nil
	}

	// Level 3: Value Discovery
	result.Level = "value"
	values, err := listValuesForField(ctx, dbName, fieldName)
	if err != nil {
		return nil, err
	}
	result.Values = values

	return result, nil
}

// listAllDatabases returns a list of all registered databases.
func listAllDatabases(ctx context.Context) ([]ListItem, error) {
	dbs, err := ListDatabases(ctx)
	if err != nil {
		return nil, err
	}

	var items []ListItem
	for _, dbInfo := range dbs {
		// Derive the slug (Name) from the physical filename
		slug := strings.TrimSuffix(filepath.Base(dbInfo.DBPath), ".db")

		items = append(items, ListItem{
			Name:        slug,        // The easy-to-type identifier (e.g., "security")
			Label:       dbInfo.DatabaseName, // The human-friendly display name (e.g., "Security Analysis")
			Description: dbInfo.Description,
			Source:      filepath.Base(dbInfo.DBPath),
			Count:       dbInfo.EntryCount,
		})
	}

	return items, nil
}

// listFieldsInDatabase returns a list of all fields in the specified database.
func listFieldsInDatabase(ctx context.Context, dbName string) ([]ListItem, error) {
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

	// 4. Query Fields
	query := `
		SELECT DISTINCT mf.field_name, mf.field_type, mf.field_description
		FROM metadata_fields mf
		ORDER BY mf.field_name
	`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query fields: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		var name, fieldType, description string
		if err := rows.Scan(&name, &fieldType, &description); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		items = append(items, ListItem{
			Name:        name,
			Type:        fieldType,
			Description: description,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// listValuesForField returns a list of unique values for the specified field in the database.
func listValuesForField(ctx context.Context, dbName string, fieldName string) ([]ListItem, error) {
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

	// 4. Get field_id
	var fieldID string
	fieldQuery := `SELECT field_id FROM metadata_fields WHERE field_name = ? LIMIT 1`
	err = database.QueryRowContext(ctx, fieldQuery, fieldName).Scan(&fieldID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("field '%s' not found in database '%s'", fieldName, dbName)
		}
		return nil, fmt.Errorf("failed to query field: %w", err)
	}

	// 5. Query Values with Counts
	query := `
		SELECT fm.field_value, COUNT(*) as count
		FROM file_metadata fm
		WHERE fm.field_id = ?
		GROUP BY fm.field_value
		ORDER BY count DESC
	`

	rows, err := database.QueryContext(ctx, query, fieldID)
	if err != nil {
		return nil, fmt.Errorf("failed to query values: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		items = append(items, ListItem{
			Name:  value,
			Count: count,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

// PrepareTargetSet creates and populates a temporary table with files matching the active Focus Scope.
// This function is critical for Phase 2 performance, allowing efficient joins against the 'files' table.
// It drops any existing 'target_set' table to ensure a clean state for the current query context.
func PrepareTargetSet(ctx context.Context, db *sql.DB, scope *ScopeConfig, repoRoot string) error {
	// 1. Clean up existing temporary table (if any)
	// We ignore errors here as the table might not exist
	_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS target_set")

	// 2. Create the temporary table
	// Using a temporary table ensures it is session-specific and cleaned up automatically
	_, err := db.ExecContext(ctx, "CREATE TEMPORARY TABLE target_set (file_path TEXT PRIMARY KEY)")
	if err != nil {
		return fmt.Errorf("failed to create temporary table target_set: %w", err)
	}

	// 3. Get all tracked files from Git
	trackedFiles, err := git.GetTrackedFiles(ctx, repoRoot)
	if err != nil {
		return fmt.Errorf("failed to get tracked files for target set: %w", err)
	}

	// 4. Filter files by scope and insert into temporary table
	// Using a prepared statement for batch insertion efficiency
	stmt, err := db.PrepareContext(ctx, "INSERT INTO target_set (file_path) VALUES (?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement for target_set: %w", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, filePath := range trackedFiles {
		// Check if file matches the scope
		if MatchScope(filePath, scope) {
			if _, err := stmt.ExecContext(ctx, filePath); err != nil {
				return fmt.Errorf("failed to insert file into target_set: %w", err)
			}
			insertedCount++
		}
	}

	logger.Debug("Target set prepared", "total_tracked", len(trackedFiles), "in_scope", insertedCount)
	return nil
}

// ExecuteCoverageAnalysis calculates analysis coverage within the active Focus Scope.
// It compares Git tracked files against the manifest database to identify blind spots.
func ExecuteCoverageAnalysis(ctx context.Context, dbName string, scopeOverride string, repoRoot string, profileName string) (*CoverageReport, error) {
	// 1. Resolve Scope
	scope, err := ResolveScopeForQuery(ctx, profileName, scopeOverride)
	if err != nil {
		return nil, err
	}

	// 2. Open Database
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 3. Prepare Target Set (Temporary Table)
	if err := PrepareTargetSet(ctx, database, scope, repoRoot); err != nil {
		return nil, err
	}

	// 4. Initialize Report
	report := &CoverageReport{
		Timestamp:       time.Now(),
		ActiveProfile:   profileName,
		ScopeDefinition: scope,
		ByLanguage:      make(map[string]LanguageCoverage),
		Recommendations: []string{},
	}

	// 5. Calculate Overall Totals
	// We use the target_set temp table to count in-scope files and join with files table for analyzed count
	totalQuery := `
		SELECT 
			(SELECT COUNT(*) FROM target_set) as in_scope,
			(SELECT COUNT(*) FROM files f JOIN target_set ts ON f.file_path = ts.file_path WHERE f.chat_id IS NOT NULL) as analyzed
	`
	err = database.QueryRowContext(ctx, totalQuery).Scan(&report.Totals.InScopeFiles, &report.Totals.AnalyzedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to query coverage totals: %w", err)
	}

	trackedFiles, _ := git.GetTrackedFiles(ctx, repoRoot)
	report.Totals.TrackedFiles = len(trackedFiles)
	report.Totals.OutOfScopeFiles = report.Totals.TrackedFiles - report.Totals.InScopeFiles

	if report.Totals.InScopeFiles > 0 {
		report.Percentages.FocusCoverage = (float64(report.Totals.AnalyzedFiles) / float64(report.Totals.InScopeFiles)) * 100
	}
	if report.Totals.TrackedFiles > 0 {
		report.Percentages.TotalCoverage = (float64(report.Totals.AnalyzedFiles) / float64(report.Totals.TrackedFiles)) * 100
	}

	// 6. Calculate Language Breakdown
	langQuery := `
		SELECT 
			COALESCE(f.language, 'Unknown') as lang,
			COUNT(*) as total,
			COUNT(f.chat_id) as analyzed
		FROM target_set ts
		LEFT JOIN files f ON ts.file_path = f.file_path
		GROUP BY lang
		ORDER BY total DESC
	`
	rows, err := database.QueryContext(ctx, langQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query language coverage: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var lang string
		var stats LanguageCoverage
		if err := rows.Scan(&lang, &stats.Total, &stats.Analyzed); err != nil {
			return nil, err
		}
		if stats.Total > 0 {
			stats.Percent = (float64(stats.Analyzed) / float64(stats.Total)) * 100
		}
		report.ByLanguage[lang] = stats
	}

	// 7. Identify Blind Spots (Directories)
	// We find files in target_set that are NOT in the files table or have NULL chat_id
	blindSpotQuery := `
		SELECT ts.file_path
		FROM target_set ts
		LEFT JOIN files f ON ts.file_path = f.file_path
		WHERE f.chat_id IS NULL
	`
	bsRows, err := database.QueryContext(ctx, blindSpotQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query blind spots: %w", err)
	}
	defer bsRows.Close()

	dirMap := make(map[string]*DirectoryBlindSpot)
	for bsRows.Next() {
		var path string
		if err := bsRows.Scan(&path); err != nil {
			return nil, err
		}

		// Group by first two segments
		segments := strings.Split(filepath.ToSlash(path), "/")
		dir := ""
		if len(segments) > 1 {
			dir = segments[0] + "/" + segments[1] + "/"
		} else if len(segments) == 1 {
			dir = segments[0]
		}

		if _, ok := dirMap[dir]; !ok {
			dirMap[dir] = &DirectoryBlindSpot{Path: dir}
		}
		dirMap[dir].TotalFiles++
	}

	for _, ds := range dirMap {
		report.BlindSpots.Directories = append(report.BlindSpots.Directories, *ds)
	}

	sort.Slice(report.BlindSpots.Directories, func(i, j int) bool {
		return report.BlindSpots.Directories[i].TotalFiles > report.BlindSpots.Directories[j].TotalFiles
	})

	if len(report.BlindSpots.Directories) > 5 {
		report.BlindSpots.Directories = report.BlindSpots.Directories[:5]
	}

	// 8. Set Status and Recommendations
	if report.Percentages.FocusCoverage >= 90 {
		report.AnalysisStatus = "High Confidence"
		report.Recommendations = append(report.Recommendations, fmt.Sprintf("%.1f%% of in-scope files analyzed. High confidence for scoped queries.", report.Percentages.FocusCoverage))
	} else if report.Percentages.FocusCoverage > 0 {
		report.AnalysisStatus = "Partial"
		report.Recommendations = append(report.Recommendations, "Coverage is partial. Consider importing more manifests to fill blind spots.")
	} else {
		report.AnalysisStatus = "No Coverage"
		report.Recommendations = append(report.Recommendations, "No files in scope have been analyzed. Run 'gsc manifest import' to add intelligence.")
	}

	return report, nil
}

// ExecuteInsightsAnalysis performs metadata aggregation for the requested fields within the active Focus Scope.
// It implements type-aware SQL aggregation (scalar vs array) and calculates summary statistics.
func ExecuteInsightsAnalysis(ctx context.Context, dbName string, fields []string, limit int, scopeOverride string, repoRoot string, profileName string) (*InsightsReport, error) {
	// 1. Resolve Scope
	scope, err := ResolveScopeForQuery(ctx, profileName, scopeOverride)
	if err != nil {
		return nil, err
	}

	// 2. Open Database
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 3. Prepare Target Set (Temporary Table)
	if err := PrepareTargetSet(ctx, database, scope, repoRoot); err != nil {
		return nil, err
	}

	// 4. Initialize Report
	report := &InsightsReport{
		Context: InsightsContext{
			Database:        dbName,
			Type:            "insights",
			Limit:           limit,
			ScopeApplied:    scope != nil,
			ScopeDefinition: scope,
			Timestamp:       time.Now(),
		},
		Insights: make(map[string][]FieldInsight),
		Summary: InsightsSummary{
			FilesWithMetadata:             make(map[string]int),
			FilesWithoutRequestedMetadata: make(map[string]int),
			NullValueCounts:               make(map[string]int),
		},
	}

	// 5. Get Total Files In Scope (Denominator for percentages)
	var totalFilesInScope int
	err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM target_set").Scan(&totalFilesInScope)
	if err != nil {
		return nil, fmt.Errorf("failed to query total files in scope: %w", err)
	}
	report.Summary.TotalFilesInScope = totalFilesInScope

	// 6. Process each requested field
	for _, fieldName := range fields {
		// 6a. Get field info (ID and Type)
		var fieldID, fieldType string
		fieldQuery := `SELECT field_id, field_type FROM metadata_fields WHERE field_name = ? LIMIT 1`
		err = database.QueryRowContext(ctx, fieldQuery, fieldName).Scan(&fieldID, &fieldType)
		if err != nil {
			if err == sql.ErrNoRows {
				logger.Warning("Field not found in database", "field", fieldName, "database", dbName)
				continue // Skip this field but continue processing others
			}
			return nil, fmt.Errorf("failed to query field info for '%s': %w", fieldName, err)
		}

		// 6b. Execute Aggregation Query based on type
		var rows *sql.Rows
		var queryErr error

		if fieldType == "array" || fieldType == "list" {
			// Array Type: Use json_each to expand values
			query := `
				SELECT json_each.value as value, COUNT(DISTINCT f.file_path) as count
				FROM file_metadata fm
				JOIN metadata_fields mf ON fm.field_id = mf.field_id
				JOIN files f ON fm.file_path = f.file_path
				JOIN target_set ts ON f.file_path = ts.file_path
				JOIN json_each(fm.field_value)
				WHERE mf.field_name = ?
				GROUP BY json_each.value
				ORDER BY count DESC
				LIMIT ?
			`
			rows, queryErr = database.QueryContext(ctx, query, fieldName, limit)
		} else {
			// Scalar Type: Standard GROUP BY
			query := `
				SELECT fm.field_value as value, COUNT(DISTINCT f.file_path) as count
				FROM file_metadata fm
				JOIN metadata_fields mf ON fm.field_id = mf.field_id
				JOIN files f ON fm.file_path = f.file_path
				JOIN target_set ts ON f.file_path = ts.file_path
				WHERE mf.field_name = ?
				GROUP BY fm.field_value
				ORDER BY count DESC
				LIMIT ?
			`
			rows, queryErr = database.QueryContext(ctx, query, fieldName, limit)
		}

		if queryErr != nil {
			return nil, fmt.Errorf("failed to execute aggregation query for field '%s': %w", fieldName, queryErr)
		}
		defer rows.Close()

		// 6c. Parse Results
		var insights []FieldInsight
		for rows.Next() {
			var insight FieldInsight
			if err := rows.Scan(&insight.Value, &insight.Count); err != nil {
				return nil, fmt.Errorf("failed to scan insight row for field '%s': %w", fieldName, err)
			}
			
			// Calculate percentage
			if totalFilesInScope > 0 {
				insight.Percentage = (float64(insight.Count) / float64(totalFilesInScope)) * 100
			}
			
			insights = append(insights, insight)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		report.Insights[fieldName] = insights

		// 6d. Calculate Summary Stats for this field
		// Count files with metadata (non-null values)
		var filesWithMeta int
		metaCountQuery := `
			SELECT COUNT(DISTINCT f.file_path)
			FROM file_metadata fm
			JOIN metadata_fields mf ON fm.field_id = mf.field_id
			JOIN files f ON fm.file_path = f.file_path
			JOIN target_set ts ON f.file_path = ts.file_path
			WHERE mf.field_name = ? AND fm.field_value IS NOT NULL AND fm.field_value != ''
		`
		if err := database.QueryRowContext(ctx, metaCountQuery, fieldName).Scan(&filesWithMeta); err != nil {
			logger.Warning("Failed to count files with metadata", "field", fieldName, "error", err)
			filesWithMeta = 0
		}
		report.Summary.FilesWithMetadata[fieldName] = filesWithMeta

		// Count files without metadata (null or empty)
		report.Summary.FilesWithoutRequestedMetadata[fieldName] = totalFilesInScope - filesWithMeta
		
		// Count null values specifically (files in target_set that have no entry in file_metadata for this field)
		var nullCount int
		nullQuery := `
			SELECT COUNT(DISTINCT ts.file_path)
			FROM target_set ts
			WHERE NOT EXISTS (
				SELECT 1
				FROM file_metadata fm
				JOIN metadata_fields mf ON fm.field_id = mf.field_id
				WHERE ts.file_path = fm.file_path AND mf.field_name = ?
			)
		`
		if err := database.QueryRowContext(ctx, nullQuery, fieldName).Scan(&nullCount); err != nil {
			logger.Warning("Failed to count null values", "field", fieldName, "error", err)
			nullCount = 0
		}
		report.Summary.NullValueCounts[fieldName] = nullCount
	}

	return report, nil
}

