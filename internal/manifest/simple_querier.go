/*
 * Component: Simple Query Executor
 * Block-UUID: 38f0fdbb-0a0b-4fc0-b4ca-0104172f138f
 * Parent-UUID: fcb4dd65-2763-4bbb-acb1-83a6253fa67d
 * Version: 1.3.0
 * Description: Executes simple value-matching queries and hierarchical list operations. Added ExecuteCoverageAnalysis to implement Phase 3 Scout Layer coverage reporting, utilizing temporary tables for efficient Git-to-DB comparison.
 * Language: Go
 * Created-at: 2026-02-05T07:13:46.193Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0)
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
	"github.com/yourusername/gsc-cli/internal/registry"
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

	var query string
	placeholders := strings.Repeat("?,", len(values))
	placeholders = placeholders[:len(placeholders)-1] // Remove trailing comma

	if fieldType == "array" || fieldType == "list" {
		// JSON Array Query: Check if any of the target values exist in the JSON array
		query = fmt.Sprintf(`
			SELECT f.file_path, f.chat_id
			FROM files f
			INNER JOIN file_metadata fm ON f.file_path = fm.file_path
			WHERE fm.field_id = ?
			  AND EXISTS (
				  SELECT 1 FROM json_each(fm.field_value)
				  WHERE json_each.value IN (%s)
			  )
		`, placeholders)
	} else {
		// Scalar Query: Standard exact match
		query = fmt.Sprintf(`
			SELECT f.file_path, f.chat_id
			FROM files f
			INNER JOIN file_metadata fm ON f.file_path = fm.file_path
			WHERE fm.field_id = ?
			  AND fm.field_value IN (%s)
		`, placeholders)
	}

	for _, v := range values {
		args = append(args, strings.TrimSpace(v))
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
// - If dbName is empty: Lists all databases.
// - If dbName is set but fieldName is empty: Lists fields in the database.
// - If both are set: Lists unique values for the field.
func GetListResult(ctx context.Context, dbName string, fieldName string) (*ListResult, error) {
	// Level 1: List Databases
	if dbName == "" {
		return listAllDatabases(ctx)
	}

	// Level 2: List Fields
	if fieldName == "" {
		return listFieldsInDatabase(ctx, dbName)
	}

	// Level 3: List Values
	return listValuesForField(ctx, dbName, fieldName)
}

// listAllDatabases returns a list of all registered databases.
func listAllDatabases(ctx context.Context) (*ListResult, error) {
	reg, err := registry.LoadRegistry()
	if err != nil {
		return nil, err
	}

	var items []ListItem
	for _, entry := range reg.Databases {
		items = append(items, ListItem{
			Name:        entry.Name,
			Description: entry.Description,
			Count:       0, // TODO: Query DB for actual file count
		})
	}

	return &ListResult{
		Level: "database",
		Items: items,
	}, nil
}

// listFieldsInDatabase returns a list of all fields in the specified database.
func listFieldsInDatabase(ctx context.Context, dbName string) (*ListResult, error) {
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

	return &ListResult{
		Level: "field",
		Items: items,
	}, nil
}

// listValuesForField returns a list of unique values for the specified field in the database.
func listValuesForField(ctx context.Context, dbName string, fieldName string) (*ListResult, error) {
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

	return &ListResult{
		Level: "value",
		Items: items,
	}, nil
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
