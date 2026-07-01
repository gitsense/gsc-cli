/**
 * Component: Topics Add Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000009
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Registers a new topic in the shared topic registry.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"fmt"
	"time"

	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
	"github.com/spf13/cobra"
)

func addCmd() *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "add <slug>",
		Short: "Register a new topic",
		Long: `Register a new topic in the shared topic registry.

Topics must be lowercase, hyphenated slugs (e.g., data-layer, cli-workflow).
Descriptions are required and help agents understand when to use each topic.`,
		Args: cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := topicstopkg.Slugify(args[0])

			if description == "" {
				return fmt.Errorf("--description is required")
			}

			// Load existing topics for validation
			existing, err := topicstopkg.LoadRecords()
			if err != nil {
				return err
			}

			// Build existing slugs list
			var existingSlugs []string
			for _, t := range existing {
				existingSlugs = append(existingSlugs, t.Slug)
			}

			// Create topic
			now := time.Now().UTC()
			topic := topicstopkg.Topic{
				Slug:        slug,
				Description: description,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			// Validate
			errs := topicstopkg.ValidateTopic(topic, existingSlugs)
			if len(errs) > 0 {
				for _, err := range errs {
					fmt.Printf("  ERROR %s\n", err)
				}
				return fmt.Errorf("topic is invalid")
			}

			// Save
			if err := topicstopkg.AppendRecord(topic); err != nil {
				return fmt.Errorf("failed to save topic: %w", err)
			}

			fmt.Printf("Topic %q registered.\n", topic.Slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Topic description (required)")
	return cmd
}
