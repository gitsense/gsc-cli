/**
 * Component: Query Command
 * Block-UUID: 2c1c2989-d17d-471e-8940-a7b9fd498d28
 * Parent-UUID: 47298171-4aaf-4826-93d5-1569a7cd6a20
 * Version: 3.10.0
 * Description: Added the 'DatabasesCmd' as a root-level convenience command. It supports listing all databases, inspecting a specific database schema via positional argument, or dumping all schemas using the --schema flag.
 * Language: Go
 * Created-at: 2026-02-13T06:38:29.128Z
 * Authors: GLM-4.7 (v1.0.0), ..., Gemini 3 Flash (v3.9.0), Gemini 3 Flash (v3.10.0)
 */


package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/bridge"
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
	queryFields        []string
	queryFieldSingular []string
	brainsSchema       bool
)

// queryCmd represents the base query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Find files by metadata value",
	Long: `Find files in a database by matching a metadata field value.
Supports exact matching and glob-style pattern matching (e.g., *connection*).
If no value is provided, it displays the current workspace context.`,
	Example: `  # Query using defaults (efficient!)
  gsc query --value critical

  # Use wildcards for pattern matching
  gsc query --field intent_triggers --value "*connection*"

  # Override defaults
  gsc query --db security --field risk_level --value high`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		outputStr, resolvedDB, err := handleQueryOrStatus(cmd.Context(), queryDB, queryField, queryValue, queryFormat, queryQuiet)
		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, queryFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
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
		startTime := time.Now()

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		ctx := cmd.Context()
		var outputStr, resolvedDB string
		var err error

		// Explicit Database List
		if queryListDB {
			config, err := manifest.GetEffectiveConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			outputStr, resolvedDB, err = handleList(ctx, "", "", queryFormat, queryQuiet, config, queryListAll)
		} else {
			fieldName := ""
			if len(args) > 0 {
				fieldName = args[0]
			}
			outputStr, resolvedDB, err = handleHierarchicalList(ctx, queryDB, fieldName, queryFormat, queryQuiet, queryListAll)
		}

		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, queryFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
	},
}

// InsightsCmd represents the metadata distribution analysis command.
var InsightsCmd = &cobra.Command{
	Use:   "insights",
	Short: "Analyze metadata distribution and completeness",
	Long: `Provides a high-level overview of how metadata is distributed across the codebase.
Useful for identifying common patterns or unanalyzed areas.`,
	Example: `  # Get insights for specific fields
  gsc insights --db security --field risk_level,topic

  # Also available as a query subcommand
  gsc query insights --db security --field risk_level,topic`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		if cmd.Flags().Changed("field") {
			return fmt.Errorf("unknown flag: --field. Did you mean --fields?")
		}

		outputStr, resolvedDB, err := handleInsights(cmd.Context(), queryDB, queryFields, queryInsightsLimit, queryScopeOverride, queryFormat, queryQuiet, queryReport)
		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, queryFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
	},
}

// CoverageCmd represents the analysis coverage command.
var CoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Analyze analysis coverage and identify blind spots",
	Long: `Compares Git tracked files against the manifest database to identify 
files that have not yet been analyzed within the current focus scope.`,
	Example: `  # Check coverage for the active database
  gsc coverage

  # Check coverage for the security database
  gsc coverage --db security

  # Check coverage with a temporary scope override
  gsc coverage --db security --scope-override "include=src/**"

  # Also available as a query subcommand
  gsc query coverage`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		outputStr, resolvedDB, err := handleCoverage(cmd.Context(), queryDB, queryScopeOverride, queryFormat, queryQuiet)
		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, queryFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
	},
}

// FieldsCmd represents the command to list all databases and their fields.
var FieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List all available databases and their fields",
	Long: `Displays the full intelligence map, showing all registered databases 
and the metadata fields available in each. This is equivalent to running 
'gsc query list --all'.`,
	Example: `  # Show the full intelligence map
  gsc fields

  # Also available as a query subcommand
  gsc query list --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		// Call handleHierarchicalList with all=true to get the full map
		outputStr, resolvedDB, err := handleHierarchicalList(cmd.Context(), "", "", queryFormat, queryQuiet, true)
		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, queryFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
	},
}

// BrainsCmd represents the command to list brains or inspect schemas.
var BrainsCmd = &cobra.Command{
	Use:   "brains [brain]",
	Short: "List available brains or inspect their schemas",
	Long: `Provides a high-level overview of the intelligence hub.
