/**
 * Component: Query Command
 * Block-UUID: d95a7a03-9399-4e5a-8f81-d39bbb4ce616
 * Parent-UUID: 08207ae5-f591-4d34-a342-e48e6e5d6f10
 * Version: 2.6.0
 * Description: CLI command definition for 'gsc query'. Added --coverage and --scope-override flags to support Phase 3 Scout Layer features. Implemented handleCoverage to orchestrate coverage analysis and reporting. Added --insights and --report flags to support Phase 2 Scout Layer features, including metadata aggregation and ASCII reporting.
 * Language: Go
 * Created-at: 2026-02-02T19:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), Claude Haiku 4.5 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), Gemini 3 Flash (v1.0.5), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0), Gemini 3 Flash (v2.5.0), GLM-4.7 (v2.6.0)
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
	queryList          bool
	queryListDB        bool
	queryFormat        string
	queryQuiet         bool
	queryCoverage      bool
	queryScopeOverride string
	queryInsights      bool
	queryReport        bool
	queryInsightsLimit int
)

// queryCmd represents the query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Find files by metadata value",
	Long: `Find files in a focused database by matching a metadata field value.
Supports hierarchical discovery (--list), context profiles, and simple value matching.`,
	Example: `  # 1. Discover what databases are available
  gsc query --list-db

  # 2. Explore fields in the default database (or list all DBs)
  gsc query --list

  # 3. See what values exist for a field
  gsc query --field risk_level --list

  # 4. Set your workspace context (using profiles)
  gsc config context create security --db security --field risk_level
  gsc config use security

  # 5. Check your current context
  gsc query

  # 6. Query using defaults (efficient!)
  gsc query --value critical

  # 7. Run coverage analysis
  gsc query --coverage

  # 8. Get insights on metadata distribution
  gsc query --insights --field risk_level,parent_keywords`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Priority 1: Explicit Database List
		if queryListDB {
			config, err := manifest.GetEffectiveConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			return handleList(ctx, "", "", queryFormat, queryQuiet, config)
		}

		// Priority 2: Coverage Analysis
		if queryCoverage {
			return handleCoverage(ctx, queryDB, queryScopeOverride, queryFormat, queryQuiet)
		}

		// Priority 3: Insights & Report
		if queryInsights || queryReport {
			if queryInsights && queryReport {
				return fmt.Errorf("--insights and --report are mutually exclusive")
			}
			return handleInsights(ctx, queryDB, queryField, queryInsightsLimit, queryScopeOverride, queryFormat, queryQuiet, queryReport)
		}

		// Priority 4: Hierarchical Discovery
		if queryList {
			return handleHierarchicalList(ctx, queryDB, queryField, queryFormat, queryQuiet)
		}

		// Priority 5: Query Execution or Status View
		return handleQueryOrStatus(ctx, queryDB, queryField, queryValue, queryFormat, queryQuiet)
	},
	SilenceUsage: true, // Silence usage output on logic errors
}

func init() {
	// Add flags
	queryCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database name (or use default)")
	queryCmd.Flags().StringVarP(&queryField, "field", "f", "", "Field name (or use default)")
	queryCmd.Flags().StringVarP(&queryValue, "value", "v", "", "Value to match (comma-separated for OR)")
	queryCmd.Flags().BoolVarP(&queryList, "list", "l", false, "List fields or values (respects default DB)")
	queryCmd.Flags().BoolVar(&queryListDB, "list-db", false, "Explicitly list all available databases")
	queryCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format (json, table)")
	queryCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers, footers, and hints (clean output)")
	queryCmd.Flags().BoolVar(&queryCoverage, "coverage", false, "Enable coverage analysis mode")
	queryCmd.Flags().StringVar(&queryScopeOverride, "scope-override", "", "Temporary scope override (e.g., include=src/**;exclude=tests/**)")
	
	// Insights/Report Flags
	queryCmd.Flags().BoolVar(&queryInsights, "insights", false, "Enable insights mode (JSON output)")
	queryCmd.Flags().BoolVar(&queryReport, "report", false, "Enable report mode (ASCII dashboard)")
	queryCmd.Flags().IntVar(&queryInsightsLimit, "limit", 10, "Limit number of top values to return (1-1000)")
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
func handleHierarchicalList(ctx context.Context, dbName string, fieldName string, format string, quiet bool) error {
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

	return handleList(ctx, resolvedDB, resolvedField, format, quiet, config)
}

// handleList performs the actual discovery call.
func handleList(ctx context.Context, dbName string, fieldName string, format string, quiet bool, config *manifest.QueryConfig) error {
	// Resolve database name if provided (might be a display name)
	if dbName != "" {
		var err error
		dbName, err = registry.ResolveDatabase(dbName)
		if err != nil {
			return err
		}
	}

	logger.Debug("Listing items", "database", dbName, "field", fieldName)

	result, err := manifest.GetListResult(ctx, dbName, fieldName)
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

	// Pass config to formatter to enable workspace headers
	output := manifest.FormatQueryResults(results, resolvedFormat, quiet, config)
	fmt.Println(output)
	return nil
}

// RegisterQueryCommand registers the query command with the root command.
func RegisterQueryCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(queryCmd)
}
