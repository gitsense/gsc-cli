/*
 * Component: Manifest List Command
 * Block-UUID: d1622670-df1a-4dbe-b095-5029820987ef
 * Parent-UUID: 093c618c-3ddc-4bae-86c3-3771e6c49925
 * Version: 1.5.0
 * Description: CLI command for listing available manifest databases. Added context nil check for robustness. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: removed redundant logger.Error calls in RunE and set SilenceUsage to true to prevent usage spam on logic errors.
 * Language: Go
 * Created-at: 2026-02-02T05:35:00Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), Claude Haiku 4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/output"
)

var listFormat string

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available manifest databases",
	Long: `List all available manifest databases in the current project.
This command reads the .gitsense/manifest.json registry and displays
information about each database, including its name, description, and tags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get context with fallback to Background if nil
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// Call the logic layer to get the list
		databases, err := manifest.ListDatabases(ctx)
		if err != nil {
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		// Format and output the results
		if len(databases) == 0 {
			fmt.Println("No manifest databases found.")
			return nil
		}

		// Convert DatabaseInfo to generic interface slice for formatter
		var dbInterfaces []interface{}
		for _, db := range databases {
			dbMap := map[string]interface{}{
				"name":        db.Name,
				"description": db.Description,
				"tags":        db.Tags,
				"db_path":     db.DBPath,
				"entry_count": db.EntryCount,
			}
			dbInterfaces = append(dbInterfaces, dbMap)
		}

		// Format and output
		output.FormatDatabaseTable(dbInterfaces, listFormat)
		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors (e.g., registry not found)
}

func init() {
	// Add flags
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "Output format (json, table, csv)")
}
