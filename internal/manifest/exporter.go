/**
 * Component: Manifest Exporter
 * Block-UUID: 2f473067-b354-4f9c-a7a8-8ec0825a8e2e
 * Parent-UUID: 6a4ad865-f0ba-4680-bb5a-753343131f21
 * Version: 1.0.1
 * Description: Logic to export manifest database content to Markdown or JSON format.
 * Language: Go
 * Created-at: 2026-02-02T08:33:44.702Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1)
 */


package manifest

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
)

// ExportDatabase exports the database content to the specified format.
func ExportDatabase(ctx context.Context, dbName string, format string) (string, error) {
	// 1. Resolve DB Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return "", err
	}

	// 2. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return "", err
	}
	defer db.CloseDB(database)

	// 3. Fetch Data
	data, err := fetchExportData(ctx, database)
	if err != nil {
		return "", err
	}

	// 4. Format Output
	switch strings.ToLower(format) {
	case "markdown", "md":
		return formatMarkdown(data), nil
	case "json":
		return formatJSON(data)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// ExportData holds all data needed for export
type ExportData struct {
	ManifestInfo  map[string]interface{}
	Repositories  []map[string]interface{}
	Branches      []map[string]interface{}
	Analyzers     []map[string]interface{}
	Fields        []map[string]interface{}
	Files         []map[string]interface{}
	FileMetadata  []map[string]interface{}
}

// fetchExportData queries all tables and populates ExportData
func fetchExportData(ctx context.Context, db *sql.DB) (*ExportData, error) {
	data := &ExportData{}

	// Helper to scan rows
	scanRows := func(query string, dest *[]map[string]interface{}) error {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return err
		}

		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range columns {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return err
			}

			entry := make(map[string]interface{})
			for i, col := range columns {
				val := values[i]
				b, ok := val.([]byte)
				if ok {
					entry[col] = string(b)
				} else {
					entry[col] = val
				}
			}
			*dest = append(*dest, entry)
		}
		return rows.Err()
	}

	// Fetch Manifest Info
	data.ManifestInfo = make(map[string]interface{})
	row := db.QueryRowContext(ctx, "SELECT name, description, tags, version, created_at FROM manifest_info LIMIT 1")
	var name, desc, tags, version, createdAt string
	if err := row.Scan(&name, &desc, &tags, &version, &createdAt); err == nil {
		data.ManifestInfo["name"] = name
		data.ManifestInfo["description"] = desc
		data.ManifestInfo["tags"] = tags
		data.ManifestInfo["version"] = version
		data.ManifestInfo["created_at"] = createdAt
	}

	// Fetch other tables
	if err := scanRows("SELECT * FROM repositories", &data.Repositories); err != nil {
		return nil, err
	}
	if err := scanRows("SELECT * FROM branches", &data.Branches); err != nil {
		return nil, err
	}
	if err := scanRows("SELECT * FROM analyzers", &data.Analyzers); err != nil {
		return nil, err
	}
	if err := scanRows("SELECT * FROM metadata_fields", &data.Fields); err != nil {
		return nil, err
	}
	if err := scanRows("SELECT * FROM files", &data.Files); err != nil {
		return nil, err
	}
	if err := scanRows("SELECT * FROM file_metadata", &data.FileMetadata); err != nil {
		return nil, err
	}

	return data, nil
}

// formatMarkdown converts ExportData to a Markdown string
func formatMarkdown(data *ExportData) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Database: %s\n\n", data.ManifestInfo["name"]))
	sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", data.ManifestInfo["description"]))
	sb.WriteString(fmt.Sprintf("**Tags:** %s\n\n", data.ManifestInfo["tags"]))

	// Repositories
	sb.WriteString("## Repositories\n")
	for _, repo := range data.Repositories {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", repo["ref"], repo["name"]))
	}
	sb.WriteString("\n")

	// Branches
	sb.WriteString("## Branches\n")
	for _, branch := range data.Branches {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", branch["ref"], branch["name"]))
	}
	sb.WriteString("\n")

	// Analyzers
	sb.WriteString("## Analyzers\n")
	for _, analyzer := range data.Analyzers {
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", analyzer["analyzer_ref_id"], analyzer["analyzer_id"], analyzer["analyzer_name"]))
	}
	sb.WriteString("\n")

	// Fields
	sb.WriteString("## Fields\n")
	for _, field := range data.Fields {
		sb.WriteString(fmt.Sprintf("- **%s** (%s): %s - %s\n", field["field_ref_id"], field["field_id"], field["field_name"], field["field_description"]))
	}
	sb.WriteString("\n")

	// Files (Summary)
	sb.WriteString("## Files\n")
	sb.WriteString("| File Path | Language | Chat ID |\n")
	sb.WriteString("|-----------|----------|---------|\n")
	for _, file := range data.Files {
		sb.WriteString(fmt.Sprintf("| %s | %s | %v |\n", file["file_path"], file["language"], file["chat_id"]))
	}

	return sb.String()
}

// formatJSON converts ExportData to a JSON string
func formatJSON(data *ExportData) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
