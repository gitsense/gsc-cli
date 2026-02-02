/*
 * Component: Manifest Import Command
 * Block-UUID: 508dc480-a13d-4fca-bad1-aff81c21198e
 * Parent-UUID: 62258b44-af06-4fad-ad26-b0146861c663
 * Version: 1.1.0
 * Description: CLI command definition for importing a manifest JSON file into a SQLite database. Removed filename derivation logic to defer to the importer logic which checks the JSON content.
 * Language: Go
 * Created-at: 2026-02-02T05:35:00Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var importCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Import a manifest JSON file into a SQLite database",
	Long: `Import a manifest JSON file downloaded from the Insights Builder or Analysis tool into a local SQLite database.
This command creates a new database file in the .gitsense directory and populates it with the metadata from the JSON file.
The database name is determined by the following priority:
1. The --name flag (if provided)
2. The 'database_name' field inside the JSON manifest
3. The filename of the JSON file (fallback)`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonPath := args[0]

		// Parse flags
		dbName, _ := cmd.Flags().GetString("name")
		// Note: description and tags flags are parsed here but currently require 
		// updates to the importer logic to be fully applied as overrides.
		_, _ = cmd.Flags().GetString("description")
		_, _ = cmd.Flags().GetString("tags")

		logger.Info(fmt.Sprintf("Importing manifest from '%s'...", jsonPath))

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
	importCmd.Flags().String("name", "", "Override the database name (defaults to manifest.database_name or filename)")
	importCmd.Flags().String("description", "", "Override the manifest description")
	importCmd.Flags().String("tags", "", "Override the manifest tags (comma-separated)")
}
