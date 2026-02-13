/*
 * Component: Schema Command
 * Block-UUID: 951dd825-025d-4abd-a951-0012647bf30c
 * Parent-UUID: cfb006e1-eee4-48db-9208-fe8c3da8289a
 * Version: 1.5.0
 * Description: Refactored the schema command to use the centralized 'manifest.FormatSchema' function. Removed local table and CSV formatting logic to ensure consistency across all commands that display schema information.
 * Language: Go
 * Created-at: 2026-02-02T07:56:00.000Z
 * Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0)
 */


package manifest

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var schemaFormat string

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema <database-name>",
	Short: "Inspect the schema of a manifest database",
	Long: `Inspect the schema of a manifest database to see available analyzers 
and their associated metadata fields. This helps understand what data is available 
for querying.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		// Resolve database name to physical name
		resolvedDB, err := registry.ResolveDatabase(dbName)
		if err != nil {
			return fmt.Errorf("failed to resolve database '%s': %w", dbName, err)
		}

		logger.Debug("Retrieving schema for database", "db", resolvedDB)

		// Call the logic layer to get schema
		schema, err := manifest.GetSchema(cmd.Context(), resolvedDB)
		if err != nil {
			return err
		}

		// Get config for the formatter
		config, err := manifest.GetEffectiveConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Format and output the results using the centralized formatter
		outputStr := manifest.FormatSchema(schema, schemaFormat, false, config)
		if outputStr != "" {
			fmt.Print(outputStr)
		}

		return nil
	},
	SilenceUsage: true,
}

func init() {
	// Add flags
	schemaCmd.Flags().StringVarP(&schemaFormat, "format", "f", "table", "Output format (json, table, csv)")
}
