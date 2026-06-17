/**
 * Component: Lessons Search Command
 * Block-UUID: 7e1a4c93-2d6b-4f08-bb5a-0c9e3f1d62a4
 * Parent-UUID: N/A
 * Version: 1.2.0
 * Description: Added usage examples and --fields validation to the search command.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0), claude-opus-4-8 (v1.1.0), claude-opus-4-8 (v1.2.0)
 */


package lessons

import (
	"strings"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var (
		fields []string
		format string
		limit  int
	)
	cmd := &cobra.Command{
		Use:          "search <query>",
		Short:        "Search committed lessons by text",
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
  gsc lessons search jsonl --limit 5 -o json`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := lessonspkg.ValidateSearchFields(fields); err != nil {
				return err
			}
			query := strings.Join(args, " ")
			records, err := lessonspkg.LoadRecords()
			if err != nil {
				return err
			}
			records = lessonspkg.SearchRecords(records, query, fields)
			records = lessonspkg.LimitRecords(records, limit)
			return renderRecordList(records, format)
		},
	}
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "Fields to search (summary,details,tags,topics,keywords); default all")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of lessons to return (0 = all)")
	return cmd
}
