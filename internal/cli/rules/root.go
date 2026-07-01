/**
 * Component: Rules CLI Root Command
 * Block-UUID: c9d0e1f2-a3b4-5678-cdef-789012345678
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Registers the rules command group with add, get, changelog, list, show, delete, search, tags, overview, build, and trigger subcommands.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0), MiMo-v2.5-pro (v2.0.0)
 */

package rules

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Queryable guardrails and conventions for coding agents",
		Long: `Manage rules that coding agents can query before modifying files.

Rules define instructions, conventions, and constraints that agents should follow
when working with specific files or patterns.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}

	// CRUD
	cmd.AddCommand(newCmd())
	cmd.AddCommand(templateCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(deleteCmd())
	cmd.AddCommand(showCmd())

	// Agent-facing
	cmd.AddCommand(getCmd())
	cmd.AddCommand(executeCmd())
	cmd.AddCommand(changelogCmd())

	// Discovery
	cmd.AddCommand(listCmd())
	cmd.AddCommand(searchCmd())
	cmd.AddCommand(tagsCmd())
	cmd.AddCommand(overviewCmd())
	cmd.AddCommand(treeCmd())

	// Build
	cmd.AddCommand(buildCmd())

	// Testing
	cmd.AddCommand(testCmd())

	// Executable rule management
	triggerCmd := &cobra.Command{
		Use:   "trigger",
		Short: "Manage executable rules",
		Long: `Manage executable rules that evaluate runtime context before tool calls.

Executable rules allow repository-owned code to inject knowledge
or block tool calls based on runtime context.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}
	triggerCmd.AddCommand(triggerNewCmd())
	triggerCmd.AddCommand(triggerTemplateCmd())
	triggerCmd.AddCommand(triggerValidateCmd())
	triggerCmd.AddCommand(triggerRunCmd())
	cmd.AddCommand(triggerCmd)

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
