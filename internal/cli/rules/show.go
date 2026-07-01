/**
 * Component: Rules Show Command
 * Block-UUID: a3b4c5d6-e7f8-9012-0123-123456789012
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules show to display a single rule in detail.
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

func showCmd() *cobra.Command {
	var (
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:          "show <id>",
		Short:        "Show a rule in detail",
		Args:         cobra.ExactArgs(1),
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
			record, err := rulespkg.ResolveSourcedRecordFromRecords(args[0], records)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("rule not found in %s scope: %s", scope, args[0])
			}
			return renderSourcedRuleDetail(*record, format)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}
