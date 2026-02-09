/**
 * Component: Query Command
 * Block-UUID: 6a320e3d-ebb4-4d70-a04c-1a385198046c
 * Parent-UUID: aff82dd6-f244-4e35-95ae-29389ad9013e
 * Version: 3.1.0
 * Description: Updated the 'query list' subcommand to support the '--all' flag. This flag enables the "Intelligence Map" view, which provides a complete hierarchical listing of all databases and their fields, optimized for AI agents like 'scout' to perform initial reconnaissance in a single turn.
 * Language: Go
 * Created-at: 2026-02-05T19:30:15.160Z
 * Authors: GLM-4.7 (v1.0.0), ..., Gemini 3 Flash (v3.0.0), Gemini 3 Flash (v3.1.0)
 */


package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	queryDB            string
	queryField         string
	queryValue         string
	queryFormat        string
	queryQuiet         bool
	queryScopeOverride string
	queryInsightsLimit int
	queryListDB        bool
	queryListAll       bool
	queryReport        bool
)

// queryCmd represents the base query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Find files by metadata value",
	Long: `Find files in a focused database by matching a metadata field value.
If no value is provided, it displays the current workspace context.`,
	Example: `  # Query using defaults (efficient!)
  gsc query --value critical

  # Override defaults
  gsc query --db security --field risk_level --value high`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleQueryOrStatus(cmd.Context(), queryDB, queryField, queryValue, queryFormat, queryQuiet)
	},
	SilenceUsage: true,
}

// queryListCmd represents the discovery subcommand
var queryListCmd = &cobra.Command{
	Use:   "list [field]",
	Short: "Discover available databases, fields, or values",
	Long: `Hierarchical discovery of the intelligence hub.
1. No arguments: Lists fields in the default/active database.
2. With [field]: Lists unique values for that specific field.
3. With --all: Lists all databases and their fields (Intelligence Map).
4. With --dbs: Lists all available databases.`,
	Example: `  # List the full intelligence map (all DBs and fields)
  gsc query list --all

  # List all available databases
  gsc query list --dbs

  # List fields in the active database
  gsc query list

  # List values for a specific field
  gsc query list risk_level`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		
		// Explicit Database List
		if queryListDB {
			config, err := manifest.GetEffectiveConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			return handleList(ctx, "", "", queryFormat, queryQuiet, config, queryListAll)
		}

		fieldName := ""
		if len(args) > 0 {
			fieldName = args[0]
		}

		return handleHierarchicalList(ctx, queryDB, fieldName, queryFormat, queryQuiet, queryListAll)
	},
}

// queryInsightsCmd represents the metadata distribution analysis subcommand
var queryInsightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Analyze metadata distribution and completeness",
	Long: `Provides a high-level overview of how metadata is distributed across the codebase.
Useful for identifying common patterns or unanalyzed areas.`,
	Example: `  # Get insights for specific fields
  gsc query insights --field risk_level,topic

  # Generate a human-readable ASCII report
  gsc query insights --field risk_level --report`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleInsights(cmd.Context(), queryDB, queryField, queryInsightsLimit, queryScopeOverride, queryFormat, queryQuiet, queryReport)
	},
}

