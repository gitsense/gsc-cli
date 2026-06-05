/*
 * Component: GitIgnore Root Command
 * Block-UUID: e74fe4f3-5dc4-4f05-8e68-b985d35bd69c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Parent command for gitignore operations, managing the .gitsense/.gitignore file.
 * Language: Go
 * Created-at: 2026-06-04T13:04:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package gitignore

import (
	"github.com/spf13/cobra"
)

// Cmd represents the gitignore command group
var Cmd = &cobra.Command{
	Use:   "gitignore",
	Short: "Manage the .gitsense/.gitignore file",
	Long: `The gitignore command provides tools to manage the .gitsense/.gitignore file,
which ensures that state files, generated artifacts, and temporary files are not tracked by git.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, print help
		cmd.Help()
	},
}

func init() {
	// Register subcommands
	Cmd.AddCommand(NewUpdateCommand())
}
