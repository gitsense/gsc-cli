/**
 * Component: Lessons Search Command
 * Block-UUID: 7e1a4c93-2d6b-4f08-bb5a-0c9e3f1d62a4
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Added --scope flag for scoped lesson search.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package lessons

import (
	"strings"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var (
		fields     []string
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search committed lessons by text",
		Long: `Search committed lessons with a case-insensitive substring match.

By default the summary, details, tags, topics, and keywords are searched.
Use --fields to narrow the search to specific fields.`,
		Example: `  # Search all default fields (summary, details, tags, topics, keywords)
  gsc lessons search manifest

  # Multi-word query (joined as one phrase)
  gsc lessons search merge conflict

  # Restrict the search to specific fields
  gsc lessons search brains --fields tags,topics

  # JSON output for agents, capped to the first 5 matches
  gsc lessons search jsonl --limit 5 -o json

  # Search only personal lessons
  gsc lessons search "api" --scope personal`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			if err := lessonspkg.ValidateSearchFields(fields); err != nil {
				return err
			}
			query := strings.Join(args, " ")
			sourcedRecords, err := lessonspkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			sourcedRecords = lessonspkg.SearchSourcedRecords(sourcedRecords, query, fields)
			sourcedRecords = limitSourcedLessons(sourcedRecords, limit)
			return renderSourcedRecordList(sourcedRecords, format)
		},
	}
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "Fields to search (summary,details,tags,topics,keywords); default all")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of lessons to return (0 = all)")
	return cmd
}
