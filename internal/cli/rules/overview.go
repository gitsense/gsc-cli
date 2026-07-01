/**
 * Component: Rules Overview Command
 * Block-UUID: e7f8a9b0-c1d2-3456-4567-567890123456
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules overview to summarize all rules in a human-readable digest.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func overviewCmd() *cobra.Command {
	var scopeValue string
	cmd := &cobra.Command{
		Use:          "overview",
		Short:        "Summarize all rules in a human-readable digest",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}
			records, err := rulespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			renderScopedOverview(scope, records)
			return nil
		},
	}
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}
