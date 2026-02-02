/*
 * Component: Manifest Flags
 * Block-UUID: ca47cfb1-749d-4bb5-82b7-11da642562e3
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines shared flags for manifest subcommands, such as database name and output format.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import "github.com/spf13/cobra"

const (
	// FlagDBName is the name of the flag for specifying the database name
	FlagDBName = "db"
	// FlagFormat is the name of the flag for specifying the output format
	FlagFormat = "format"
)

// AddManifestFlags adds common flags to a manifest command
func AddManifestFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(FlagDBName, "d", "", "Name of the database to use (default: inferred from context)")
	cmd.Flags().StringP(FlagFormat, "f", "table", "Output format (table, json, csv)")
}
