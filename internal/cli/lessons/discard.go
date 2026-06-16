/**
 * Component: Lessons Discard Command
 * Block-UUID: aa3b1bd3-ddd8-45a9-9f47-e0e5b76020c6
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Initial implementation of the discard command for deleting lesson drafts.
 * Language: Go
 * Created-at: 2026-06-15
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package lessons

import (
	"fmt"
	"os"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func discardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "discard",
		Short:        "Discard the current lesson draft",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := lessonspkg.DraftPath()
			if err != nil {
				return err
			}

			if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
				fmt.Println("No lesson draft found to discard.")
				return nil
			}

			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to discard lesson draft: %w", err)
			}

			fmt.Printf("Discarded lesson draft:\n  %s\n", path)
			return nil
		},
	}
	return cmd
}
