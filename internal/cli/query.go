/*
 * Component: Query Command
 * Block-UUID: 4940efee-d95e-40c3-a3e1-fc64e8d9dfc5
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for 'gsc query', supporting hierarchical discovery, stateful defaults, and simple value matching.
 * Language: Go
 * Created-at: 2026-02-02T19:00:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	queryDB     string
	queryField  string
	queryValue  string
	queryList   bool
	queryFormat string
	querySet    string
	queryClear  string
)

// queryCmd represents the query command
var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Find files by metadata value",
	Long: `Find files in a focused database by matching a metadata field value.
Supports hierarchical discovery (--list), stateful defaults (--set-default),
and simple value matching with OR logic (comma-separated values).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Priority 1: Set Default
		if querySet != "" {
			return handleSetDefault(querySet)
		}

		// Priority 2: Clear Default
		if queryClear != "" {
			return handleClearDefault(queryClear)
		}

		// Priority 3: List Discovery
		if queryList {
			return handleList(ctx, queryDB, queryField, queryFormat)
		}

		// Priority 4: Query Execution or Status View
		return handleQueryOrStatus(ctx, queryDB, queryField, queryValue, queryFormat)
	},
}

func init() {
	// Add flags
	queryCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database name (or use default)")
	queryCmd.Flags().StringVarP(&queryField, "field", "f", "", "Field name (or use default)")
	queryCmd.Flags().StringVarP(&queryValue, "value", "v", "", "Value to match (comma-separated for OR)")
	queryCmd.Flags().BoolVarP(&queryList, "list", "l", false, "List databases, fields, or values")
	queryCmd.Flags().StringVarP(&queryFormat, "format", "o", "table", "Output format (json, table)")
	queryCmd.Flags().StringVar(&querySet, "set-default", "", "Set a default value (e.g., db=auth)")
	queryCmd.Flags().StringVar(&queryClear, "clear-default", "", "Clear a default value (e.g., db)")
}

// handleSetDefault parses and sets a default configuration value.
func handleSetDefault(input string) error {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid format for --set-default. Expected key=value, got: %s", input)
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	logger.Info("Setting default", "key", key, "value", value)
	if err := manifest.SetDefault(key, value); err != nil {
		return err
	}

	logger.Success("Default set successfully")
	return nil
}

// handleClearDefault clears a default configuration value.
func handleClearDefault(key string) error {
	logger.Info("Clearing default", "key", key)
	if err := manifest.ClearDefault(key); err != nil {
		return err
	}

	logger.Success("Default cleared successfully")
	return nil
}

// handleList performs hierarchical discovery based on provided flags.
func handleList(ctx context.Context, dbName string, fieldName string, format string) error {
	logger.Info("Listing items", "database", dbName, "field", fieldName)

	result, err := manifest.GetListResult(ctx, dbName, fieldName)
	if err != nil {
		return err
	}

	output := manifest.FormatListResult(result, format)
	fmt.Println(output)
	return nil
}

// handleQueryOrStatus determines whether to show status or execute a query.
func handleQueryOrStatus(ctx context.Context, dbName string, fieldName string, value string, format string) error {
	// Load config to check defaults
	config, err := manifest.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// If no value is provided, show status view
	if value == "" {
		status := manifest.FormatStatusView(config)
		fmt.Println(status)
		return nil
	}

	// Resolve parameters (flags override defaults)
	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Query.DefaultDatabase
	}

	resolvedField := fieldName
	if resolvedField == "" {
		resolvedField = config.Query.DefaultField
	}

	resolvedFormat := format
	if resolvedFormat == "" {
		resolvedFormat = config.Query.DefaultFormat
	}

	// Validate required parameters
	if resolvedDB == "" {
		return fmt.Errorf("database is required. Use --db flag or set default with --set-default db=<name>")
	}
	if resolvedField == "" {
		return fmt.Errorf("field is required. Use --field flag or set default with --set-default field=<name>")
	}

	// Execute Query
	logger.Info("Executing query", "database", resolvedDB, "field", resolvedField, "value", value)
	results, err := manifest.ExecuteSimpleQuery(ctx, resolvedDB, resolvedField, value)
	if err != nil {
		return err
	}

	// Format Output
	output := manifest.FormatQueryResults(results, resolvedFormat)
	fmt.Println(output)
	return nil
}

// RegisterQueryCommand registers the query command with the root command.
func RegisterQueryCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(queryCmd)
}
