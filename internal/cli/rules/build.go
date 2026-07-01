/**
 * Component: Rules Build Command
 * Block-UUID: a9b0c1d2-e3f4-5678-abcd-678901234567
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules build to regenerate the gsc-rules manifest and import the Brain from committed records.
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

func buildCmd() *cobra.Command {
	var targetValue string

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Rebuild the gsc-rules manifest and Brain from committed records",
		Long: `Rebuild the gsc-rules manifest and Brain from rule records JSONL.

By default, records are read from .gitsense/rules/records.jsonl.
The manifest is written to .gitsense/manifests/gsc-rules.json and imported as a Brain.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			records, err := rulespkg.LoadRecordsFromTarget(target)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Printf("No committed rules found in %s scope. Nothing to build.\n", target)
				return nil
			}
			if err := rulespkg.RebuildAndImportForTarget(target); err != nil {
				return fmt.Errorf("failed to rebuild rules manifest: %w", err)
			}
			manifestPath, err := rulespkg.ManifestPathForTarget(target)
			if err != nil {
				return err
			}
			if target == gitsensescope.TargetPersonal {
				fmt.Printf("Built gsc-rules manifest from %d rule(s) in %s scope.\n", len(records), target)
				fmt.Println("Personal Brain import is not supported yet.")
			} else {
				fmt.Printf("Built gsc-rules Brain from %d rule(s) in %s scope.\n", len(records), target)
			}
			fmt.Printf("Manifest: %s\n", manifestPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
