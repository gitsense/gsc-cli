/**
 * Component: Topics Show Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000007
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Shows detailed information about a specific topic.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"encoding/json"
	"fmt"

	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
	"github.com/spf13/cobra"
)

type TopicDetail struct {
	Slug        string `json:"slug"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func showCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "show <slug>",
		Short: "Show details for a specific topic",
		Long:  `Show detailed information about a registered topic.`,
		Args:  cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			registry, err := topicstopkg.LoadRegistry()
			if err != nil {
				return err
			}

			topic := registry.Get(slug)
			if topic == nil {
				return fmt.Errorf("topic %q not found", slug)
			}

			switch format {
			case "", "human":
				fmt.Printf("Topic: %s\n", topic.Slug)
				fmt.Printf("Description: %s\n", topic.Description)
				fmt.Printf("Created: %s\n", topic.CreatedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("Updated: %s\n", topic.UpdatedAt.Format("2006-01-02 15:04:05"))
			case "json":
				detail := TopicDetail{
					Slug:        topic.Slug,
					Description: topic.Description,
					CreatedAt:   topic.CreatedAt.Format("2006-01-02T15:04:05Z"),
					UpdatedAt:   topic.UpdatedAt.Format("2006-01-02T15:04:05Z"),
				}
				data, err := json.MarshalIndent(detail, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			default:
				return fmt.Errorf("unknown format %q (use human or json)", format)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")
	return cmd
}
