/*
 * Component: Manifest Delete Command
 * Block-UUID: ee3cc466-fa61-4c80-9345-f459d9ef9e85
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command for deleting a manifest database by its file name.
 * Language: Go
 * Created-at: 2026-02-10T17:22:37.159Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/manifest"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete <database-name>",
	Short: "Delete a manifest database",
	Long: `Delete a manifest database from the project.
This command removes the physical SQLite database file and updates the
.gitsense/manifest.json registry to remove the entry.

Arguments:
  database-name    The physical filename of the database (without .db extension)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		// Call the logic layer to delete the manifest
		if err := manifest.DeleteManifest(dbName); err != nil {
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		fmt.Printf("Manifest '%s' deleted successfully.\n", dbName)
		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors
}

func init() {
	// No flags currently, but reserved for future use (e.g., --force)
}
