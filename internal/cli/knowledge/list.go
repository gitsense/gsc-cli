/**
 * Component: Knowledge List Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000007
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added --sort and --asc flags, UPDATED column to table output.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0)
 */


package knowledge

import (
	"encoding/json"
	"fmt"
	"time"

	knowledgepkg "github.com/gitsense/gsc-cli/internal/knowledge"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var (
		types    []string
		limit    int
		truncate int
		format   string
		sort     string
		asc      bool
	)
	cmd := &cobra.Command{
		Use:   "list --topic <slug>",
		Short: "List knowledge items in a topic",
		Long: `List all lessons, notes, and rules in a specific topic.

Use --type to filter by entity type (lessons, notes, rules).
Use --limit to cap the number of results.
Use --sort to choose sort field (updated, importance, type). Default: updated.
Use --asc to sort ascending (default: descending).`,
		Example: `  # List all items in a topic
  gsc knowledge list --topic data-layer

  # List only lessons in a topic
  gsc knowledge list --topic data-layer --type lessons

  # List with limit
  gsc knowledge list --topic data-layer --limit 10 -o json

  # Sort by importance (high first)
  gsc knowledge list --topic data-layer --sort importance

  # Sort by type, ascending
  gsc knowledge list --topic data-layer --sort type --asc`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get topic from flag
			topic, _ := cmd.Flags().GetString("topic")
			if topic == "" {
				return fmt.Errorf("--topic is required")
			}

			opts := knowledgepkg.ListOptions{
				Topic: topic,
				Types: types,
				Limit: limit,
				Sort:  knowledgepkg.SortField(sort),
				Asc:   asc,
			}

			response, err := knowledgepkg.List(opts)
			if err != nil {
				return err
			}

			switch format {
			case "", "table":
				renderListTable(response, topic, truncate)
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
	cmd.Flags().String("topic", "", "Topic slug (required)")
	cmd.Flags().StringSliceVar(&types, "type", nil, "Filter by entity type (lessons, notes, rules)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results (0 = all)")
	cmd.Flags().IntVar(&truncate, "truncate", 50, "Truncate summary to N characters (0 = no truncation)")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().StringVar(&sort, "sort", "updated", "Sort by field: updated, importance, type")
	cmd.Flags().BoolVar(&asc, "asc", false, "Sort ascending (default: descending)")
	return cmd
}

func renderListTable(response *knowledgepkg.ListResponse, topic string, truncateLen int) {
	if len(response.Items) == 0 {
		fmt.Printf("No items found in topic %q.\n", topic)
		return
	}

	fmt.Printf("Topic: %s\n\n", topic)

	// Header
	fmt.Printf("%-8s %-20s %-12s %-10s %s\n", "TYPE", "ID", "UPDATED", "IMPORTANCE", "SUMMARY")
	fmt.Printf("%-8s %-20s %-12s %-10s %s\n", "--------", "--------------------", "------------", "----------", "--------------------")

	// Rows
	for _, item := range response.Items {
		id := item.ID
		if len(id) > 20 {
			id = id[:17] + "..."
		}
		summary := item.Summary
		if truncateLen > 0 && len(summary) > truncateLen {
			summary = summary[:truncateLen-3] + "..."
		}
		importance := item.Importance
		if importance == "" {
			importance = "-"
		}
		updated := formatTime(item.UpdatedAt)
		fmt.Printf("%-8s %-20s %-12s %-10s %s\n", item.Type, id, updated, importance, summary)
	}

	// Facets
	fmt.Printf("\nTotal: %d lessons, %d notes, %d rules\n",
		response.Facets["lessons"],
		response.Facets["notes"],
		response.Facets["rules"],
	)
}

// formatTime formats a time.Time as YYYY-MM-DD, or "-" if zero.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}