1. No arguments: Lists all registered manifest brains (databases).
2. With [brain]: Shows the schema for that specific brain.
3. With --schema: Shows the schema for every registered brain.`,
	Example: `  # List all brains
  gsc brains

  # Show schema for the 'arch' brain
  gsc brains arch

  # Show schemas for all brains
  gsc brains --schema`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		outputStr, resolvedDB, err := handleBrains(cmd.Context(), args, brainsSchema, queryFormat, queryQuiet)
		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, queryFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
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
	InsightsCmd.Flags().StringSliceVar(&queryFields, "fields", []string{}, "Field(s) to analyze (required, comma-separated or repeated)")
	InsightsCmd.Flags().StringSliceVar(&queryFieldSingular, "field", []string{}, "Did you mean --fields?")
	InsightsCmd.Flags().IntVar(&queryInsightsLimit, "limit", 10, "Limit top values (1-1000)")
	InsightsCmd.Flags().StringVar(&queryScopeOverride, "scope-override", "", "Temporary scope override")
	InsightsCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database override")
	InsightsCmd.Flags().StringVarP(&queryFormat, "format", "o", "", "Output format (json/table)")
	InsightsCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers")

	// Coverage Subcommand Flags
	CoverageCmd.Flags().StringVar(&queryScopeOverride, "scope-override", "", "Temporary scope override")
	CoverageCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database override")
	CoverageCmd.Flags().StringVarP(&queryFormat, "format(o)", "o", "table", "Output format")
	CoverageCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers")

	// Fields Subcommand Flags
	FieldsCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format")
	FieldsCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers and hints")

	// Brains Subcommand Flags
	BrainsCmd.Flags().BoolVar(&brainsSchema, "schema", false, "Show schema information")
	BrainsCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format")
	BrainsCmd.Flags().BoolVar(&queryQuiet, "quiet", false, "Suppress headers and hints")

	// Register Subcommands
	queryCmd.AddCommand(queryListCmd)
	queryCmd.AddCommand(InsightsCmd)
	queryCmd.AddCommand(CoverageCmd)
	queryCmd.AddCommand(FieldsCmd)
	queryCmd.AddCommand(BrainsCmd)
}

// handleBrains orchestrates the brain listing and schema inspection.
func handleBrains(ctx context.Context, args []string, showSchema bool, format string, quiet bool) (string, string, error) {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}

	// Case 1: Positional argument provided (Show schema for specific DB)
	if len(args) > 0 {
		resolvedDB, err := registry.ResolveDatabase(args[0])
		if err != nil {
			return "", "", err
		}
		schema, err := manifest.GetSchema(ctx, resolvedDB)
		if err != nil {
			return "", "", err
		}
		return manifest.FormatSchema(schema, format, quiet, config), resolvedDB, nil
	}

	// Case 2: --schema flag provided (Show schema for all DBs)
	if showSchema {
		databases, err := manifest.ListDatabases(ctx)
		if err != nil {
			return "", "", err
		}
		var outputs []string
		for _, dbInfo := range databases {
			schema, err := manifest.GetSchema(ctx, dbInfo.DatabaseName)
			if err != nil {
				logger.Warning("Failed to get schema for database", "db", dbInfo.DatabaseName, "error", err)
				continue
			}
			output := manifest.FormatSchema(schema, format, quiet, config)
			if quiet {
				outputs = append(outputs, output)
			} else {
				outputs = append(outputs, fmt.Sprintf("--- Brain: %s ---\n%s", dbInfo.DatabaseName, output))
			}
		}
		return strings.Join(outputs, "\n\n"), "", nil
	}

	// Case 3: Default (List all databases)
	return handleList(ctx, "", "", format, quiet, config, true)
}

// handleCoverage orchestrates the coverage analysis process.
func handleCoverage(ctx context.Context, dbName string, scopeOverride string, format string, quiet bool) (string, string, error) {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	if resolvedDB == "" {
		return "", "", fmt.Errorf("database is required for coverage analysis. Use --db flag.")
	}

	// Resolve database name to physical name
	resolvedDB, err = registry.ResolveDatabase(resolvedDB)
	if err != nil {
		return "", "", err
	}

	repoRoot, err := git.FindGitRoot()
	if err != nil {
		return "", "", fmt.Errorf("failed to find git root: %w", err)
	}

	logger.Debug("Executing coverage analysis", "database", resolvedDB, "scope_override", scopeOverride)
	report, err := manifest.ExecuteCoverageAnalysis(ctx, resolvedDB, scopeOverride, repoRoot, config.ActiveProfile)
	if err != nil {
		return "", "", err
	}

	output := manifest.FormatCoverageReport(report, format, quiet, config)
	return output, resolvedDB, nil
}

 // handleInsights orchestrates the insights and report generation process.
func handleInsights(ctx context.Context, dbName string, fields []string, limit int, scopeOverride string, format string, quiet bool, isReport bool) (string, string, error) {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	if resolvedDB == "" {
		return "", "", fmt.Errorf("database is required for insights. Use --db flag.")
	}

	// Resolve database name to physical name
	resolvedDB, err = registry.ResolveDatabase(resolvedDB)
	if err != nil {
		return "", "", err
	}

	if len(fields) == 0 {
		return "", "", fmt.Errorf("--fields is required for insights/report mode")
	}

	if limit < 1 || limit > 1000 {
		return "", "", fmt.Errorf("--limit must be between 1 and 1000")
	}

	repoRoot, err := git.FindGitRoot()
	if err != nil {
		return "", "", fmt.Errorf("failed to find git root: %w", err)
	}

	logger.Debug("Executing insights analysis", "database", resolvedDB, "fields", fields, "limit", limit, "scope_override", scopeOverride)
	report, err := manifest.ExecuteInsightsAnalysis(ctx, resolvedDB, fields, limit, scopeOverride, repoRoot, config.ActiveProfile)
	if err != nil {
		return "", "", err
	}

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

	return output, resolvedDB, nil
}

// handleHierarchicalList resolves the database from defaults if not provided.
func handleHierarchicalList(ctx context.Context, dbName string, fieldName string, format string, quiet bool, all bool) (string, string, error) {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	if resolvedDB != "" {
		resolvedDB, err = registry.ResolveDatabase(resolvedDB)
		if err != nil {
			return "", "", err
		}
	}

	resolvedField := fieldName
	if resolvedField == "" {
		resolvedField = config.Query.DefaultField
	}

	return handleList(ctx, resolvedDB, resolvedField, format, quiet, config, all)
}

// handleList performs the actual discovery call.
func handleList(ctx context.Context, dbName string, fieldName string, format string, quiet bool, config *manifest.QueryConfig, all bool) (string, string, error) {
	if dbName != "" {
		var err error
		dbName, err = registry.ResolveDatabase(dbName)
		if err != nil {
			return "", "", err
		}
	}

	logger.Debug("Listing items", "database", dbName, "field", fieldName, "all", all)

	result, err := manifest.GetListResult(ctx, dbName, fieldName, all)
	if err != nil {
		return "", "", err
	}

	output := manifest.FormatListResult(result, format, quiet, config)
	return output, dbName, nil
}

// handleQueryOrStatus determines whether to show status or execute a query.
func handleQueryOrStatus(ctx context.Context, dbName string, fieldName string, value string, format string, quiet bool) (string, string, error) {
	config, err := manifest.GetEffectiveConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to load config: %w", err)
	}

	if value == "" {
		status := manifest.FormatStatusView(config, quiet)
		return status, "", nil
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Global.DefaultDatabase
	}

	if resolvedDB != "" {
		resolvedDB, err = registry.ResolveDatabase(resolvedDB)
		if err != nil {
			return "", "", err
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
		return "", "", fmt.Errorf("database is required. Use --db flag.")
	}
	if resolvedField == "" {
		return "", "", fmt.Errorf("field is required. Use --field flag.")
	}

	logger.Debug("Executing query", "database", resolvedDB, "field", resolvedField, "value", value)
	results, err := manifest.ExecuteSimpleQuery(ctx, resolvedDB, resolvedField, value)
	if err != nil {
		return "", "", err
	}

	repoRoot, err := git.FindGitRoot()
	if err != nil {
		return "", "", fmt.Errorf("failed to find git root for coverage analysis: %w", err)
	}

	coverageReport, err := manifest.ExecuteCoverageAnalysis(ctx, resolvedDB, "", repoRoot, config.ActiveProfile)
	if err != nil {
		logger.Warning("Failed to execute coverage analysis", "error", err)
		coverageReport = &manifest.CoverageReport{
			Percentages:    manifest.CoveragePercentages{FocusCoverage: 0},
			AnalysisStatus: "Unknown",
		}
	}

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

	output := manifest.FormatQueryResults(response, resolvedFormat, quiet, config)
	return output, resolvedDB, nil
}

// RegisterQueryCommand registers the query command with the root command.
func RegisterQueryCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(queryCmd)
}
