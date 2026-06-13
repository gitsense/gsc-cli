/**
 * Component: Lessons List Command
 * Block-UUID: 42414a2a-9e83-4315-b595-a97370dc506a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons list to summarize committed lesson records from canonical storage.
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

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List committed lessons",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := lessonspkg.LoadRecords()
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Println("No lessons recorded.")
				return nil
			}
			for _, record := range records {
				fmt.Printf("%s [%s] %s\n", record.ID, record.Importance, record.Summary)
			}
			return nil
		},
	}
	return cmd
}
