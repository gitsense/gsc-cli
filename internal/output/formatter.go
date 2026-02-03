/*
 * Component: Output Formatter
 * Block-UUID: bfb52c34-495b-4680-8ba5-1571715364b5
 * Parent-UUID: 22b52dbd-bb89-49f6-bab3-1d9d6173cb13
 * Version: 1.2.0
 * Description: Provides utility functions to format data into JSON, Table, or CSV strings for CLI output. Added IsTerminal helper for TTY detection.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
)

// FormatJSON marshals the provided data interface into a formatted JSON string and prints it.
func FormatJSON(data interface{}) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// FormatTable constructs a text-based table from headers and rows.
// It automatically calculates column widths for alignment.
func FormatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, header := range headers {
		colWidths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Build separator line
	separator := "+"
	for _, w := range colWidths {
		separator += strings.Repeat("-", w+2) + "+"
	}

	var sb strings.Builder
	sb.WriteString(separator + "\n")

	// Write Header
	sb.WriteString("| ")
	for i, header := range headers {
		sb.WriteString(fmt.Sprintf("%-*s", colWidths[i], header) + " | ")
	}
	sb.WriteString("\n")
	sb.WriteString(separator + "\n")

	// Write Rows
	for _, row := range rows {
		sb.WriteString("| ")
		for i, cell := range row {
			// Handle cases where row might be shorter than headers
			val := ""
			if i < len(row) {
				val = cell
			}
			sb.WriteString(fmt.Sprintf("%-*s", colWidths[i], val) + " | ")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(separator + "\n")

	return sb.String()
}

// FormatCSV converts headers and rows into a CSV formatted string and prints it.
func FormatCSV(headers []string, rows [][]string) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Write headers
	if err := writer.Write(headers); err != nil {
		fmt.Printf("Error writing CSV headers: %v\n", err)
		return
	}

	// Write rows
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			fmt.Printf("Error writing CSV row: %v\n", err)
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		fmt.Printf("Error flushing CSV writer: %v\n", err)
		return
	}

	fmt.Print(sb.String())
}

// FormatDatabaseTable formats a slice of DatabaseInfo structs into a table and prints it.
// This is a convenience function for the list command.
func FormatDatabaseTable(databases []interface{}, format string) {
	if len(databases) == 0 {
		fmt.Println("No databases found.")
		return
	}

	switch format {
	case "json":
		FormatJSON(databases)
	case "csv":
		headers := []string{"Name", "Description", "Tags", "DB Path", "Entry Count"}
		rows := make([][]string, len(databases))
		for i, db := range databases {
			dbInfo := db.(map[string]interface{})
			tags := ""
			if tagList, ok := dbInfo["tags"].([]string); ok {
				tags = strings.Join(tagList, ", ")
			}
			rows[i] = []string{
				fmt.Sprintf("%v", dbInfo["name"]),
				fmt.Sprintf("%v", dbInfo["description"]),
				tags,
				fmt.Sprintf("%v", dbInfo["db_path"]),
				fmt.Sprintf("%v", dbInfo["entry_count"]),
			}
		}
		FormatCSV(headers, rows)
	case "table":
		headers := []string{"Name", "Description", "Tags", "DB Path", "Entry Count"}
		rows := make([][]string, len(databases))
		for i, db := range databases {
			dbInfo := db.(map[string]interface{})
			tags := ""
			if tagList, ok := dbInfo["tags"].([]string); ok {
				tags = strings.Join(tagList, ", ")
			}
			rows[i] = []string{
				fmt.Sprintf("%v", dbInfo["name"]),
				fmt.Sprintf("%v", dbInfo["description"]),
				tags,
				fmt.Sprintf("%v", dbInfo["db_path"]),
				fmt.Sprintf("%v", dbInfo["entry_count"]),
			}
		}
		fmt.Print(FormatTable(headers, rows))
	default:
		fmt.Printf("Unsupported format: %s\n", format)
	}
}

// IsTerminal checks if the output is being written to a terminal (TTY).
// This is used to determine whether to show decorative headers/footers.
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}
