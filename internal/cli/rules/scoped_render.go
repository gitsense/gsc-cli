package rules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
)

func renderSourcedRecordList(records []rulespkg.SourcedRule, format string) error {
	switch format {
	case "", "table":
		if len(records) == 0 {
			fmt.Print("No rules match.\n")
			return nil
		}
		repoRecords, personalRecords := splitSourcedRulesBySource(records)
		if len(repoRecords) > 0 {
			fmt.Println("Repo rules:")
			fmt.Print(rulespkg.RenderRulesTable(rulespkg.UnwrapSourcedRules(repoRecords)))
		}
		if len(personalRecords) > 0 {
			if len(repoRecords) > 0 {
				fmt.Println()
			}
			fmt.Println("Personal rules:")
			fmt.Print(rulespkg.RenderRulesTable(rulespkg.UnwrapSourcedRules(personalRecords)))
		}
		return nil
	case "json":
		if records == nil {
			records = []rulespkg.SourcedRule{}
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

func renderSourcedRuleDetail(record rulespkg.SourcedRule, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	case "", "human":
		fmt.Printf("Source: %s\n", record.Source)
		fmt.Print(rulespkg.RenderRuleDetail(record.Rule))
		return nil
	default:
		return fmt.Errorf("unknown format %q (use human or json)", format)
	}
}

func splitSourcedRulesBySource(records []rulespkg.SourcedRule) (repo, personal []rulespkg.SourcedRule) {
	for _, record := range records {
		if record.Source == gitsensescope.SourceRepo {
			repo = append(repo, record)
		} else {
			personal = append(personal, record)
		}
	}
	return repo, personal
}

func filterSourcedRulesByAnyTag(records []rulespkg.SourcedRule, tags []string) []rulespkg.SourcedRule {
	var out []rulespkg.SourcedRule
	seen := make(map[string]bool)
	for _, sr := range records {
		key := string(sr.Source) + ":" + sr.Rule.ID
		if seen[key] {
			continue
		}
		for _, filterTag := range tags {
			for _, ruleTag := range sr.Rule.Tags {
				if rulespkg.MatchesTag(ruleTag, filterTag) {
					out = append(out, sr)
					seen[key] = true
					break
				}
			}
			if seen[key] {
				break
			}
		}
	}
	return out
}

func scopeEmptyRulesMessage(scope gitsensescope.Scope) string {
	if scope == gitsensescope.ScopeAll {
		return "No rules found in repo or personal scope."
	}
	return fmt.Sprintf("No rules found in %s scope.", scope)
}

func scopeLabel(scope gitsensescope.Scope) string {
	if scope == gitsensescope.ScopeAll {
		return "all (repo + personal)"
	}
	return string(scope)
}

func unwrappedRulesBySource(records []rulespkg.SourcedRule) (repo, personal []rulespkg.Rule) {
	repoRecords, personalRecords := splitSourcedRulesBySource(records)
	return rulespkg.UnwrapSourcedRules(repoRecords), rulespkg.UnwrapSourcedRules(personalRecords)
}

func renderScopedOverview(scope gitsensescope.Scope, records []rulespkg.SourcedRule) {
	fmt.Printf("Scope: %s\n\n", scopeLabel(scope))
	if len(records) == 0 {
		fmt.Println(scopeEmptyRulesMessage(scope))
		return
	}
	repoRecords, personalRecords := unwrappedRulesBySource(records)
	if len(repoRecords) > 0 {
		fmt.Println("Repo rules:")
		fmt.Print(rulespkg.RenderOverview(repoRecords))
	}
	if len(personalRecords) > 0 {
		if len(repoRecords) > 0 {
			fmt.Println()
		}
		fmt.Println("Personal rules:")
		fmt.Print(rulespkg.RenderOverview(personalRecords))
	}
}

func renderScopedTags(scope gitsensescope.Scope, records []rulespkg.SourcedRule, format string) error {
	allRecords := rulespkg.UnwrapSourcedRules(records)
	tags := rulespkg.CountTags(allRecords)
	switch format {
	case "json":
		if tags == nil {
			tags = []rulespkg.TagFacet{}
		}
		data, err := json.MarshalIndent(tags, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	case "", "table":
		if len(tags) == 0 {
			fmt.Println("No tags found.")
			return nil
		}
		fmt.Printf("Scope: %s\n\n", scopeLabel(scope))
		if scope == gitsensescope.ScopeAll {
			repoRecords, personalRecords := unwrappedRulesBySource(records)
			var sections []string
			if len(repoRecords) > 0 {
				sections = append(sections, "Repo rule tags:\n"+rulespkg.RenderTagTable(rulespkg.CountTags(repoRecords)))
			}
			if len(personalRecords) > 0 {
				sections = append(sections, "Personal rule tags:\n"+rulespkg.RenderTagTable(rulespkg.CountTags(personalRecords)))
			}
			fmt.Print(strings.Join(sections, "\n"))
			return nil
		}
		fmt.Print(rulespkg.RenderTagTable(tags))
		return nil
	default:
		return fmt.Errorf("unknown format %q (use table or json)", format)
	}
}
