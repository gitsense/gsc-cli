/**
 * Component: Lessons Delete Command
 * Block-UUID: 9483aa5c-39bc-4389-8263-ac1409d6c3be
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Added --target flag for scoped lesson deletion.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package lessons

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func deleteCmd() *cobra.Command {
	var (
		yes         bool
		targetValue string
	)
	cmd := &cobra.Command{
		Use:   "delete <lesson-id>",
		Short: "Delete a lesson from current repository knowledge",
		Long: `Delete a committed lesson by its lsn_ ID.

This permanently removes the lesson from records.jsonl and rebuilds the gsc-lessons Brain.
The change will be visible in git diff — use git to recover a deleted lesson if needed.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			record, err := lessonspkg.ResolveRecordFromTarget(args[0], target)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("lesson not found in %s store: %s", target, args[0])
			}

			id := record.ID
			fmt.Print(lessonspkg.RenderRecord(*record))
			if !yes {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return fmt.Errorf("deletion requires confirmation. Run with --yes to confirm, or run the command directly in your terminal")
				}
				if !confirm("Delete this lesson from "+string(target)+" scope?", false) {
					fmt.Println("Canceled. Lesson left unchanged.")
					return nil
				}
			}
			deleted, err := lessonspkg.DeleteRecordFromTarget(id, target)
			if err != nil {
				return err
			}
			if !deleted {
				return fmt.Errorf("lesson not found in %s store: %s", target, id)
			}
			if err := lessonspkg.RebuildAndImportForTarget(target); err != nil {
				return err
			}
			fmt.Printf("Deleted lesson from %s scope: %s\n", target, id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
