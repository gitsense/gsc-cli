/**
 * Component: Notes Search Command
 * Block-UUID: e7f8a9b0-c1d2-3456-efab-678901234567
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc notes search for full-text search across note fields.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package notes

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var (
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:          "search <query>",
		Short:        "Search notes by text",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			sourcedRecords, err := notespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			matched := notespkg.SearchSourcedRecords(sourcedRecords, args[0])
			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}
			if len(matched) == 0 {
				fmt.Println("No notes match.")
				return nil
			}
			switch format {
			case "json":
				return renderSourcedRecordList(matched, "json")
			default:
				return renderSourcedRecordList(matched, "table")
			}
		},
	}
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of notes to return (0 = all)")
	return cmd
}
