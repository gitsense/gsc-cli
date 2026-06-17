/**
 * Component: Lessons CLI Root Command
 * Block-UUID: cad50c17-069b-4398-a641-96e49b22cd76
 * Parent-UUID: c03980ae-2812-400e-b415-2c2e2db9d222
 * Version: 1.10.0
 * Description: Registered the add command, made the whole draft lifecycle deprecated at top level, and errored on unknown subcommands.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0), MiMo-v2.5-pro (v1.3.0), claude-opus-4-8 (v1.4.0), claude-opus-4-8 (v1.5.0), claude-opus-4-8 (v1.6.0), claude-opus-4-8 (v1.7.0), claude-opus-4-8 (v1.8.0), claude-opus-4-8 (v1.9.0), claude-opus-4-8 (v1.10.0)
 */


package lessons

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lessons",
		Short: "Capture confirmed repository lessons for future humans and agents",
		Long: `Capture curated repository knowledge as GitSense lessons.

Knowledge is everything we could remember. Lessons are the parts worth carrying forward.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}

	// Canonical create and update lifecycles, plus one-shot create.
	cmd.AddCommand(draftCmd())
	cmd.AddCommand(updateCmd())
	cmd.AddCommand(addCmd())

	// Discovery.
	cmd.AddCommand(listCmd())
	cmd.AddCommand(searchCmd())
	cmd.AddCommand(tagsCmd())
	cmd.AddCommand(overviewCmd())
	cmd.AddCommand(showCmd())

	// Maintenance.
	cmd.AddCommand(deleteCmd())
	cmd.AddCommand(buildCmd())

	// Deprecated top-level lifecycle aliases. They remain functional but warn,
	// pushing users to "gsc lessons draft ...". cobra hides them from help.
	for _, dep := range []struct {
		cmd   *cobra.Command
		usage string
	}{
		{newCmd(), "new"},
		{validateCmd(), "validate"},
		{reviewCmd(), "review"},
		{commitCmd(), "commit"},
		{discardCmd(), "discard"},
	} {
		dep.cmd.Deprecated = fmt.Sprintf("use \"gsc lessons draft %s\" instead.", dep.usage)
		cmd.AddCommand(dep.cmd)
	}

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
