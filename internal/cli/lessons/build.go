/**
 * Component: Lessons Build Command
 * Block-UUID: 1ade570a-9f65-48e5-87ab-6262d9ff4a5e
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Added --target flag for scoped lesson manifest rebuild.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: claude-sonnet-4-6 (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package lessons

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func buildCmd() *cobra.Command {
	var (
		source      string
		targetValue string
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Rebuild the gsc-lessons manifest and Brain from committed records",
		Long: `Rebuild the gsc-lessons manifest and Brain from lesson records JSONL.

By default, records are read from .gitsense/lessons/records.jsonl.
Use --source to read records from a file:// URI, relative path, absolute path, or URL.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			var records []lessonspkg.Record
			if source != "" {
				records, err = lessonspkg.LoadRecordsFromSource(source)
			} else {
				records, err = lessonspkg.LoadRecordsFromTarget(target)
			}
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Printf("No committed lessons found in %s scope. Nothing to build.\n", target)
				return nil
			}
			var manifestPath string
			if source != "" {
				manifestPath, err = lessonspkg.ManifestPathForTarget(target)
				if err != nil {
					return err
				}
				if err := lessonspkg.RebuildAndImportRecordsForTarget(records, target); err != nil {
					if target == gitsensescope.TargetRepo {
						return fmt.Errorf("failed to import Brain: %w", err)
					}
					return fmt.Errorf("failed to rebuild manifest: %w", err)
				}
			} else {
				if err := lessonspkg.RebuildAndImportForTarget(target); err != nil {
					return fmt.Errorf("failed to import Brain: %w", err)
				}
				manifestPath, err = lessonspkg.ManifestPathForTarget(target)
				if err != nil {
					return err
				}
			}
			if target == gitsensescope.TargetRepo {
				fmt.Printf("Built gsc-lessons Brain from %d lesson(s) in %s scope.\n", len(records), target)
			} else {
				fmt.Printf("Built gsc-lessons manifest from %d lesson(s) in %s scope.\n", len(records), target)
			}
			fmt.Printf("Manifest: %s\n", manifestPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Read lesson records JSONL from a source URI or path instead of target scope")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
