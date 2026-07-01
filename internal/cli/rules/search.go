/**
 * Component: Rules Search Command
 * Block-UUID: c5d6e7f8-a9b0-1234-2345-345678901234
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules search for full-text search across rule fields.
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

func searchCmd() *cobra.Command {
	var (
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search rules by text",
		Long: `Search rules by text from repo, personal, or both scopes.

JSON output is an array of sourced rule records. Each item includes "source"
and "rule" fields so existing search consumers still receive an array while
agents can distinguish repo and personal provenance.`,
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
			matched := rulespkg.SearchSourcedRecords(records, args[0])
			if limit > 0 && len(matched) > limit {
				matched = matched[:limit]
			}
			if len(matched) == 0 {
				fmt.Printf("No rules match in %s scope.\n", scope)
				return nil
			}
			return renderSourcedRecordList(matched, format)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of rules to return (0 = all)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}
