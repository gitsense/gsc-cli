/**
 * Component: Query Output Formatter
 * Block-UUID: b86ffdeb-2ea9-4d6e-92f7-ace17f641cf3
 * Parent-UUID: 2fbad423-a793-469f-9d3e-9ff30b47bcab
 * Version: 1.0.2
 * Description: Formats query results, list results, and status views. Updated to include footer hints for hierarchical navigation.
 * Language: Go
 * Created-at: 2026-02-02T19:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yourusername/gsc-cli/internal/output"
)

// FormatQueryResults formats a slice of QueryResult into the specified format.
func FormatQueryResults(results []QueryResult, format string) string {
	switch strings.ToLower(format) {
	case "json":
		return formatQueryResultsJSON(results)
	case "table":
		return formatQueryResultsTable(results)
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

func formatQueryResultsTable(results []QueryResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	headers := []string{"File Path", "Chat ID"}
	var rows [][]string

	for _, r := range results {
		rows = append(rows, []string{r.FilePath, fmt.Sprintf("%d", r.ChatID)})
	}

	return output.FormatTable(headers, rows)
}

// FormatListResult formats a ListResult into the specified format.
func FormatListResult(listResult *ListResult, format string) string {
	switch strings.ToLower(format) {
	case "json":
		return formatListResultJSON(listResult)
	case "table":
		return formatListResultTable(listResult)
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

func formatListResultTable(listResult *ListResult) string {
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
	return fmt.Sprintf("%s\n%s\n", table, footer)
}

// FormatStatusView formats the current query context as a status view.
func FormatStatusView(config *QueryConfig) string {
	var sb strings.Builder

	sb.WriteString("Current Query Context (from .gitsense/config.json):\n")
	sb.WriteString(fmt.Sprintf("  Database: %s\n", getStatusValue(config.Query.DefaultDatabase)))
	sb.WriteString(fmt.Sprintf("  Field:    %s\n", getStatusValue(config.Query.DefaultField)))
	sb.WriteString(fmt.Sprintf("  Format:   %s\n", getStatusValue(config.Query.DefaultFormat)))
	sb.WriteString("\n")
	sb.WriteString("Need help? Run 'gsc query --help' for detailed documentation.\n")
	sb.WriteString("\n")
	sb.WriteString("Quick Actions:\n")
	sb.WriteString("  - Run 'gsc query --list' to see fields in the default database (or list all DBs).\n")
	sb.WriteString("  - Run 'gsc query --list-db' to explicitly list all databases.\n")
	sb.WriteString("  - Run 'gsc query --set-default db=<name>' to set a default database.\n")
	sb.WriteString("  - Run 'gsc query --value <val>' to search using defaults.\n")
	sb.WriteString("  - Run 'gsc query --clear-default <key>' to reset.\n")

	return sb.String()
}

func getStatusValue(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}
