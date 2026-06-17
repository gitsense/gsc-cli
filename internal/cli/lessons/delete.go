/**
 * Component: Lessons Delete Command
 * Block-UUID: 9483aa5c-39bc-4389-8263-ac1409d6c3be
 * Parent-UUID: 911b4f3f-e3e0-42a2-9b48-d0a4edb8c8c9
 * Version: 1.2.0
 * Description: Accepts a full lesson ID or a unique short-ID prefix via ResolveRecord and deletes by the resolved ID.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), claude-sonnet-4-6 (v1.1.0), claude-opus-4-8 (v1.2.0)
 */


package lessons

import (
	"fmt"
	"os"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func deleteCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <lesson-id>",
		Short: "Delete a lesson from current repository knowledge",
		Long: `Delete a committed lesson by its lsn_ ID.

This permanently removes the lesson from records.jsonl and rebuilds the gsc-lessons Brain.
The change will be visible in git diff — use git to recover a deleted lesson if needed.

To remove all lessons and start fresh:
  rm .gitsense/lessons/records.jsonl
  gsc manifest delete gsc-lessons`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lessonspkg.ResolveRecord(args[0])
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("lesson not found: %s", args[0])
			}
			id := record.ID
			fmt.Print(lessonspkg.RenderRecord(*record))
			if !yes {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return fmt.Errorf("deletion requires confirmation. Run with --yes to confirm, or run the command directly in your terminal")
				}
				if !confirm("Delete this lesson from current repository knowledge?", false) {
					fmt.Println("Canceled. Lesson left unchanged.")
					return nil
				}
			}
			deleted, err := lessonspkg.DeleteRecord(id)
			if err != nil {
				return err
			}
			if !deleted {
				return fmt.Errorf("lesson not found: %s", id)
			}
			if err := lessonspkg.RebuildAndImport(); err != nil {
				return err
			}
			fmt.Printf("Deleted lesson: %s\n", id)
			fmt.Println("Rebuilt and imported Brain: gsc-lessons")
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
