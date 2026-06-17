/**
 * Component: Lessons Draft Lifecycle Command
 * Block-UUID: 9bedb9ba-3690-4763-ae2a-ce7d40da5f73
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Groups the draft lifecycle (new, validate, review, commit, discard) under "gsc lessons draft" and errors on unknown subcommands.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0), claude-opus-4-8 (v1.1.0)
 */


package lessons

import "github.com/spf13/cobra"

func draftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Author the working lesson draft",
		Long: `Author the working lesson draft.

A draft is the staging area for a new lesson. Fill it in, validate it,
review it for human confirmation, then commit it to repository knowledge.`,
		SilenceUsage: true,
		RunE:         helpOrUnknown,
	}

	cmd.AddCommand(newCmd())
	cmd.AddCommand(validateCmd())
	cmd.AddCommand(reviewCmd())
	cmd.AddCommand(commitCmd())
	cmd.AddCommand(discardCmd())
	return cmd
}
