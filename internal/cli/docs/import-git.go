/**
 * Component: Docs Import Git Command
 * Block-UUID: 75adbede-8b50-4308-995a-c8788ce0dd9b
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs import-git' that displays the Git repository import and analysis management guide.
 * Language: Go
 * Created-at: 2026-05-31T14:35:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewImportGitCmd creates and returns the 'gsc docs import-git' subcommand.
func NewImportGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-git",
		Short: "How to import Git repositories and manage analysis",
		Long: `Displays the Git repository import guide that covers importing repositories,
managing shadow repositories, incremental updates, rebuilds, and analysis management
(dump, load, copy) for Git repositories.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("import-git")
		},
	}

	return cmd
}
