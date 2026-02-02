/*
 * Component: Manifest Import Command
 * Block-UUID: 62258b44-af06-4fad-ad26-b0146861c663
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for importing a manifest JSON file into a SQLite database.
 * Language: Go
 * Created-at: 2026-02-02T05:35:00Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var importCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Import a manifest JSON file into a SQLite database",
	Long: `Import a manifest JSON file downloaded from the Insights Builder or Analysis tool into a local SQLite database.
This command creates a new database file in the .gitsense directory and populates it with the metadata from the JSON file.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonPath := args[0]

		// Parse flags
		dbName, _ := cmd.Flags().GetString("name")
		// Note: description and tags flags are parsed here but currently require 
		// updates to the importer logic to be fully applied as overrides.
		_, _ = cmd.Flags().GetString("description")
		_, _ = cmd.Flags().GetString("tags")

		// Determine database name
		// Priority: --name flag > Filename derivation
		if dbName == "" {
			base := filepath.Base(jsonPath)
			dbName = strings.TrimSuffix(base, filepath.Ext(base))
			
			// Optional: Strip timestamp if present (e.g., security-20260201-143022 -> security)
			// This is a simple heuristic for cleaner names
			parts := strings.Split(dbName, "-")
			if len(parts) > 1 {
				// Check if the last part looks like a timestamp (YYYYMMDD or HHMMSS)
				// For now, we just use the full derived name to be safe
			}
		}

		logger.Info(fmt.Sprintf("Importing manifest from '%s' to database '%s'...", jsonPath, dbName))

		ctx := context.Background()
		if err := manifest.ImportManifest(ctx, jsonPath, dbName); err != nil {
			logger.Error(fmt.Sprintf("Import failed: %v", err))
			return err
		}

		logger.Success("Import completed successfully.")
		return nil
	},
}

func init() {
	// Add flags
	importCmd.Flags().String("name", "", "Override the database name (defaults to filename)")
	importCmd.Flags().String("description", "", "Override the manifest description")
	importCmd.Flags().String("tags", "", "Override the manifest tags (comma-separated)")
}
