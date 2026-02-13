/*
 * Component: Manifest Import Command
 * Block-UUID: 16b34bb8-2011-417b-ae15-9663b216f128
 * Parent-UUID: 17f6fbf9-b729-4268-b4da-683cd9c815e3
 * Version: 1.3.0
 * Description: CLI command definition for importing a manifest JSON file. Added --force flag to allow overwriting existing databases and --no-backup flag to skip backup creation. Updated to support professional CLI output: removed redundant logger.Error calls in RunE and set SilenceUsage to true to prevent usage spam on logic errors.
 * Language: Go
 * Created-at: 2026-02-02T05:35:00Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var importCmd = &cobra.Command{
	Use:   "import <file.json>",
	Short: "Import a manifest JSON file into a SQLite database",
	Long: `Import a manifest JSON file downloaded from the Insights Builder or Analysis tool into a local SQLite database.
This command creates a new database file in the .gitsense directory and populates it with the metadata from the JSON file.
The database name is determined by the following priority:
1. The --name flag (if provided)
2. The 'database_name' field inside the JSON manifest
3. The filename of the JSON file (fallback)

By default, this command will fail if a database with the same name already exists.
Use the --force flag to overwrite an existing database. When using --force, a backup of the
existing database is automatically created in .gitsense/backups/ unless --no-backup is specified.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonPath := args[0]

		// Parse flags
		dbName, _ := cmd.Flags().GetString("name")
		force, _ := cmd.Flags().GetBool("force")
		noBackup, _ := cmd.Flags().GetBool("no-backup")
		
		// Note: description and tags flags are parsed here but currently require 
		// updates to the importer logic to be fully applied as overrides.
		_, _ = cmd.Flags().GetString("description")
		_, _ = cmd.Flags().GetString("tags")

		logger.Info(fmt.Sprintf("Importing manifest from '%s'...", jsonPath))

		ctx := context.Background()
		if err := manifest.ImportManifest(ctx, jsonPath, dbName, force, noBackup); err != nil {
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		logger.Success("Import completed successfully.")
		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors (e.g., DB exists)
}

func init() {
	// Add flags
	importCmd.Flags().String("name", "", "Override the database name (defaults to manifest.database_name or filename)")
	importCmd.Flags().String("description", "", "Override the manifest description")
	importCmd.Flags().String("tags", "", "Override the manifest tags (comma-separated)")
	importCmd.Flags().Bool("force", false, "Overwrite existing database if it exists")
	importCmd.Flags().Bool("no-backup", false, "Skip creating a backup of the existing database (use with caution)")
}
