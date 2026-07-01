/**
 * Component: Rules List Command
 * Block-UUID: f2a3b4c5-d6e7-8901-f012-012345678901
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc rules list with tag/topic/importance/type filters and table/json output.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package rules

import (
	"encoding/json"
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var (
		tags       []string
		topic      string
		importance string
		ruleType   string
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rules as a filterable table",
		Long: `List rules from repo, personal, or both scopes.

JSON output is an array of sourced rule records. Each item includes "source"
and "rule" fields so existing list consumers still receive an array while
agents can distinguish repo and personal provenance.`,
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
			// Apply non-tag filters first
			records = rulespkg.FilterSourcedRecords(records, rulespkg.ListFilter{
				Topic:      topic,
				Importance: importance,
				Type:       ruleType,
			})
			// Apply OR tag filtering if tags are specified
			if len(tags) > 0 {
				records = filterSourcedRulesByAnyTag(records, tags)
			}
			if limit > 0 && len(records) > limit {
				records = records[:limit]
			}
			return renderSourcedRecordList(records, format)
		},
	}
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Only rules whose tags match this value (repeatable, OR semantics)")
	cmd.Flags().StringVar(&topic, "topic", "", "Only rules whose topic matches this value")
	cmd.Flags().StringVar(&importance, "importance", "", "Only rules with this importance (high, medium, low)")
	cmd.Flags().StringVar(&ruleType, "type", "", "Only rules of this type (instruction, tool-trigger)")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of rules to return (0 = all)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}

// filterRulesByAnyTag returns rules that have at least one of the specified tags (OR semantics).
// Uses MatchesTag for consistent slug-based normalization and substring matching.
// Each rule is returned at most once, preserving the original order.
func filterRulesByAnyTag(records []rulespkg.Rule, tags []string) []rulespkg.Rule {
	var out []rulespkg.Rule
	seen := make(map[string]bool)
	for _, r := range records {
		if seen[r.ID] {
			continue
		}
		for _, filterTag := range tags {
			for _, ruleTag := range r.Tags {
				if rulespkg.MatchesTag(ruleTag, filterTag) {
					out = append(out, r)
					seen[r.ID] = true
					break
				}
			}
			if seen[r.ID] {
				break
			}
		}
	}
	return out
}

func renderRecordList(records []rulespkg.Rule, format string) error {
	switch format {
	case "", "table":
		fmt.Print(rulespkg.RenderRulesTable(records))
		return nil
	case "json":
		if records == nil {
			records = []rulespkg.Rule{}
		}
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unknown format %q (use table or json)", format)
	}
}
