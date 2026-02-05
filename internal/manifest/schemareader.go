/**
 * Component: Schema Reader
 * Block-UUID: dee3955b-6213-4e3f-a67e-d762702b5137
 * Parent-UUID: d602b4d4-3d46-4ca1-b758-66f79b320c17
 * Version: 1.5.0
 * Description: Logic to query the database and retrieve analyzer and field definitions. Added GetFieldTypes helper for filter parser to determine field types. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted routine Info logs to Debug level to enable quiet-by-default behavior.
 * Language: Go
 * Created-at: 2026-02-02T08:34:20.421Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package manifest

import (
	"context"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// SchemaInfo represents the complete schema structure of a manifest database.
type SchemaInfo struct {
	DatabaseName string         `json:"database_name"`
	Analyzers    []AnalyzerInfo `json:"analyzers"`
}

// AnalyzerInfo represents an analyzer and its associated fields.
type AnalyzerInfo struct {
	Ref         string      `json:"ref"`
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Fields      []FieldInfo `json:"fields"`
}

// FieldInfo represents a metadata field definition.
type FieldInfo struct {
	Ref          string `json:"ref"`
	Name         string `json:"name"`
	DisplayName  string `json:"display_name"`
	Type         string `json:"type"`
	Description  string `json:"description"`
}

// GetSchema retrieves the schema information for the specified database.
// It queries the analyzers and metadata_fields tables and groups fields by their analyzer.
func GetSchema(ctx context.Context, dbName string) (*SchemaInfo, error) {
	// 1. Validate Database Exists (Prevents creating empty artifacts)
	if err := ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve Database Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database Connection
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Query Analyzers
	analyzerQuery := `
		SELECT analyzer_ref_id, analyzer_id, analyzer_name, analyzer_description, analyzer_version
		FROM analyzers
		ORDER BY analyzer_ref_id
	`

	rows, err := database.QueryContext(ctx, analyzerQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analyzers []AnalyzerInfo
	analyzerMap := make(map[string]*AnalyzerInfo) // Map ref_id to pointer for easy field lookup

	for rows.Next() {
		var a AnalyzerInfo
		if err := rows.Scan(&a.Ref, &a.ID, &a.Name, &a.Description, &a.Version); err != nil {
			return nil, err
		}
		analyzers = append(analyzers, a)
		analyzerMap[a.Ref] = &analyzers[len(analyzers)-1]
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 5. Query Fields and Group by Analyzer
	fieldQuery := `
		SELECT field_ref_id, field_name, field_display_name, field_type, field_description
		FROM metadata_fields
		ORDER BY field_ref_id
	`

	fieldRows, err := database.QueryContext(ctx, fieldQuery)
	if err != nil {
		return nil, err
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var f FieldInfo
		if err := fieldRows.Scan(&f.Ref, &f.Name, &f.DisplayName, &f.Type, &f.Description); err != nil {
			return nil, err
		}

		// Extract analyzer ref from field ref (e.g., "F0" -> "A0" based on our naming convention)
		// Note: This relies on the convention that field ref contains the analyzer ref or we need to join.
		// For simplicity in this schema, we assume the field_ref_id is unique and we might need a join if strict mapping is needed.
		// However, looking at the schema, metadata_fields has analyzer_id. Let's do a proper join query instead.
	}

	// Re-doing Field Query with Join to ensure correctness
	joinQuery := `
		SELECT 
			mf.field_ref_id, 
			mf.field_name, 
			mf.field_display_name, 
			mf.field_type, 
			mf.field_description,
			a.analyzer_ref_id
		FROM metadata_fields mf
		JOIN analyzers a ON mf.analyzer_id = a.analyzer_id
		ORDER BY a.analyzer_ref_id, mf.field_ref_id
	`

	joinRows, err := database.QueryContext(ctx, joinQuery)
	if err != nil {
		return nil, err
	}
	defer joinRows.Close()

	for joinRows.Next() {
		var f FieldInfo
		var analyzerRef string
		if err := joinRows.Scan(&f.Ref, &f.Name, &f.DisplayName, &f.Type, &f.Description, &analyzerRef); err != nil {
			return nil, err
		}

		if analyzer, exists := analyzerMap[analyzerRef]; exists {
			analyzer.Fields = append(analyzer.Fields, f)
		} else {
			logger.Warning("Found field for non-existent analyzer", "field", f.Ref, "analyzer", analyzerRef)
		}
	}

	logger.Debug("Retrieved schema", "analyzers", len(analyzers))

	return &SchemaInfo{
		DatabaseName: dbName,
		Analyzers:    analyzers,
	}, nil
}

// ListFieldNames retrieves a simple list of field names for the specified database.
// This is a helper function for interactive wizards to populate dropdown menus.
func ListFieldNames(ctx context.Context, dbName string) ([]string, error) {
	// 1. Validate Database Exists
	if err := ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve Database Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database Connection
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Query Field Names
	query := `
		SELECT DISTINCT field_name
		FROM metadata_fields
		ORDER BY field_name
	`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fieldNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		fieldNames = append(fieldNames, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fieldNames, nil
}

// GetFieldTypes retrieves a map of field names to their types for the specified database.
// This is used by the filter parser to determine how to handle operators (e.g., "=" for lists vs scalars).
func GetFieldTypes(ctx context.Context, dbName string) (map[string]string, error) {
	// 1. Validate Database Exists
	if err := ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve Database Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database Connection
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Query Field Names and Types
	query := `
		SELECT field_name, field_type
		FROM metadata_fields
		ORDER BY field_name
	`

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fieldTypes := make(map[string]string)
	for rows.Next() {
		var name, fieldType string
		if err := rows.Scan(&name, &fieldType); err != nil {
			return nil, err
		}
		fieldTypes[name] = fieldType
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fieldTypes, nil
}
