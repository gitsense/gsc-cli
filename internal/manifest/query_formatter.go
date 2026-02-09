/**
 * Component: Query Output Formatter
 * Block-UUID: cd3b09e8-2e43-4212-aad6-c58fab6c1fc2
 * Parent-UUID: dbe56bd5-3a80-42f8-abe1-2c003970a03f
 * Version: 3.0.0
 * Description: Updated all hints, quick actions, and status views to reflect the promotion of --list, --insights, and --coverage to subcommands. This ensures the CLI output guides users toward the new ergonomic command structure.
 * Language: Go
 * Created-at: 2026-02-09T00:36:02.868Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), Gemini 3 Flash (v2.3.0), GLM-4.7 (v2.4.0), GLM-4.7 (v2.5.0), GLM-4.7 (v2.5.1), Gemini 3 Flash (v2.6.0), Gemini 3 Flash (v2.7.0), GLM-4.7 (v2.7.1), Claude Haiku 4.5 (v2.8.0), Gemini 3 Flash (v2.9.0), Gemini 3 Flash (v2.9.1), Gemini 3 Flash (v2.9.2), Gemini 3 Flash (v3.0.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yourusername/gsc-cli/internal/output"
)

// FormatQueryResults formats a slice of QueryResult into the specified format.
func FormatQueryResults(response *QueryResponse, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatQueryResultsJSON(response)
	case "table":
		return formatQueryResultsTable(response, quiet, config)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatQueryResultsJSON(response *QueryResponse) string {
	bytes, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func formatQueryResultsTable(response *QueryResponse, quiet bool, config *QueryConfig) string {
	if response == nil || len(response.Results) == 0 {
		return "No results found."
	}

	headers := []string{"File Path", "Chat ID"}
	var rows [][]string

	for _, r := range response.Results {
		rows = append(rows, []string{r.FilePath, fmt.Sprintf("%d", r.ChatID)})
	}

	table := output.FormatTable(headers, rows)
	
	if quiet {
		return table
	}

	var sb strings.Builder

	if output.IsTerminal() {
		sb.WriteString(FormatWorkspaceHeader(config, quiet))
		sb.WriteString(fmt.Sprintf("Database: %s\n", response.Summary.Database))
		sb.WriteString(fmt.Sprintf("   Query: %s = %s\n\n", response.Query.MatchField, response.Query.MatchValue))
	} else {
		sb.WriteString(fmt.Sprintf("[Context: %s]\n", getActiveProfileName()))
	}

	sb.WriteString(table)

	if !quiet {
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("Coverage Analysis (Confidence: %s)\n", response.Summary.Confidence))
		sb.WriteString("----------------------------------------------------------\n")
		sb.WriteString(fmt.Sprintf("Focus Coverage:    %s %.1f%% (%d results)\n\n", 
			renderProgressBar(response.Summary.CoveragePercent), 
			response.Summary.CoveragePercent, 
			response.Summary.TotalResults))
		sb.WriteString("Hint: Run 'gsc query coverage' for a detailed breakdown of blind spots.\n\n")
		sb.WriteString(fmt.Sprintf("[Context: %s] | Switch: gsc config use <name>", getActiveProfileName()))
	}

	return sb.String()
}

// FormatListResult formats a ListResult into the specified format.
func FormatListResult(listResult *ListResult, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatListResultJSON(listResult)
	case "table":
		return formatListResultTable(listResult, quiet, config)
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

func formatListResultTable(listResult *ListResult, quiet bool, config *QueryConfig) string {
	if len(listResult.Items) == 0 {
		return "No items found."
	}

	var sb strings.Builder

	if !quiet && output.IsTerminal() {
		sb.WriteString(FormatWorkspaceHeader(config, quiet))
	}

	var headers []string
	var rows [][]string
	var footer string

	switch listResult.Level {
	case "database":
		headers = []string{"DB Name", "Summary", "DB File", "File Count"}
		for _, item := range listResult.Items {
			summary := item.Description
			if len(summary) > 60 {
				summary = summary[:57] + "..."
			}
			rows = append(rows, []string{
				item.Name,
				summary,
				item.Source,
				fmt.Sprintf("%d", item.Count),
			})
		}
		footer = "Hint: Use 'gsc query list --db <name>' to see fields in a database."
	case "field":
		headers = []string{"Field Name", "Type", "Description"}
		for _, item := range listResult.Items {
			rows = append(rows, []string{
				item.Name,
				item.Type,
				truncate(item.Description, 80),
			})
		}
		footer = "Hint: Use 'gsc query list <field>' to see values for a field.\nHint: Use 'gsc query list --dbs' to see all databases."
	case "value":
		headers = []string{"Value", "Count"}
		for _, item := range listResult.Items {
			rows = append(rows, []string{
				item.Name,
				fmt.Sprintf("%d", item.Count),
			})
		}
		footer = "Hint: Use 'gsc query --value <val>' to find files.\nHint: Use 'gsc query list' to go back to fields."
	default:
		return fmt.Sprintf("Unknown list level: %s", listResult.Level)
	}

	table := output.FormatTable(headers, rows)
	sb.WriteString(table)
	
	if !quiet {
		sb.WriteString("\n")
		sb.WriteString(footer)
	}

	return sb.String()
}

// FormatStatusView formats the current query context as a status view.
func FormatStatusView(config *QueryConfig, quiet bool) string {
	if quiet {
		if config.ActiveProfile == "" {
			return "none"
		}
		return config.ActiveProfile
	}

	var sb strings.Builder

	sb.WriteString("Find files by metadata value or analyze codebase coverage and insights.\n\n")
	sb.WriteString(FormatWorkspaceHeader(config, quiet))

	sb.WriteString("Primary Subcommands:\n")
	sb.WriteString("  list [field]       Discover fields or values (hierarchical)\n")
	sb.WriteString("  insights           Analyze metadata distribution (Phase 2)\n")
	sb.WriteString("  coverage           Analyze analysis blind spots (Phase 3)\n\n")

	sb.WriteString("Primary Flags:\n")
	sb.WriteString("  -v, --value <val>  Match metadata value (comma-separated for OR)\n")
	sb.WriteString("  -d, --db <name>    Override default database\n")
	sb.WriteString("  -f, --field <name> Override default field\n\n")

	sb.WriteString("Quick Actions:\n")
	sb.WriteString("  • List fields:     gsc query list\n")
	sb.WriteString("  • View insights:   gsc query insights --field <field>\n")
	sb.WriteString("  • Check coverage:  gsc query coverage\n\n")

	sb.WriteString("Need more help? Run 'gsc query --help' for the full manual.\n")

	return sb.String()
}

// FormatCoverageReport formats a CoverageReport into the specified format.
func FormatCoverageReport(report *CoverageReport, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatCoverageJSON(report)
	case "table":
		return formatCoverageTable(report, quiet, config)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatCoverageJSON(report *CoverageReport) string {
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func formatCoverageTable(report *CoverageReport, quiet bool, config *QueryConfig) string {
	var sb strings.Builder

	dbName := config.Global.DefaultDatabase
	if dbName == "" {
		dbName = "unknown"
	}

	sb.WriteString(fmt.Sprintf("GitSense Coverage Report: %s\n", dbName))
	sb.WriteString(strings.Repeat("=", 41+len(dbName)) + "\n")
	sb.WriteString(fmt.Sprintf("Active Profile: %s\n", getStatusValue(report.ActiveProfile)))
	
	scopeStr := "All tracked files"
	if report.ScopeDefinition != nil {
		scopeStr = fmt.Sprintf("Include %v | Exclude %v", report.ScopeDefinition.Include, report.ScopeDefinition.Exclude)
	}
	sb.WriteString(fmt.Sprintf("Focus Scope: %s\n", scopeStr))
	sb.WriteString(fmt.Sprintf("Report Generated: %s\n\n", report.Timestamp.Format(time.RFC3339)))

	sb.WriteString("Overall Coverage\n")
	sb.WriteString("----------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("Total Tracked Files: %8d\n", report.Totals.TrackedFiles))
	sb.WriteString(fmt.Sprintf("In-Scope Files:      %8d\n", report.Totals.InScopeFiles))
	sb.WriteString(fmt.Sprintf("Analyzed Files:      %8d\n", report.Totals.AnalyzedFiles))
	sb.WriteString(fmt.Sprintf("Out-of-Scope Files:  %8d\n\n", report.Totals.OutOfScopeFiles))

	sb.WriteString("Coverage Percentages\n")
	sb.WriteString("----------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("Focus Coverage:    %s %.1f%% (%d/%d in-scope)\n", 
		renderProgressBar(report.Percentages.FocusCoverage), 
		report.Percentages.FocusCoverage, 
		report.Totals.AnalyzedFiles, 
		report.Totals.InScopeFiles))
	sb.WriteString(fmt.Sprintf("Total Coverage:    %s %.1f%% (%d/%d total)\n\n", 
		renderProgressBar(report.Percentages.TotalCoverage), 
		report.Percentages.TotalCoverage, 
		report.Totals.AnalyzedFiles, 
		report.Totals.TrackedFiles))

	sb.WriteString("Coverage by Language\n")
	sb.WriteString("----------------------------------------------------------\n")
	
	var langs []string
	for l := range report.ByLanguage {
		langs = append(langs, l)
	}
	sort.Slice(langs, func(i, j int) bool {
		return report.ByLanguage[langs[i]].Total > report.ByLanguage[langs[j]].Total
	})

	for _, l := range langs {
		stats := report.ByLanguage[l]
		sb.WriteString(fmt.Sprintf("%-11s %s %5.1f%% (%d/%d)\n", 
			l+":", 
			renderProgressBar(stats.Percent), 
			stats.Percent, 
			stats.Analyzed, 
			stats.Total))
	}
	sb.WriteString("\n")

	sb.WriteString("Top Unanalyzed Directories\n")
	sb.WriteString("----------------------------------------------------------\n")
	if len(report.BlindSpots.Directories) == 0 {
		sb.WriteString("No blind spots detected in scope.\n")
	} else {
		for _, ds := range report.BlindSpots.Directories {
			sb.WriteString(fmt.Sprintf("%-25s (%2d files, %3.0f%% analyzed)\n", 
				ds.Path, ds.TotalFiles, ds.Percent))
		}
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("Analysis Status: %s\n", strings.ToUpper(report.AnalysisStatus)))
	sb.WriteString("----------------------------------------------------------\n")
	for _, rec := range report.Recommendations {
		if strings.Contains(rec, "High confidence") {
			sb.WriteString("✓ " + rec + "\n")
		} else if strings.Contains(rec, "partial") {
			sb.WriteString("⚠ " + rec + "\n")
		} else {
			sb.WriteString("→ " + rec + "\n")
		}
	}
	sb.WriteString("\nHint: Use 'gsc query insights' to see metadata distribution.\n")

	return sb.String()
}

func FormatInsightsReport(report *InsightsReport, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatInsightsJSON(report)
	case "table":
		return formatReportTable(report, quiet, config)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatInsightsJSON(report *InsightsReport) string {
	bytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func FormatReport(report *InsightsReport, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatInsightsJSON(report)
	case "table":
		return formatReportTable(report, quiet, config)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatReportTable(report *InsightsReport, quiet bool, config *QueryConfig) string {
	var sb strings.Builder

	dbName := config.Global.DefaultDatabase
	if dbName == "" {
		dbName = report.Context.Database
	}

	sb.WriteString(fmt.Sprintf("GitSense Intelligence Report: %s\n", dbName))
	sb.WriteString(strings.Repeat("=", 41+len(dbName)) + "\n")
	sb.WriteString(fmt.Sprintf("Active Profile: %s\n", getStatusValue(config.ActiveProfile)))
	
	scopeStr := "All tracked files"
	if report.Context.ScopeDefinition != nil {
		scopeStr = fmt.Sprintf("Include %v | Exclude %v", report.Context.ScopeDefinition.Include, report.Context.ScopeDefinition.Exclude)
	}
	sb.WriteString(fmt.Sprintf("Focus Scope: %s\n", scopeStr))
	sb.WriteString(fmt.Sprintf("Report Generated: %s\n\n", report.Context.Timestamp.Format(time.RFC3339)))

	totalFiles := report.Summary.TotalFilesInScope
	analyzedCount := 0
	if len(report.Summary.FilesWithMetadata) > 0 {
		for _, count := range report.Summary.FilesWithMetadata {
			if count > analyzedCount {
				analyzedCount = count
			}
		}
	}
	
	percent := 0.0
	if totalFiles > 0 {
		percent = (float64(analyzedCount) / float64(totalFiles)) * 100
	}
	
	sb.WriteString(fmt.Sprintf("Status: %d/%d In-Scope Files Analyzed (%.0f%%)\n\n", analyzedCount, totalFiles, percent))

	var fieldNames []string
	for name := range report.Insights {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)

	for _, fieldName := range fieldNames {
		insights := report.Insights[fieldName]
		
		maxValWidth := 0
		for _, insight := range insights {
			displayValue := insight.Value
			if displayValue == "" {
				displayValue = "(unrated)"
			}
			if len(displayValue) > 100 {
				displayValue = displayValue[:97] + "..."
			}
			if len(displayValue) > maxValWidth {
				maxValWidth = len(displayValue)
			}
		}
		
		sb.WriteString(fmt.Sprintf("Field: %s (Top %d)\n", fieldName, report.Context.Limit))
		sb.WriteString("----------------------------------------------------------\n")
		
		for _, insight := range insights {
			displayValue := insight.Value
			if displayValue == "" {
				displayValue = "(unrated)"
			}
			if len(displayValue) > 100 {
				displayValue = displayValue[:97] + "..."
			}
			sb.WriteString(fmt.Sprintf("%-*s %s %5.1f%% (%d files)\n", 
				maxValWidth,
				displayValue, 
				renderProgressBar(insight.Percentage), 
				insight.Percentage, 
				insight.Count))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Metadata Completeness\n")
	sb.WriteString("----------------------------------------------------------\n")
	for _, fieldName := range fieldNames {
		withMeta := report.Summary.FilesWithMetadata[fieldName]
		
		completeness := 0.0
		if totalFiles > 0 {
			completeness = (float64(withMeta) / float64(totalFiles)) * 100
		}
		
		sb.WriteString(fmt.Sprintf("%-20s: %5.1f%% of in-scope files have values\n", fieldName, completeness))
	}
	sb.WriteString("\n")

	sb.WriteString("Hint: Use 'gsc grep <term> --filter \"<field>=<value>\"' to investigate.\n")
	sb.WriteString("Hint: Run 'gsc query coverage' for detailed coverage analysis.\n")

	return sb.String()
}

func renderProgressBar(percent float64) string {
	width := 20
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("#", filled) + strings.Repeat(" ", width-filled)
	return "[" + bar + "]"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

func getStatusValue(value string) string {
	if value == "" {
		return "(none)"
	}
	return value
}

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
