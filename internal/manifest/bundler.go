/**
 * Component: Manifest Bundler
 * Block-UUID: 6506e1b6-236a-4b99-8db6-2764ef27819c
 * Parent-UUID: 171f5bce-ad84-4e6c-9a8e-6c3ce9dac0c0
 * Version: 1.1.1
 * Description: Logic to generate context bundles from SQL queries against a manifest database. Fixed integer conversion to handle SQLite int64 properly.
 * Language: Go
 * Created-at: 2026-02-02T08:33:07.479Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.1.1)
 */


package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// CreateBundle executes a SQL query and formats the results as a context bundle.
func CreateBundle(ctx context.Context, dbName string, query string, format string) (string, error) {
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

	// 3. Execute Query
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// 4. Scan Results
	var files []BundleFile
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	for rows.Next() {
		// Create a slice of interface{} to hold the column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		// Map columns to struct fields
		file := BundleFile{}
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				val = string(b)
			}

			switch strings.ToLower(col) {
			case "file_path":
				file.FilePath = fmt.Sprintf("%v", val)
			case "chat_id":
				// SQLite returns int64 for integers
				if intVal, ok := val.(int64); ok {
					file.ChatID = int(intVal)
				} else {
					logger.Warning("Failed to convert chat_id to int: %v (type: %T)", val, val)
					file.ChatID = 0
				}
			}
		}

		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	// 5. Format Output
	switch strings.ToLower(format) {
	case "context-list":
		return formatContextList(files), nil
	case "json":
		return formatBundleJSON(files)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// BundleFile represents a file entry in a bundle
type BundleFile struct {
	FilePath string `json:"file_path"`
	ChatID   int    `json:"chat_id"`
}

// formatContextList formats files as "filename.ext (chat-id: 123)"
func formatContextList(files []BundleFile) string {
	var sb strings.Builder
	for _, file := range files {
		sb.WriteString(fmt.Sprintf("%s (chat-id: %d)\n", file.FilePath, file.ChatID))
	}
	return sb.String()
}

// formatBundleJSON formats files as a JSON array
func formatBundleJSON(files []BundleFile) (string, error) {
	bytes, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
