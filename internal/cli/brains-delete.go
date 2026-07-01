/**
 * Component: Brains Delete Command
 * Block-UUID: e3f4a5b6-c7d8-9012-efab-012345678901
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command for deleting a brain (SQLite database) by its database name.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/manifest"
)

// brainsDeleteCmd represents the brains delete command
var brainsDeleteCmd = &cobra.Command{
	Use:   "delete <database-name>",
	Short: "Delete a brain (SQLite database)",
	Long: `Delete a brain (SQLite database) from the project.

This command removes the physical SQLite database file and updates the
.gitsense/manifest.json registry to remove the entry.

Unlike 'gsc manifest delete', this command is explicit about deleting
the brain (database), not the manifest file.

Arguments:
  database-name    The database name (e.g., code-intent, gsc-lessons, gsc-rules)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		// Call the logic layer to delete the manifest
		if err := manifest.DeleteManifest(dbName); err != nil {
			return err
		}

		fmt.Printf("Brain '%s' deleted successfully.\n", dbName)
		return nil
	},
	SilenceUsage: true,
}

func init() {
	// Add brains delete as a subcommand of brains
	BrainsCmd.AddCommand(brainsDeleteCmd)
}
