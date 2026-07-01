/**
 * Component: Knowledge Topics Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000009
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc knowledge topics for viewing topic statistics.
 * Language: Go
 * Created-at: 2026-06-24T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package knowledge

import (
	"encoding/json"
	"fmt"
	"time"

	knowledgepkg "github.com/gitsense/gsc-cli/internal/knowledge"
	"github.com/spf13/cobra"
)

func topicsCmd() *cobra.Command {
	var (
		sort   string
		asc    bool
		empty  bool
		format string
	)
	cmd := &cobra.Command{
		Use:   "topics",
		Short: "View topic statistics across all knowledge",
		Long: `Display aggregated statistics for all topics, showing the count of
lessons, notes, and rules in each topic.

Use --sort to choose sort field (count, name). Default: count.
Use --asc to sort ascending (default: descending for count, ascending for name).
Use --empty to include topics with no items.`,
		Example: `  # View all topics with items
  gsc knowledge topics

  # Sort alphabetically
  gsc knowledge topics --sort name

  # Include empty topics
  gsc knowledge topics --empty

  # JSON output
  gsc knowledge topics -o json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := knowledgepkg.TopicsOptions{
				Sort:  sort,
				Asc:   asc,
				Empty: empty,
			}

			response, err := knowledgepkg.Topics(opts)
			if err != nil {
				return err
			}

			switch format {
			case "", "table":
				renderTopicsTable(response)
			case "json":
				data, err := json.MarshalIndent(response, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			default:
				return fmt.Errorf("unknown format %q (use table or json)", format)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sort, "sort", "count", "Sort by field: count, name")
	cmd.Flags().BoolVar(&asc, "asc", false, "Sort ascending")
	cmd.Flags().BoolVar(&empty, "empty", false, "Include topics with no items")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	return cmd
}

func renderTopicsTable(response *knowledgepkg.TopicsResponse) {
	if len(response.Topics) == 0 {
		fmt.Println("No topics found.")
		return
	}

	// Header
	fmt.Printf("%-24s %-7s %-5s %-5s %-6s %s\n", "TOPIC", "LESSONS", "NOTES", "RULES", "TOTAL", "UPDATED")
	fmt.Printf("%-24s %-7s %-5s %-5s %-6s %s\n", "------------------------", "-------", "-----", "-----", "------", "----------")

	// Rows
	for _, t := range response.Topics {
		updated := formatTopicTime(t.LatestUpdate)
		fmt.Printf("%-24s %-7d %-5d %-5d %-6d %s\n",
			truncateTopic(t.Slug, 24),
			t.Lessons,
			t.Notes,
			t.Rules,
			t.Total,
			updated,
		)
	}

	// Summary
	totalLessons, totalNotes, totalRules := 0, 0, 0
	for _, t := range response.Topics {
		totalLessons += t.Lessons
		totalNotes += t.Notes
		totalRules += t.Rules
	}
	fmt.Printf("\nTotal: %d topics, %d lessons, %d notes, %d rules\n",
		len(response.Topics),
		totalLessons,
		totalNotes,
		totalRules,
	)
}

func truncateTopic(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func formatTopicTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}