// queryCoverageCmd represents the analysis coverage subcommand
var queryCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Analyze analysis coverage and identify blind spots",
	Long: `Compares Git tracked files against the manifest database to identify 
files that have not yet been analyzed within the current focus scope.`,
	Example: `  # Check coverage for the active database
  gsc query coverage

  # Check coverage with a temporary scope override
  gsc query coverage --scope-override "include=src/**"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleCoverage(cmd.Context(), queryDB, queryScopeOverride, queryFormat, queryQuiet)
	},
}

func init() {
	// Top-level Query Flags
	queryCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database name (or use default)")
	queryCmd.Flags().StringVarP(&queryField, "field", "f", "", "Field name (or use default)")
	queryCmd.Flags().StringVarP(&queryValue, "value", "v", "", "Value to match (comma-separated for OR)")
	queryCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format (json, table)")
	queryCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers, footers, and hints")

	// List Subcommand Flags
	queryListCmd.Flags().BoolVar(&queryListDB, "dbs", false, "List all available databases")
	queryListCmd.Flags().BoolVar(&queryListAll, "all", false, "Show all databases and their fields (Intelligence Map)")
	queryListCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database to list fields from")
	queryListCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format")
	queryListCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers and hints")

	// Insights Subcommand Flags
	queryInsightsCmd.Flags().StringVarP(&queryField, "field", "f", "", "Field(s) to analyze (required, comma-separated)")
	queryInsightsCmd.Flags().BoolVar(&queryReport, "report", false, "Generate ASCII dashboard report")
	queryInsightsCmd.Flags().IntVar(&queryInsightsLimit, "limit", 10, "Limit top values (1-1000)")
	queryInsightsCmd.Flags().StringVar(&queryScopeOverride, "scope-override", "", "Temporary scope override")
	queryInsightsCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database override")
	queryInsightsCmd.Flags().StringVarP(&queryFormat, "format", "o", "", "Output format (json/table)")
	queryInsightsCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers")

	// Coverage Subcommand Flags
	queryCoverageCmd.Flags().StringVar(&queryScopeOverride, "scope-override", "", "Temporary scope override")
	queryCoverageCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database override")
	queryCoverageCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format")
	queryCoverageCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers")

	// Register Subcommands
	queryCmd.AddCommand(queryListCmd)
	queryCmd.AddCommand(queryInsightsCmd)
	queryCmd.AddCommand(queryCoverageCmd)
}

// handleCoverage orchestrates the coverage analysis process.
func handleCoverage(ctx context.Context, dbName string, scopeOverride string, format string, quiet bool) error {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	if resolvedDB == "" {
		return fmt.Errorf("database is required for coverage analysis. Use --db flag or set a profile with 'gsc config use <name>'")
	}

	// Resolve database name to physical name
	resolvedDB, err = registry.ResolveDatabase(resolvedDB)
	if err != nil {
		return err
	}

	repoRoot, err := git.FindGitRoot()
	if err != nil {
		return fmt.Errorf("failed to find git root: %w", err)
	}

	logger.Debug("Executing coverage analysis", "database", resolvedDB, "scope_override", scopeOverride)
	report, err := manifest.ExecuteCoverageAnalysis(ctx, resolvedDB, scopeOverride, repoRoot, config.ActiveProfile)
	if err != nil {
		return err
	}

	// Pass config to formatter to enable workspace headers
	output := manifest.FormatCoverageReport(report, format, quiet, config)
	fmt.Println(output)
	return nil
}

// handleInsights orchestrates the insights and report generation process.
func handleInsights(ctx context.Context, dbName string, fieldsStr string, limit int, scopeOverride string, format string, quiet bool, isReport bool) error {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	if resolvedDB == "" {
		return fmt.Errorf("database is required for insights. Use --db flag or set a profile with 'gsc config use <name>'")
	}

	// Resolve database name to physical name
	resolvedDB, err = registry.ResolveDatabase(resolvedDB)
	if err != nil {
		return err
	}

	// Parse fields
	if fieldsStr == "" {
		return fmt.Errorf("--field is required for insights/report mode")
	}
	fields := strings.Split(fieldsStr, ",")

	// Validate limit
	if limit < 1 || limit > 1000 {
		return fmt.Errorf("--limit must be between 1 and 1000")
	}

	repoRoot, err := git.FindGitRoot()
	if err != nil {
		return fmt.Errorf("failed to find git root: %w", err)
	}

	logger.Debug("Executing insights analysis", "database", resolvedDB, "fields", fields, "limit", limit, "scope_override", scopeOverride)
	report, err := manifest.ExecuteInsightsAnalysis(ctx, resolvedDB, fields, limit, scopeOverride, repoRoot, config.ActiveProfile)
	if err != nil {
		return err
	}

	// Determine output format
	// --insights defaults to JSON, --report defaults to Table
	outputFormat := format
	if outputFormat == "" {
		if isReport {
			outputFormat = "table"
		} else {
			outputFormat = "json"
		}
	}

	var output string
	if isReport {
		output = manifest.FormatReport(report, outputFormat, quiet, config)
	} else {
		output = manifest.FormatInsightsReport(report, outputFormat, quiet, config)
	}

	fmt.Println(output)
	return nil
}

// handleHierarchicalList resolves the database from defaults if not provided.
func handleHierarchicalList(ctx context.Context, dbName string, fieldName string, format string, quiet bool, all bool) error {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	// Resolve database name to physical name
	if resolvedDB != "" {
		resolvedDB, err = registry.ResolveDatabase(resolvedDB)
		if err != nil {
			return err
		}
	}

	resolvedField := fieldName
	if resolvedField == "" {
		resolvedField = config.Query.DefaultField
	}

	return handleList(ctx, resolvedDB, resolvedField, format, quiet, config, all)
}

// handleList performs the actual discovery call.
func handleList(ctx context.Context, dbName string, fieldName string, format string, quiet bool, config *manifest.QueryConfig, all bool) error {
	// Resolve database name if provided (might be a display name)
	if dbName != "" {
		var err error
		dbName, err = registry.ResolveDatabase(dbName)
		if err != nil {
			return err
		}
	}

	logger.Debug("Listing items", "database", dbName, "field", fieldName, "all", all)

	result, err := manifest.GetListResult(ctx, dbName, fieldName, all)
	if err != nil {
		return err
	}

	output := manifest.FormatListResult(result, format, quiet, config)
	fmt.Println(output)
	return nil
}

// handleQueryOrStatus determines whether to show status or execute a query.
func handleQueryOrStatus(ctx context.Context, dbName string, fieldName string, value string, format string, quiet bool) error {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if value == "" {
		status := manifest.FormatStatusView(config, quiet)
		fmt.Println(status)
		return nil
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	// Resolve database name to physical name
	if resolvedDB != "" {
		resolvedDB, err = registry.ResolveDatabase(resolvedDB)
		if err != nil {
			return err
		}
	}

	resolvedField := fieldName
	if resolvedField == "" {
		resolvedField = config.Query.DefaultField
	}

	resolvedFormat := format
	if resolvedFormat == "" {
		resolvedFormat = config.Query.DefaultFormat
	}

	if resolvedDB == "" {
		return fmt.Errorf("database is required. Use --db flag or set a profile with 'gsc config use <name>'")
	}
	if resolvedField == "" {
		return fmt.Errorf("field is required. Use --field flag or set a profile with 'gsc config use <name>'")
	}

	logger.Debug("Executing query", "database", resolvedDB, "field", resolvedField, "value", value)
	results, err := manifest.ExecuteSimpleQuery(ctx, resolvedDB, resolvedField, value)
	if err != nil {
		return err
	}

	// Perform Coverage Analysis for the enriched response
	repoRoot, err := git.FindGitRoot()
	if err != nil {
		return fmt.Errorf("failed to find git root for coverage analysis: %w", err)
	}

	coverageReport, err := manifest.ExecuteCoverageAnalysis(ctx, resolvedDB, "", repoRoot, config.ActiveProfile)
	if err != nil {
		logger.Warning("Failed to execute coverage analysis", "error", err)
		// Create a dummy report to avoid nil pointer issues
		coverageReport = &manifest.CoverageReport{
			Percentages:    manifest.CoveragePercentages{FocusCoverage: 0},
			AnalysisStatus: "Unknown",
		}
	}

	// Construct the enriched response
	response := &manifest.QueryResponse{
		Query: manifest.SimpleQuery{
			Database:   resolvedDB,
			MatchField: resolvedField,
			MatchValue: value,
		},
		Results: results,
		Summary: manifest.QuerySummary{
			TotalResults:    len(results),
			CoveragePercent: coverageReport.Percentages.FocusCoverage,
			Confidence:      coverageReport.AnalysisStatus,
			Database:        resolvedDB,
		},
	}

	// Pass config to formatter to enable workspace headers
	output := manifest.FormatQueryResults(response, resolvedFormat, quiet, config)
	fmt.Println(output)
	return nil
}

// RegisterQueryCommand registers the query command with the root command.
func RegisterQueryCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(queryCmd)
}
