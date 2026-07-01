/**
 * Component: Rules Delete Command
 * Block-UUID: b4c5d6e7-f8a9-0123-1234-234567890123
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules delete to remove a rule from the canonical store.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func deleteCmd() *cobra.Command {
	var targetValue string

	cmd := &cobra.Command{
		Use:          "delete <id>",
		Short:        "Delete a rule",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			// Resolve in target scope
			records, err := rulespkg.LoadRecordsFromTarget(target)
			if err != nil {
				return err
			}
			record, err := rulespkg.ResolveRecordFromRecords(args[0], records)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("rule not found in %s store: %s", target, args[0])
			}

			deleted, err := rulespkg.DeleteRecordFromTarget(record.ID, target)
			if err != nil {
				return err
			}
			if !deleted {
				return fmt.Errorf("rule not found in %s store: %s", target, record.ID)
			}
			// Rebuild the Brain after deletion
			if err := rulespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}
			fmt.Printf("Rule deleted from %s scope: %s\n", target, record.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
