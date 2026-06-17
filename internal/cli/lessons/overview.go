/**
 * Component: Lessons Overview Command
 * Block-UUID: 3a7f1e08-9d4c-4b62-8e15-6c0a2f9b7d34
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons overview, a human-readable digest of all committed lessons, optionally clustered by tag.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0)
 */


package lessons

import (
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func overviewCmd() *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:     "overview",
		Aliases: []string{"summary"},
		Short:   "Summarize all committed lessons in a human-readable digest",
		Long: `Print a human-readable digest of all committed lessons: counts, importance,
the tag vocabulary, and each lesson's title.

Use --by tag to cluster lessons under the tags that connect them.
For machine-readable output use "gsc lessons list -o json".`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := lessonspkg.LoadRecords()
			if err != nil {
				return err
			}
			switch by {
			case "":
				fmt.Print(lessonspkg.RenderOverview(records))
				return nil
			case "tag":
				fmt.Print(lessonspkg.RenderOverviewByTag(records))
				return nil
			default:
				return fmt.Errorf("unknown --by value %q (supported: tag)", by)
			}
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "Cluster lessons by a field (tag)")
	return cmd
}
