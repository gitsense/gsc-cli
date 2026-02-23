/**
 * Component: Output Formatter
 * Block-UUID: fd3cc890-0e80-499d-a5a1-6f88a1f02824
 * Parent-UUID: 2bc306a2-ee33-4c1e-b2d7-dc580e2201ef
 * Version: 1.10.0
 * Description: Provides utility functions to format data into JSON, Table, or CSV strings. Added FormatMetadataYAML and FormatMetadataJSON to support the ripgrep metadata appendix. Updated FormatBridgeMarkdown to support Exit Code and N/A database handling for the exec command. Added FormatExecList for displaying saved execution outputs.
 * Language: Go
 * Created-at: 2026-02-15T02:49:39.077Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), Gemini 3 Flash (v1.8.0), GLM-4.7 (v1.9.0), Gemini 3 Flash (v1.10.0)
 */


package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
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

// truncate shortens a string to a maximum length, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Ensure we don't cut in the middle of a multibyte character
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
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

// GetTerminalWidth returns the width of the terminal, or 80 if it cannot be determined.
func GetTerminalWidth() int {
	if !IsTerminal() {
		return 80 // Default for non-terminal (piped output)
	}
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80
	}
	return width
}

// IsTerminal checks if the output is being written to a terminal (TTY).
// This is used to determine whether to show decorative headers/footers.
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// FormatBridgeMarkdown constructs the Markdown message for the CLI Bridge.
func FormatBridgeMarkdown(command string, duration time.Duration, dbName string, format string, output string, exitCode int) string {
	var sb strings.Builder

	sb.WriteString("## GSC CLI Output\n\n")
	sb.WriteString("| Property | Value |\n")
	sb.WriteString("| :--- | :--- |\n")
	sb.WriteString(fmt.Sprintf("| **Command** | `%s` |\n", command))
	sb.WriteString(fmt.Sprintf("| **Execution Time** | %v |\n", duration.Round(time.Millisecond)))
	
	// Only show Database if it is not "N/A"
	if dbName != "N/A" {
		sb.WriteString(fmt.Sprintf("| **Database** | `%s` |\n", dbName))
	}

	// Show Exit Code if provided (valid range 0-255, or -1 to hide)
	if exitCode >= 0 && exitCode <= 255 {
		statusIcon := "✅"
		if exitCode != 0 {
			statusIcon = "❌"
		}
		sb.WriteString(fmt.Sprintf("| **Exit Code** | %s %d |\n", statusIcon, exitCode))
	}

	sb.WriteString(fmt.Sprintf("| **Format** | %s |\n", strings.ToUpper(format)))
	sb.WriteString("\n")

	lang := "text"
	if strings.ToLower(format) == "json" {
		lang = "json"
	}

	sb.WriteString(fmt.Sprintf("```%s\n", lang))
	sb.WriteString(output)
	if !strings.HasSuffix(output, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n")

	return sb.String()
}

// ExecOutput represents a saved execution output for listing purposes.
// This struct is defined here to avoid circular dependencies with the exec package.
type ExecOutput struct {
	ID        string
	Command   string
	ExitCode  int
	Timestamp string
}

// FormatExecList formats a list of saved execution outputs into a table.
func FormatExecList(outputs []ExecOutput) string {
	if len(outputs) == 0 {
		return "No saved outputs found."
	}

	headers := []string{"ID", "Command", "Exit Code", "Timestamp"}
	
	rows := make([][]string, len(outputs))
	for i, out := range outputs {
		rows[i] = []string{
			out.ID,
			truncate(out.Command, 40),
			fmt.Sprintf("%d", out.ExitCode),
			out.Timestamp,
		}
	}

	return FormatTable(headers, rows)
}
