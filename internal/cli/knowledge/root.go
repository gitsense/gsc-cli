/**
 * Component: Knowledge CLI Root Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000005
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added knowledge topics subcommand.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0)
 */


package knowledge

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Unified search and discovery across lessons, notes, and rules",
		Long: `Search and browse repository knowledge across all entity types.

Use 'knowledge search' for general questions across all knowledge types.
Use 'knowledge list' to browse items in a specific topic.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}

	cmd.AddCommand(searchCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(topicsCmd())

	return cmd
}

// helpOrUnknown prints help for a bare parent command but errors (non-zero) on
// an unrecognized subcommand instead of silently printing help.
func helpOrUnknown(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
	}
	return cmd.Help()
}
