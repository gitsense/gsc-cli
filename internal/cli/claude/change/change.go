/**
 * Component: Change CLI Root Command
 * Block-UUID: cff30d0e-4944-4e6e-931e-a1da2d322d26
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Parent command for Change CLI (start, stop subcommands). Change enables in-place code editing with git diff generation based on validated discovery results.
 * Language: Go
 * Created-at: 2026-04-15T04:07:15.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package change

import (
	"github.com/spf13/cobra"
)

// GetAllChangeCommands returns all change subcommands
func GetAllChangeCommands() []*cobra.Command {
	return []*cobra.Command{
		StartCmd(),
		StopCmd(),
	}
}

// ChangeCmd creates the root "change" command
func ChangeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "change",
		Short: "Apply code changes based on validated discovery results",
		Long: `Change is a code editing tool that applies changes to files based on validated discovery results.

Change runs in one phase:
1. Change: In-place code editing with git diff generation

The change turn requires a completed validation turn to provide the list of validated files to modify.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Register all subcommands
	for _, subCmd := range GetAllChangeCommands() {
		cmd.AddCommand(subCmd)
	}

	return cmd
}
