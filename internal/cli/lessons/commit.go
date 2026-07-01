/**
 * Component: Lessons Commit Command
 * Block-UUID: a512a45d-c455-4204-ad3f-396471d98cd2
 * Parent-UUID: 8d956c31-95d4-4c96-aa0b-54d6d7d24ba7
 * Version: 1.2.0
 * Description: Removed --yes flag and confirmation prompt. Commit is intentional by nature and runs unconditionally after validation.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0)
 */

package lessons

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func commitCmd() *cobra.Command {
	var (
		confirmedBy string
		targetValue string
	)
	cmd := &cobra.Command{
		Use:          "commit",
		Short:        "Commit the reviewed lesson draft and update the gsc-lessons Brain",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}
			path, err := lessonspkg.DraftPath()
			if err != nil {
				return err
			}
			result := lessonspkg.ReadAndValidateDraft(path)
			if !result.Valid() {
				return fmt.Errorf("lesson draft is invalid")
			}
			record, _, err := lessonspkg.CommitDraftToTarget(confirmedBy, target)
			if err != nil {
				return err
			}
			recordsPath, _ := lessonspkg.RecordsPathForTarget(target)
			fmt.Printf("Committed lesson to %s scope: %s\n", target, record.ID)
			fmt.Println()
			fmt.Println("The lesson is now available to future agent sessions.")
			fmt.Println()
			fmt.Println("To preserve it for teammates and future clones, commit:")
			fmt.Printf("  %s\n", recordsPath)
			fmt.Println()
			fmt.Println("To delete this lesson:")
			fmt.Printf("  gsc lessons delete %s --target %s\n", record.ID, target)
			return nil
		},
	}
	cmd.Flags().StringVar(&confirmedBy, "confirmed-by", "human", "Confirmation source recorded on the lesson")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
