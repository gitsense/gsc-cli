/**
 * Component: Lessons Review Command
 * Block-UUID: 7f3a2e1d-5c8b-4d9e-a6f0-1b2c3d4e5f6a
 * Parent-UUID: e5c59ef0-ee7c-4637-9086-bdd2fc2deaa8
 * Version: 1.3.0
 * Description: Added discard as alternative next step when lesson is incorrect.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0), MiMo-v2.5-pro (v1.3.0)
 */


package lessons

import (
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "review",
		Short:        "Validate and preview the current lesson draft",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := lessonspkg.DraftPath()
			if err != nil {
				return err
			}
			result := lessonspkg.ReadAndValidateDraft(path)
			fmt.Print(lessonspkg.RenderDraftReview(result, path))
			if !result.Valid() {
				return fmt.Errorf("lesson draft is invalid")
			}
			fmt.Println()
			fmt.Println("If this lesson is correct, run:")
			fmt.Println("  gsc lessons commit")
			fmt.Println()
			fmt.Println("If this lesson is incorrect, run:")
			fmt.Println("  gsc lessons discard")
			return nil
		},
	}
	return cmd
}
