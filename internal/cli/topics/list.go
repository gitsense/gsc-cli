/**
 * Component: Topics List Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000006
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Lists registered topics with counts of lessons, notes, and rules.
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

type TopicWithCounts struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Lessons     int    `json:"lessons"`
	Notes       int    `json:"notes"`
	Rules       int    `json:"rules"`
}

func listCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered topics",
		Long:  `List all registered topics with their descriptions.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			topics, err := topicstopkg.LoadRecords()
			if err != nil {
				return err
			}

			switch format {
			case "", "table":
				renderTopicsTable(topics)
			case "json":
				if topics == nil {
					topics = []topicstopkg.Topic{}
				}
				data, err := json.MarshalIndent(topics, "", "  ")
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
	return cmd
}

func renderTopicsTable(topics []topicstopkg.Topic) {
	if len(topics) == 0 {
		fmt.Println("No topics registered.")
		return
	}

	// Find max slug length for alignment
	maxSlug := 5
	for _, t := range topics {
		if len(t.Slug) > maxSlug {
			maxSlug = len(t.Slug)
		}
	}

	// Print header
	fmt.Printf("%-*s  %s\n", maxSlug, "SLUG", "DESCRIPTION")
	fmt.Printf("%s  %s\n", strings.Repeat("-", maxSlug), strings.Repeat("-", 40))

	// Print rows
	for _, t := range topics {
		desc := t.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Printf("%-*s  %s\n", maxSlug, t.Slug, desc)
	}
}
