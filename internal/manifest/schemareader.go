/*
 * Component: Schema Reader
 * Block-UUID: d0b7da07-3274-4d28-8e14-4ae769a867d9
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Logic to query the database and retrieve analyzer and field definitions for the schema command.
 * Language: Go
 * Created-at: 2026-02-02T07:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"context"
	"database/sql"

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
	// 1. Resolve Database Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 2. Open Database Connection
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 3. Query Analyzers
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

	// 4. Query Fields and Group by Analyzer
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
			logger.Warning("Found field %s for non-existent analyzer %s", f.Ref, analyzerRef)
		}
	}

	logger.Info("Retrieved schema for %d analyzers", len(analyzers))

	return &SchemaInfo{
		DatabaseName: dbName,
		Analyzers:    analyzers,
	}, nil
}
