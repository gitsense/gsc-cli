/**
 * Component: Query Output Formatter
 * Block-UUID: a824a26c-72a5-47a9-970e-8d3421cd7b39
 * Parent-UUID: 8dcb54b2-c07b-422b-b641-4164b44bab49
 * Version: 3.8.0
 * Description: Centralized schema formatting logic by adding 'FormatSchema'. This supports the new 'databases' convenience command and allows for consistent schema output (JSON, Table, CSV) across the CLI.
 * Language: Go
 * Created-at: 2026-02-15T02:50:00.486Z
 * Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v3.6.0), Gemini 3 Flash (v3.7.0), GLM-4.7 (v3.8.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/output"
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
		sb.WriteString("Hint: Run 'gsc query coverage' for a detailed breakdown of blind spots.\n")
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
	var sb strings.Builder

	switch listResult.Level {
	case "discovery":
		// Check if this is the --all view (databases have nested fields)
		isAllView := false
		if len(listResult.Databases) > 0 && len(listResult.Databases[0].Fields) > 0 {
			isAllView = true
		}

		if isAllView {
			sb.WriteString("GitSense Chat Intelligence Map (All Databases)\n")
			sb.WriteString("==============================================\n\n")
		} else {
			sb.WriteString("GitSense Chat Intelligence Map\n")
			sb.WriteString("==============================\n\n")
		}

		// Get terminal width for dynamic truncation
		termWidth := output.GetTerminalWidth()

		// 1. Render Databases
		if len(listResult.Databases) > 0 {
			if !isAllView {
				sb.WriteString("Available Databases:\n")
				sb.WriteString("-------------------\n")
			}

			for _, dbItem := range listResult.Databases {
				name := dbItem.Name
				if dbItem.Name == listResult.ActiveDatabase {
					name = name + " (active)"
				}
				
				sb.WriteString(fmt.Sprintf("%s\n", dbItem.ManifestName))
				sb.WriteString(fmt.Sprintf("    ○ %s\n", name))
				sb.WriteString(fmt.Sprintf("    ○ %s\n", dbItem.Description))
				sb.WriteString("\n")

				// If --all view, render fields immediately under the database
				if isAllView && len(dbItem.Fields) > 0 {
					maxFieldWidth := 0
					for _, f := range dbItem.Fields {
						if len(f.Name) > maxFieldWidth {
							maxFieldWidth = len(f.Name)
						}
					}

					for _, f := range dbItem.Fields {
						// Calculate available width for description
						// Format: "    name - description"
						availableDescWidth := termWidth - 4 - maxFieldWidth - 3
						if availableDescWidth < 20 {
							availableDescWidth = 20 // Minimum width
						}
						sb.WriteString(fmt.Sprintf("    %-*s - %s\n", 
							maxFieldWidth, 
							f.Name, 
							truncate(f.Description, availableDescWidth)))
					}
					// Add a blank line after fields for readability
					sb.WriteString("\n")
				}
			}
		}

		// 2. Render Fields (Standard Dashboard only)
		if !isAllView && len(listResult.Fields) > 0 {
			sb.WriteString(fmt.Sprintf("Available Fields (in '%s'):\n", listResult.ActiveDatabase))
			sb.WriteString("--------------------------------\n")
			
			maxNameWidth := 0
			for _, item := range listResult.Fields {
				if len(item.Name) > maxNameWidth {
					maxNameWidth = len(item.Name)
				}
			}

			for _, item := range listResult.Fields {
				// Calculate available width for description
				availableDescWidth := termWidth - 4 - maxNameWidth - 3
				if availableDescWidth < 20 {
					availableDescWidth = 20 // Minimum width
				}
				sb.WriteString(fmt.Sprintf("    %-*s - %s\n", 
					maxNameWidth, 
					item.Name, 
					truncate(item.Description, availableDescWidth)))
			}
			sb.WriteString("\n")
		}

	case "value":
		headers := []string{"Value", "Count"}
		var rows [][]string
		for _, item := range listResult.Values {
			rows = append(rows, []string{
				item.Name,
				fmt.Sprintf("%d", item.Count),
			})
		}
		sb.WriteString(output.FormatTable(headers, rows))
		sb.WriteString("\n")

	default:
		return fmt.Sprintf("Unknown list level: %s", listResult.Level)
	}

	// Render Hints
	if !quiet && len(listResult.Hints) > 0 {
		for i, hint := range listResult.Hints {
			prefix := "Hint: "
			if i > 0 {
				prefix = "      "
			}
			sb.WriteString(fmt.Sprintf("%s%s\n", prefix, hint))
		}
	}

	return sb.String()
}

// FormatSchema formats a SchemaInfo into the specified format.
func FormatSchema(schema *SchemaInfo, format string, quiet bool, config *QueryConfig) string {
	switch strings.ToLower(format) {
	case "json":
		return formatSchemaJSON(schema)
	case "table":
		return formatSchemaTable(schema, quiet, config)
	case "csv":
		return formatSchemaCSV(schema)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatSchemaJSON(schema *SchemaInfo) string {
	bytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func formatSchemaTable(schema *SchemaInfo, quiet bool, config *QueryConfig) string {
	var sb strings.Builder

	if !quiet {
		sb.WriteString(fmt.Sprintf("Schema for Database: %s\n", schema.DatabaseName))
		sb.WriteString(strings.Repeat("-", 21+len(schema.DatabaseName)) + "\n")
	}

	if len(schema.Analyzers) == 0 {
		sb.WriteString("No analyzers found in database.\n")
		return sb.String()
	}

	headers := []string{"Analyzer Ref", "Analyzer Name", "Field Ref", "Field Name", "Type"}
	var rows [][]string

	for _, analyzer := range schema.Analyzers {
		if len(analyzer.Fields) == 0 {
			rows = append(rows, []string{analyzer.Ref, analyzer.Name, "", "", ""})
		} else {
			for _, field := range analyzer.Fields {
				rows = append(rows, []string{
					analyzer.Ref,
					analyzer.Name,
					field.Ref,
					field.Name,
					field.Type,
				})
			}
		}
	}

	sb.WriteString(output.FormatTable(headers, rows))
	return sb.String()
}

func formatSchemaCSV(schema *SchemaInfo) string {
	if len(schema.Analyzers) == 0 {
		return ""
	}

	headers := []string{"Analyzer Ref", "Analyzer Name", "Field Ref", "Field Name", "Type"}
	var rows [][]string

	for _, analyzer := range schema.Analyzers {
		if len(analyzer.Fields) == 0 {
			rows = append(rows, []string{analyzer.Ref, analyzer.Name, "", "", ""})
		} else {
			for _, field := range analyzer.Fields {
				rows = append(rows, []string{
					analyzer.Ref,
					analyzer.Name,
					field.Ref,
					field.Name,
					field.Type,
				})
			}
		}
	}

	// Note: output.FormatCSV prints directly. We might want to refactor it to return a string.
	// For now, we'll use a temporary capture or just call it.
	output.FormatCSV(headers, rows)
	return "" // CSV is printed directly by the utility
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
	sb.WriteString("  • View insights:   gsc query insights --db <name> --field <field>\n")
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

	sb.WriteString(fmt.Sprintf("GitSense Chat Coverage Report: %s\n", dbName))
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

	sb.WriteString(fmt.Sprintf("GitSense Chat Intelligence Report: %s\n", dbName))
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

// FormatManifestList formats a slice of DatabaseInfo into the specified format.
// This replaces the generic map-based formatting in the output package.
func FormatManifestList(databases []DatabaseInfo, format string) string {
	if len(databases) == 0 {
		return "No manifest databases found."
	}

	switch strings.ToLower(format) {
	case "json":
		return formatManifestListJSON(databases)
	case "table":
		return formatManifestListTable(databases)
	case "human":
		return formatManifestListHuman(databases)
	default:
		return fmt.Sprintf("Unsupported format: %s", format)
	}
}

func formatManifestListJSON(databases []DatabaseInfo) string {
	bytes, err := json.MarshalIndent(databases, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

func formatManifestListTable(databases []DatabaseInfo) string {
	headers := []string{"DB", "Description", "Tags", "Files"}
	rows := make([][]string, len(databases))
	for i, db := range databases {
		tags := strings.Join(db.Tags, ", ")
		rows[i] = []string{
			db.DatabaseName,
			truncate(db.Description, 60), // Truncate description to 60 chars
			tags,
			fmt.Sprintf("%d", db.EntryCount),
		}
	}
	return output.FormatTable(headers, rows)
}

func formatManifestListHuman(databases []DatabaseInfo) string {
	var sb strings.Builder

	for _, db := range databases {
		sb.WriteString(fmt.Sprintf("%s\n", db.ManifestName))
		sb.WriteString(fmt.Sprintf("   Database: %s\n", db.DatabaseName))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", db.Description))
		
		if len(db.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("   Tag: %s\n", strings.Join(db.Tags, ", ")))
		} else {
			sb.WriteString("   Tag: (none)\n")
		}
		
		sb.WriteString(fmt.Sprintf("   Files: %d\n", db.EntryCount))
		sb.WriteString("\n")
	}

	return sb.String()
}
