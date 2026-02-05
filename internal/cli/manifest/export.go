/**
 * Component: Manifest Export Command
 * Block-UUID: dc5476b5-82db-4b99-98c9-02177e5fb1f0
 * Parent-UUID: 496caa85-dfe4-48db-8585-2a2f0021fe2a
 * Version: 1.3.0
 * Description: CLI command definition for exporting a manifest database to a human-readable format (Markdown or JSON). Removed unused getter function. Updated to resolve database names from user input to physical names. Updated to support professional CLI output: removed redundant logger.Error calls in RunE and set SilenceUsage to true to prevent usage spam on logic errors.
 * Language: Go
 * Created-at: 2026-02-02T08:00:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
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
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		// Write to file or stdout
		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, []byte(output), 0644); err != nil {
				// Error is returned to Cobra, which will print it cleanly via root.HandleExit
				return err
			}
			logger.Success(fmt.Sprintf("Export saved to %s", exportOutput))
		} else {
			fmt.Println(output)
		}

		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors (e.g., DB not found)
}

func init() {
	// Add flags
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "markdown", "Output format (markdown, json)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file path (default: stdout)")
}
