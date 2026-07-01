/**
 * Component: Topics Search Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000008
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Searches topics by slug or description.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"encoding/json"
	"fmt"
	"strings"

	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
	"github.com/spf13/cobra"
)

func searchCmd() *cobra.Command {
	var format string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search topics by slug or description",
		Long:  `Search registered topics by matching against slug and description.`,
		Args:  cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.ToLower(strings.TrimSpace(args[0]))
			topics, err := topicstopkg.LoadRecords()
			if err != nil {
				return err
			}

			var matches []topicstopkg.Topic
			for _, t := range topics {
				if strings.Contains(strings.ToLower(t.Slug), query) ||
					strings.Contains(strings.ToLower(t.Description), query) {
					matches = append(matches, t)
				}
			}

			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}

			switch format {
			case "", "table":
				if len(matches) == 0 {
					fmt.Println("No topics found.")
					return nil
				}
				renderTopicsTable(matches)
			case "json":
				if matches == nil {
					matches = []topicstopkg.Topic{}
				}
				data, err := json.MarshalIndent(matches, "", "  ")
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
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results (0 = all)")
	return cmd
}
