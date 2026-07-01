/**
 * Component: Topics Update Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000010
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Updates an existing topic's description.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"fmt"

	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "update <slug>",
		Short: "Update an existing topic's description",
		Long:  `Update the description of an existing topic.`,
		Args:  cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := topicstopkg.Slugify(args[0])

			if description == "" {
				return fmt.Errorf("--description is required")
			}

			// Load registry to check existence
			registry, err := topicstopkg.LoadRegistry()
			if err != nil {
				return err
			}

			if !registry.Exists(slug) {
				return fmt.Errorf("topic %q not found", slug)
			}

			// Update
			updated, err := topicstopkg.UpdateRecord(slug, description)
			if err != nil {
				return fmt.Errorf("failed to update topic: %w", err)
			}
			if !updated {
				return fmt.Errorf("topic %q not found", slug)
			}

			fmt.Printf("Topic %q updated.\n", slug)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "New topic description (required)")
	return cmd
}
