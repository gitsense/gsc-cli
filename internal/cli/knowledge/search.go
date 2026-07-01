/**
 * Component: Knowledge Search Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000006
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc knowledge search for unified search across lessons, notes, and rules.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"

	knowledgepkg "github.com/gitsense/gsc-cli/internal/knowledge"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var (
		types    []string
		topic    string
		limit    int
		truncate int
		format   string
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search across lessons, notes, and rules",
		Long: `Search all knowledge types with a unified query.

Results are ranked by relevance:
  - Exact topic match (highest)
  - Exact tag match
  - Summary term match
  - Body term match (lowest)

Use --type to filter by entity type (lessons, notes, rules).
Use --topic to filter by a specific topic.
Use --limit to cap the number of results.`,
		Example: `  # Search all knowledge
  gsc knowledge search "manifest import performance"

  # Search only lessons
  gsc knowledge search "database migration" --type lessons

  # Search within a topic
  gsc knowledge search "streaming" --topic data-layer

  # Limit results
  gsc knowledge search "error handling" --limit 10 -o json`,
		Args: cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(args[0])
			if query == "" {
				return fmt.Errorf("search query is required")
			}

			opts := knowledgepkg.SearchOptions{
				Types: types,
				Topic: topic,
				Limit: limit,
			}

			response, err := knowledgepkg.Search(query, opts)
			if err != nil {
				return err
			}

			switch format {
			case "", "table":
				renderSearchTable(response, truncate)
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
	cmd.Flags().StringSliceVar(&types, "type", nil, "Filter by entity type (lessons, notes, rules)")
	cmd.Flags().StringVar(&topic, "topic", "", "Filter by topic")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results (0 = all)")
	cmd.Flags().IntVar(&truncate, "truncate", 50, "Truncate summary to N characters (0 = no truncation)")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	return cmd
}

func renderSearchTable(response *knowledgepkg.SearchResponse, truncateLen int) {
	if len(response.Items) == 0 {
		fmt.Println("No results found.")
		return
	}

	// Header
	fmt.Printf("%-8s %-20s %-20s %-10s %s\n", "TYPE", "ID", "TOPIC", "SCORE", "SUMMARY")
	fmt.Printf("%-8s %-20s %-20s %-10s %s\n", "--------", "--------------------", "--------------------", "----------", "--------------------")

	// Rows
	for _, item := range response.Items {
		id := item.ID
		if len(id) > 20 {
			id = id[:17] + "..."
		}
		topic := item.Topic
		if len(topic) > 20 {
			topic = topic[:17] + "..."
		}
		summary := item.Summary
		if truncateLen > 0 && len(summary) > truncateLen {
			summary = summary[:truncateLen-3] + "..."
		}
		fmt.Printf("%-8s %-20s %-20s %-10.2f %s\n", item.Type, id, topic, item.Score, summary)
	}

	// Facets
	fmt.Printf("\nResults: %d lessons, %d notes, %d rules\n",
		response.Facets["lessons"],
		response.Facets["notes"],
		response.Facets["rules"],
	)
}
