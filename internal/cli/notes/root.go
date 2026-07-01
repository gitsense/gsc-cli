/**
 * Component: Notes CLI Root Command
 * Block-UUID: e1f2a3b4-c5d6-7890-efab-901234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Registers the notes command group with add, update, delete, get, list, search, tags, overview, show, and build subcommands.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Searchable scratchpad notes for coding agents",
		Long: `Manage notes that coding agents can query for context and reference.

Notes are a scratchpad for research, context, and observations that help
agents understand the codebase. Unlike rules (guardrails) and lessons
(learned constraints), notes are for things you want to keep track of.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}

	// CRUD
	cmd.AddCommand(addCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(deleteCmd())
	cmd.AddCommand(showCmd())

	// Agent-facing
	cmd.AddCommand(getCmd())

	// Discovery
	cmd.AddCommand(listCmd())
	cmd.AddCommand(searchCmd())
	cmd.AddCommand(tagsCmd())
	cmd.AddCommand(overviewCmd())

	// Build
	cmd.AddCommand(buildCmd())

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
