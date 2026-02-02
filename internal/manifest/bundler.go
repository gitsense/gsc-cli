
/*
 * Component: Manifest Bundler
 * Block-UUID: ab533514-9416-4ec7-a220-17494139a480
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Logic to generate context bundles from SQL queries against a manifest database.
 * Language: Go
 * Created-at: 2026-02-02T08:15:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"context"
	"database/sql"
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
				// Handle integer conversion
				if intVal, ok := val.(int64); ok {
					file.ChatID = int(intVal)
				} else if intVal, ok := val.(int); ok {
					file.ChatID = intVal
				} else {
					file.ChatID = 0 // Default or error
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
