/**
 * Component: Manifest Export Command
 * Block-UUID: 496caa85-dfe4-48db-8585-2a2f0021fe2a
 * Parent-UUID: d119be58-653b-43da-b1dc-acc6a78cf45e
 * Version: 1.2.0
 * Description: CLI command definition for exporting a manifest database to a human-readable format (Markdown or JSON). Removed unused getter function. Updated to resolve database names from user input to physical names.
 * Language: Go
 * Created-at: 2026-02-02T08:00:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.2.0)
 */


package manifest

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	exportFormat string
	exportOutput string
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export <database-name>",
	Short: "Export a manifest database to a file",
	Long: `Export a manifest database to a human-readable format (Markdown or JSON).
	This command reads the specified database and generates a comprehensive report
	containing manifest info, repositories, branches, analyzers, fields, and file data.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		// Resolve database name to physical name
		resolvedDB, err := registry.ResolveDatabase(dbName)
		if err != nil {
			return fmt.Errorf("failed to resolve database '%s': %w", dbName, err)
		}

		logger.Info(fmt.Sprintf("Exporting database '%s' to %s format...", resolvedDB, exportFormat))

		ctx := context.Background()
		output, err := manifest.ExportDatabase(ctx, resolvedDB, exportFormat)
		if err != nil {
			logger.Error(fmt.Sprintf("Export failed: %v", err))
			return err
		}

		// Write to file or stdout
		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, []byte(output), 0644); err != nil {
				logger.Error(fmt.Sprintf("Failed to write output file: %v", err))
				return err
			}
			logger.Success(fmt.Sprintf("Export saved to %s", exportOutput))
		} else {
			fmt.Println(output)
		}

		return nil
	},
}

func init() {
	// Add flags
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "markdown", "Output format (markdown, json)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (default: stdout)")
}
