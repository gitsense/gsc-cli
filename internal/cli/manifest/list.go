/*
 * Component: Manifest List Command
 * Block-UUID: 1b572cc8-85e4-41af-9014-4fce10b8daf2
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command for listing available manifest databases.
 * Language: Go
 * Created-at: 2026-02-02T05:35:00Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/output"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var listFormat string

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available manifest databases",
	Long: `List all available manifest databases in the current project.
This command reads the .gitsense/manifest.json registry and displays
information about each database, including its name, description, and tags.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Call the logic layer to get the list
		databases, err := manifest.ListDatabases(cmd.Context())
		if err != nil {
			logger.Error("Failed to list databases: %v", err)
			os.Exit(1)
		}

		// Format and output the results
		if len(databases) == 0 {
			fmt.Println("No manifest databases found.")
			return
		}

		switch listFormat {
		case "json":
			output.FormatJSON(databases)
		case "csv":
			output.FormatCSV(databases)
		case "table":
			output.FormatTable(databases)
		default:
			logger.Error("Unsupported format: %s", listFormat)
			os.Exit(1)
		}
	},
}

func init() {
	// Add this command to the parent manifest command
	// Note: This assumes 'Cmd' is exported from root.go in the same package
	// We will wire this up in the root.go file generation step.
	
	// Add flags
	listCmd.Flags().StringVarP(&listFormat, "format", "f", "table", "Output format (json, table, csv)")
}

// GetListCommand returns the list command for registration
func GetListCommand() *cobra.Command {
	return listCmd
}
