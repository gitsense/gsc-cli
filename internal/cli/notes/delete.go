/**
 * Component: Notes Delete Command
 * Block-UUID: b4c5d6e7-f8a9-0123-abcd-345678901234
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc notes delete with --target flag for scoped writes.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package notes

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	"github.com/spf13/cobra"
)

func deleteCmd() *cobra.Command {
	var targetValue string

	cmd := &cobra.Command{
		Use:          "delete <id>",
		Short:        "Delete a note",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			record, err := notespkg.ResolveRecordFromTarget(args[0], target)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("note not found in %s store: %s", target, args[0])
			}

			deleted, err := notespkg.DeleteRecordFromTarget(record.ID, target)
			if err != nil {
				return err
			}
			if !deleted {
				return fmt.Errorf("note not found in %s store: %s", target, record.ID)
			}
			// Rebuild the Brain after deletion
			if err := notespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}
			fmt.Printf("Note deleted from %s scope: %s\n", target, record.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
