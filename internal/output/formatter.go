/**
 * Component: Output Formatter
 * Block-UUID: 8fcb5580-1ca1-45fd-96e1-918713c3f1f2
 * Parent-UUID: 024969de-ce14-4b2c-9082-25bb43166c43
 * Version: 1.17.0
 * Description: Removed FormatContractInfo and FormatContractTest to resolve import cycle. These functions have been moved to internal/contract/manager.go.
 * Language: Go
 * Created-at: 2026-03-26T20:42:52.215Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), Gemini 3 Flash (v1.7.0), Gemini 3 Flash (v1.8.0), GLM-4.7 (v1.9.0), Gemini 3 Flash (v1.10.0), Gemini 3 Flash (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v1.16.0), GLM-4.7 (v1.17.0)
 */


package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/types/contract"
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

// ContractDisplay represents a contract for listing purposes.
type ContractDisplay struct {
	UUID        string
	Description string
	Workdirs    []contract.WorkdirEntry
	Status      string
	ExpiresAt   string
}

// FormatContractList formats a list of contracts into a table.
func FormatContractList(contracts []ContractDisplay) string {
	if len(contracts) == 0 {
		return "No contracts found."
	}

	headers := []string{"UUID", "Description", "Primary Workdir", "Status", "Expires"}
	
	rows := make([][]string, len(contracts))
	for i, c := range contracts {
		workdirPath := ""
		if len(c.Workdirs) > 0 {
			workdirPath = c.Workdirs[0].Path
		}
		rows[i] = []string{
			c.UUID,
			truncate(c.Description, 30),
			truncate(workdirPath, 40),
			c.Status,
			c.ExpiresAt,
		}
	}

	return FormatTable(headers, rows)
}

// ProvenanceDisplay represents a provenance entry for listing purposes.
type ProvenanceDisplay struct {
	Timestamp string
	Action    string
	FilePath  string
	Version   string
	Status    string
}

// FormatProvenanceList formats a list of provenance entries into a table.
func FormatProvenanceList(entries []ProvenanceDisplay) string {
	if len(entries) == 0 {
		return "No provenance entries found."
	}

	headers := []string{"Timestamp", "Action", "File", "Version", "Status"}
	
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{
			e.Timestamp,
			e.Action,
			truncate(e.FilePath, 40),
			e.Version,
			e.Status,
		}
	}

	return FormatTable(headers, rows)
}

// FormatContractStatus formats a single contract for the 'status' command.
// It provides a detailed, human-readable view of the contract's state.
func FormatContractStatus(c ContractDisplay) string {
	var sb strings.Builder

	sb.WriteString("Contract Status\n\n")
	sb.WriteString(fmt.Sprintf("  UUID:         %s\n", c.UUID))
	sb.WriteString(fmt.Sprintf("  Description:  %s\n", c.Description))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", c.Status))
	sb.WriteString(fmt.Sprintf("  Expires At:   %s\n\n", c.ExpiresAt))

	if len(c.Workdirs) > 0 {
		primary := c.Workdirs[0]
		sb.WriteString(fmt.Sprintf("  Primary Workdir:   %s (%s)\n", filepath.Base(primary.Path), primary.Path))
		if len(c.Workdirs) > 1 {
			sb.WriteString("  Secondary Workdirs:\n")
			for i := 1; i < len(c.Workdirs); i++ {
				sec := c.Workdirs[i]
				sb.WriteString(fmt.Sprintf("    - %s (%s)\n", sec.Name, sec.Path))
			}
		}
	}

	return sb.String()
}
