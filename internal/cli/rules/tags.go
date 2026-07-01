/**
 * Component: Rules Tags Command
 * Block-UUID: d6e7f8a9-b0c1-2345-3456-456789012345
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules tags to list rule tags with counts.
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

func tagsCmd() *cobra.Command {
	var (
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List rule tags and how many rules use each",
		Long: `List rule tags from repo, personal, or both scopes.

JSON output remains a plain array of tag facets for compatibility. With
--scope all, tag counts are merged across repo and personal rules.`,
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
			return renderScopedTags(scope, records, format)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}
