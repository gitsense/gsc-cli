/*
 * Component: Manifest List Command
 * Block-UUID: a527e3e4-c2e9-4685-8f70-b9ab0dd470c8
 * Parent-UUID: d1622670-df1a-4dbe-b095-5029820987ef
 * Version: 1.6.0
 * Description: CLI command for listing available manifest databases. Updated output map keys to explicitly separate 'database_name' (slug) and 'database_label' (human-readable) to align with the new schema terminology.
 * Language: Go
 * Created-at: 2026-02-02T05:35:00Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), Claude Haiku 4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)
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
				"database_name":  db.DatabaseName,  // The physical slug/ID
				"description":    db.Description,
				"tags":           db.Tags,
				"entry_count":    db.EntryCount,
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
