/**
 * Component: Query Command
 * Block-UUID: 08207ae5-f591-4d34-a342-e48e6e5d6f10
 * Parent-UUID: 2afd0a1d-b135-40e9-9a7a-9c391b7eb412
 * Version: 2.5.0
 * Description: CLI command definition for 'gsc query'. Added --coverage and --scope-override flags to support Phase 3 Scout Layer features. Implemented handleCoverage to orchestrate coverage analysis and reporting.
 * Language: Go
 * Created-at: 2026-02-02T19:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), Claude Haiku 4.5 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), Gemini 3 Flash (v1.0.5), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0), Gemini 3 Flash (v2.5.0)
 */


package cli

import (
	"context"
	"fmt"

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
  gsc query --coverage`,
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

		// Priority 3: Hierarchical Discovery
		if queryList {
			return handleHierarchicalList(ctx, queryDB, queryField, queryFormat, queryQuiet)
		}

		// Priority 4: Query Execution or Status View
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
