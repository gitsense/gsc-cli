/**
 * Component: Manifest List Command
 * Block-UUID: 58d910ed-3dcd-4e12-95fa-bfd451afdd8b
 * Parent-UUID: a527e3e4-c2e9-4685-8f70-b9ab0dd470c8
 * Version: 1.7.0
 * Description: CLI command for listing available manifest databases. Updated output map keys to explicitly separate 'database_name' (slug) and 'database_label' (human-readable) to align with the new schema terminology.
 * Language: Go
 * Created-at: 2026-02-13T04:40:14.796Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), Claude Haiku 4.5 (v1.2.0), Claude Haiku 4.5 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0)
 */


package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/manifest"
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

		// Format and output the results using the centralized manifest formatter
		fmt.Print(manifest.FormatManifestList(databases, listFormat))
		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors (e.g., registry not found)
}

func init() {
	// Add flags
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "human", "Output format (json, table, csv, human)")
}
