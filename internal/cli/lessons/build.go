/**
 * Component: Lessons Build Command
 * Block-UUID: 1ade570a-9f65-48e5-87ab-6262d9ff4a5e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons build to regenerate the gsc-lessons manifest and import the Brain from committed records. Required because the manifest is gitignored; run at session start or after pulling new lessons from teammates.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: claude-sonnet-4-6 (v1.0.0)
 */

package lessons

import (
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	var source string
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Rebuild the gsc-lessons manifest and Brain from committed records",
		Long: `Rebuild the gsc-lessons manifest and Brain from lesson records JSONL.

By default, records are read from .gitsense/lessons/records.jsonl.
Use --source to read records from a file:// URI, relative path, absolute path, or URL.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				records []lessonspkg.Record
				err     error
			)
			if source != "" {
				records, err = lessonspkg.LoadRecordsFromSource(source)
			} else {
				records, err = lessonspkg.LoadRecords()
			}
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Println("No committed lessons found. Nothing to build.")
				return nil
			}
			if err := lessonspkg.RebuildAndImportFromRecords(records); err != nil {
				return err
			}
			if source != "" {
				fmt.Printf("Built gsc-lessons Brain from %d lesson(s) in source: %s\n", len(records), source)
			} else {
				fmt.Printf("Built gsc-lessons Brain from %d lesson(s).\n", len(records))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Read lesson records JSONL from a source URI or path instead of .gitsense/lessons/records.jsonl")
	return cmd
}
