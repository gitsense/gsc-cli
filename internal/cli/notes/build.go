/**
 * Component: Notes Build Command
 * Block-UUID: c1d2e3f4-a5b6-7890-cdef-012345678901
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc notes build with --target flag for scoped writes.
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

func buildCmd() *cobra.Command {
	var targetValue string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Rebuild the gsc-notes manifest and Brain from committed records",
		Long: `Rebuild the gsc-notes manifest and Brain from note records JSONL.

By default, records are read from .gitsense/notes/records.jsonl.
The manifest is written to .gitsense/manifests/gsc-notes.json and imported as a Brain.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			records, err := notespkg.LoadRecordsFromTarget(target)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Printf("No committed notes found in %s scope. Nothing to build.\n", target)
				return nil
			}
			if err := notespkg.RebuildAndImportForTarget(target); err != nil {
				return fmt.Errorf("failed to import Brain: %w", err)
			}
			manifestPath, err := notespkg.ManifestPathForTarget(target)
			if err != nil {
				return err
			}
			if target == gitsensescope.TargetRepo {
				fmt.Printf("Built gsc-notes Brain from %d note(s) in %s scope.\n", len(records), target)
			} else {
				fmt.Printf("Built gsc-notes manifest from %d note(s) in %s scope.\n", len(records), target)
			}
			fmt.Printf("Manifest: %s\n", manifestPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
