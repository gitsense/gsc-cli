/**
 * Component: Docs Locate Command
 * Block-UUID: c6c07e07-744c-4b77-8141-c07e9c0511fa
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs locate' that displays the guide for finding where GitSense Chat is installed and where data is stored.
 * Language: Go
 * Created-at: 2026-05-31T14:46:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewLocateCmd creates and returns the 'gsc docs locate' subcommand.
func NewLocateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "locate",
		Short: "Find where GitSense Chat is installed and where your data is stored",
		Long: `Displays the locate guide that covers finding the GSC_HOME environment variable,
directory structure, database location, log files, configuration files, shadow repositories,
and analysis data storage.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("locate")
		},
	}

	return cmd
}
