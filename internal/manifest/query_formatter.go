/**
 * Component: Query Output Formatter
 * Block-UUID: 95d7ab0b-b3bf-4e9f-a088-61ba8a1b1f41
 * Parent-UUID: b986d05e-e7e9-4aec-ac5a-fc4b81e86142
 * Version: 2.1.0
 * Description: Formats query results, list results, and status views. Updated to support active profiles, quiet mode, TTY-aware decoration stripping, and prominent workspace headers.
 * Language: Go
 * Created-at: 2026-02-02T19:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourusername/gsc-cli/internal/output"
)

// FormatQueryResults formats a slice of QueryResult into the specified format.
// Updated to accept config for workspace header generation.
func FormatQueryResults(results []QueryResult, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatQueryResultsJSON(results)
	case "table":
		return formatQueryResultsTable(results, quiet, config)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatQueryResultsJSON(results []QueryResult) string {
	bytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func formatQueryResultsTable(results []QueryResult, quiet bool, config *QueryConfig) string {
	if len(results) == 0 {
		return "No results found."
	}

	headers := []string{"File Path", "Chat ID"}
	var rows [][]string

	for _, r := range results {
		rows = append(rows, []string{r.FilePath, fmt.Sprintf("%d", r.ChatID)})
	}

	table := output.FormatTable(headers, rows)
	
	if quiet {
		return table
	}

	// Check if we are in a terminal
	if output.IsTerminal() {
		// Prepend the prominent header
		header := FormatWorkspaceHeader(config)
		return fmt.Sprintf("%s%s\n[Context: %s] | Switch: gsc config use <name>", 
			header, table, getActiveProfileName())
	}

	// Fallback to simple header if piping
	return fmt.Sprintf("[Context: %s]\n%s\n[Context: %s] | Switch: gsc config use <name>", 
		getActiveProfileName(), table, getActiveProfileName())
}

// FormatListResult formats a ListResult into the specified format.
func FormatListResult(listResult *ListResult, format string, quiet bool) string {
	switch strings.ToLower(format) {
	case "json":
		return formatListResultJSON(listResult)
	case "table":
		return formatListResultTable(listResult, quiet)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatListResultJSON(listResult *ListResult) string {
	bytes, err := json.MarshalIndent(listResult, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func formatListResultTable(listResult *ListResult, quiet bool) string {
	if len(listResult.Items) == 0 {
		return "No items found."
	}

	var headers []string
	var rows [][]string
	var footer string

	switch listResult.Level {
	case "database":
		headers = []string{"Name", "Description", "Files"}
		for _, item := range listResult.Items {
			rows = append(rows, []string{
				item.Name,
				item.Description,
				fmt.Sprintf("%d", item.Count),
			})
		}
		footer = "Hint: Use 'gsc query --db <name> --list' to see fields in a database."
	case "field":
		headers = []string{"Field Name", "Type", "Description"}
		for _, item := range listResult.Items {
			rows = append(rows, []string{
				item.Name,
				item.Type,
				item.Description,
			})
		}
		footer = "Hint: Use 'gsc query --field <name> --list' to see values for a field.\nHint: Use 'gsc query --list-db' to see all databases."
	case "value":
		headers = []string{"Value", "Count"}
		for _, item := range listResult.Items {
			rows = append(rows, []string{
				item.Name,
				fmt.Sprintf("%d", item.Count),
			})
		}
		footer = "Hint: Use 'gsc query --value <val>' to find files.\nHint: Use 'gsc query --list' to go back to fields."
	default:
		return fmt.Sprintf("Unknown list level: %s", listResult.Level)
	}

	table := output.FormatTable(headers, rows)
	
	if quiet {
		return table
	}

	return fmt.Sprintf("%s\n%s\n", table, footer)
}

// FormatStatusView formats the current query context as a status view.
func FormatStatusView(config *QueryConfig, quiet bool) string {
	if quiet {
		// In quiet mode, just output the active profile name or "none"
		if config.ActiveProfile == "" {
			return "none"
		}
		return config.ActiveProfile
	}

	var sb strings.Builder

	sb.WriteString("Current Workspace:\n")
	sb.WriteString(fmt.Sprintf("  Active Profile: %s\n", getStatusValue(config.ActiveProfile)))
	sb.WriteString(fmt.Sprintf("  Database:       %s\n", getStatusValue(config.Global.DefaultDatabase)))
	sb.WriteString(fmt.Sprintf("  Field:          %s\n", getStatusValue(config.Query.DefaultField)))
	sb.WriteString(fmt.Sprintf("  Format:         %s\n", getStatusValue(config.Query.DefaultFormat)))
	sb.WriteString("\n")
	sb.WriteString("Need help? Run 'gsc query --help' for detailed documentation.\n")
	sb.WriteString("\n")
	sb.WriteString("Quick Actions:\n")
	sb.WriteString("  - Run 'gsc query --list' to see fields in the default database (or list all DBs).\n")
	sb.WriteString("  - Run 'gsc query --list-db' to explicitly list all databases.\n")
	sb.WriteString("  - Run 'gsc config context list' to see available profiles.\n")
	sb.WriteString("  - Run 'gsc config use <name>' to switch context.\n")
	sb.WriteString("  - Run 'gsc query --value <val>' to search using defaults.\n")

	return sb.String()
}

func getStatusValue(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

// getActiveProfileName is a helper to get the profile name from the current config.
// It attempts to load the config to find the active profile name.
func getActiveProfileName() string {
	config, err := LoadConfig()
	if err != nil {
		return "unknown"
	}
	if config.ActiveProfile == "" {
		return "default"
	}
	return config.ActiveProfile
}
