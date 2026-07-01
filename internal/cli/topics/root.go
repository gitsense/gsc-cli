/**
 * Component: Topics CLI Root Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000005
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Registers gsc topics subcommands for managing the shared topic registry.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topics",
		Short: "Manage the shared topic registry for knowledge discovery",
		Long: `Manage topics that organize repository knowledge across lessons, notes, and rules.

Topics are broad navigational domains (e.g., data-layer, cli-workflow, authentication).
Every lesson, note, and rule must reference exactly one primary topic.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}

	cmd.AddCommand(listCmd())
	cmd.AddCommand(showCmd())
	cmd.AddCommand(searchCmd())
	cmd.AddCommand(addCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(migrateCmd())

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
