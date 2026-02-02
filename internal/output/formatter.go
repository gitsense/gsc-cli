/*
 * Component: Output Formatter
 * Block-UUID: 38043fb1-7c08-45fb-9c68-f0f648c5a060
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides utility functions to format data into JSON, Table, or CSV strings for CLI output.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

// FormatJSON marshals the provided data interface into a formatted JSON string.
func FormatJSON(data interface{}) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
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

// FormatCSV converts headers and rows into a CSV formatted string.
func FormatCSV(headers []string, rows [][]string) (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Write headers
	if err := writer.Write(headers); err != nil {
		return "", fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write rows
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("error flushing CSV writer: %w", err)
	}

	return sb.String(), nil
}
