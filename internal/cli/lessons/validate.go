/**
 * Component: Lessons Validate Command
 * Block-UUID: ed66f3ee-61b0-4677-8078-dd3519a6a9e1
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons validate to perform a machine-oriented validity check before human review.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "validate",
		Short:        "Validate the current lesson draft without rendering a review",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := lessonspkg.DraftPath()
			if err != nil {
				return err
			}
			result := lessonspkg.ReadAndValidateDraft(path)
			if result.Valid() {
				fmt.Printf("OK lesson draft is valid: %s\n", path)
				return nil
			}
			fmt.Printf("Lesson draft is invalid: %s\n", path)
			for _, validationErr := range result.Errors {
				fmt.Printf("  ERROR %s\n", validationErr)
			}
			return fmt.Errorf("lesson draft is invalid")
		},
	}
	return cmd
}
