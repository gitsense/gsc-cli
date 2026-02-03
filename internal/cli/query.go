/**
 * Component: Query Command
 * Block-UUID: 47ef4276-947b-4db7-9de0-fea6d97569dc
 * Parent-UUID: c5da899d-3bf3-4ce3-8f0d-eed7d6951fe8
 * Version: 1.0.5
 * Description: CLI command definition for 'gsc query'. Updated --list behavior to respect default_database and added --list-db for explicit database discovery.
 * Language: Go
 * Created-at: 2026-02-02T19:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), Claude Haiku 4.5 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), Gemini 3 Flash (v1.0.5)
 */


package cli

import (
	"context"
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
	queryListDB bool
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
	Example: `  # 1. Discover what databases are available
  gsc query --list-db

  # 2. Explore fields in the default database (if set) or list all DBs
  gsc query --list

  # 3. See what values exist for a field in the default database
  gsc query --field risk_level --list

  # 4. Set your workspace context
  gsc query --set-default db=payments
  gsc query --set-default field=risk_level

  # 5. Check your current context
  gsc query

  # 6. Query using defaults (efficient!)
  gsc query --value critical`,
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

		// Priority 3: Explicit Database List
		if queryListDB {
			return handleList(ctx, "", "", queryFormat)
		}

		// Priority 4: Hierarchical Discovery
		if queryList {
			return handleHierarchicalList(ctx, queryDB, queryField, queryFormat)
		}

		// Priority 5: Query Execution or Status View
		return handleQueryOrStatus(ctx, queryDB, queryField, queryValue, queryFormat)
	},
}

func init() {
	// Add flags
	queryCmd.Flags().StringVarP(&queryDB, "db", "d", "", "Database name (or use default)")
	queryCmd.Flags().StringVarP(&queryField, "field", "f", "", "Field name (or use default)")
	queryCmd.Flags().StringVarP(&queryValue, "value", "v", "", "Value to match (comma-separated for OR)")
	queryCmd.Flags().BoolVarP(&queryList, "list", "l", false, "List fields or values (respects default DB)")
	queryCmd.Flags().BoolVar(&queryListDB, "list-db", false, "Explicitly list all available databases")
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

	if key == "db" {
		if err := manifest.ValidateDBExists(value); err != nil {
			return fmt.Errorf("database '%s' does not exist: %w", value, err)
		}
	}

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

// handleHierarchicalList resolves the database from defaults if not provided.
func handleHierarchicalList(ctx context.Context, dbName string, fieldName string, format string) error {
	config, err := manifest.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	resolvedDB := dbName
	if resolvedDB == "" {
		resolvedDB = config.Query.DefaultDatabase
	}

	resolvedField := fieldName
	if resolvedField == "" {
		resolvedField = config.Query.DefaultField
	}

	return handleList(ctx, resolvedDB, resolvedField, format)
}

// handleList performs the actual discovery call.
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
	config, err := manifest.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if value == "" {
		status := manifest.FormatStatusView(config)
		fmt.Println(status)
		return nil
	}

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

	if resolvedDB == "" {
		return fmt.Errorf("database is required. Use --db flag or set default with --set-default db=<name>")
	}
	if resolvedField == "" {
		return fmt.Errorf("field is required. Use --field flag or set default with --set-default field=<name>")
	}

	logger.Info("Executing query", "database", resolvedDB, "field", resolvedField, "value", value)
	results, err := manifest.ExecuteSimpleQuery(ctx, resolvedDB, resolvedField, value)
	if err != nil {
		return err
	}

	output := manifest.FormatQueryResults(results, resolvedFormat)
	fmt.Println(output)
	return nil
}

// RegisterQueryCommand registers the query command with the root command.
func RegisterQueryCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(queryCmd)
}
