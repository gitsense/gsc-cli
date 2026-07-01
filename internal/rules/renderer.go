/**
 * Component: Rules Renderer
 * Block-UUID: b8c9d0e1-f2a3-4567-bcde-678901234567
 * Parent-UUID: N/A
 * Version: 3.0.0
 * Description: Renders rules as human-readable tables, JSON, and detailed views. Supports instruction and tool-trigger rule types.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0)
 */


package rules

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

// RenderRulesTable renders rules as an aligned table for "gsc rules list".
func RenderRulesTable(rules []Rule) string {
	if len(rules) == 0 {
		return "No rules match.\n"
	}
	// Check if we have mixed types
	hasToolTrigger := false
	for _, r := range rules {
		if r.IsExecutable() {
			hasToolTrigger = true
			break
		}
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	if hasToolTrigger {
		fmt.Fprintln(w, "ID\tTYPE\tIMP\tUPDATED\tTOPIC\tSUMMARY")
		fmt.Fprintln(w, "--\t----\t---\t-------\t-----\t-------")
	} else {
		fmt.Fprintln(w, "ID\tIMP\tUPDATED\tTOPIC\tSUMMARY")
		fmt.Fprintln(w, "--\t---\t-------\t-----\t-------")
	}
	for _, r := range rules {
		topic := r.Topic
		if topic == "" {
			topic = "-"
		}
		if hasToolTrigger {
			typeLabel := string(r.Type)
			if typeLabel == "" {
				typeLabel = "declarative"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				ShortID(r.ID),
				typeLabel,
				r.Importance,
				ruleDate(r),
				truncate(topic, 28),
				truncate(r.Summary, 56),
			)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				ShortID(r.ID),
				r.Importance,
				ruleDate(r),
				truncate(topic, 28),
				truncate(r.Summary, 64),
			)
		}
	}
	w.Flush()
	return buf.String()
}

// RenderMatchedRulesTable renders matched rules as a table for "gsc rules get".
func RenderMatchedRulesTable(rules []MatchedRule) string {
	if len(rules) == 0 {
		return "No rules match.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tIMP\tACTIONS\tMATCH\tINSTRUCTIONS")
	fmt.Fprintln(w, "--\t---\t-------\t-----\t------------")
	for _, mr := range rules {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			ShortID(mr.Rule.ID),
			mr.Rule.Importance,
			truncate(strings.Join(mr.Rule.Actions, ","), 16),
			truncate(mr.MatchReason, 20),
			truncate(strings.Join(mr.Rule.Instructions, "; "), 48),
		)
	}
	w.Flush()
	return buf.String()
}

// RenderRuleDetail renders a single rule in detail.
func RenderRuleDetail(r Rule) string {
	var sb strings.Builder
	typeLabel := string(r.Type)
	if typeLabel == "" {
		typeLabel = "declarative"
	}
	fmt.Fprintf(&sb, "%s [%s] (%s)\n", r.ID, r.Importance, typeLabel)
	fmt.Fprintf(&sb, "Summary: %s\n\n", r.Summary)
	if r.Details != "" {
		fmt.Fprintf(&sb, "Details:\n  %s\n\n", r.Details)
	}
	if r.Owner != "" {
		fmt.Fprintf(&sb, "Owner: %s\n", r.Owner)
	}
	if len(r.Contact) > 0 {
		fmt.Fprintf(&sb, "Contact: %s\n", strings.Join(r.Contact, ", "))
	}

	// Tool-trigger specific fields
	if r.IsExecutable() {
		fmt.Fprintf(&sb, "Enabled: %v\n", r.IsEnabled())
		if r.Priority != 0 {
			fmt.Fprintf(&sb, "Priority: %d\n", r.Priority)
		}
		if r.Trigger != nil {
			fmt.Fprintf(&sb, "Trigger:\n")
			fmt.Fprintf(&sb, "  Runtime: %s\n", r.Trigger.Runtime)
			entryPath := r.Trigger.Entry
			if triggersDir, err := TriggersDir(); err == nil {
				entryPath = filepath.Join(triggersDir, r.Trigger.Entry)
			}
			fmt.Fprintf(&sb, "  Entry: %s\n", entryPath)
			if r.Trigger.TimeoutMs > 0 {
				fmt.Fprintf(&sb, "  Timeout: %dms\n", r.Trigger.TimeoutMs)
			}
		}
		if r.InstrCfg != nil {
			fmt.Fprintf(&sb, "Instruction:\n")
			fmt.Fprintf(&sb, "  Mode: %s\n", r.InstrCfg.Mode)
			if r.InstrCfg.Text != "" {
				fmt.Fprintf(&sb, "  Text: %s\n", r.InstrCfg.Text)
			}
			if r.InstrCfg.Query != "" {
				fmt.Fprintf(&sb, "  Query: %s\n", r.InstrCfg.Query)
			}
		}
		if r.Frequency != nil {
			fmt.Fprintf(&sb, "Frequency: %s", r.Frequency.Mode)
			if r.Frequency.Key != "" {
				fmt.Fprintf(&sb, " (key: %s)", r.Frequency.Key)
			}
			sb.WriteString("\n")
		}
		// Show hashes
		ruleHash := r.ComputeRuleHash()
		fmt.Fprintf(&sb, "RuleHash: %s\n", ruleHash)
		triggerHash, err := r.ComputeTriggerHash()
		if err != nil {
			fmt.Fprintf(&sb, "TriggerHash: error: %v\n", err)
		} else {
			fmt.Fprintf(&sb, "TriggerHash: %s\n", triggerHash)
		}
		sb.WriteString("\n")
	}

	writeList(&sb, "Glob Patterns", r.GlobPatterns)
	writeList(&sb, "Exclude Globs", r.ExcludeGlobs)
	writeList(&sb, "Files", r.AppliesTo.Files)
	writeList(&sb, "Linked Files", r.AppliesTo.LinkedFiles)
	writeList(&sb, "Commands", r.AppliesTo.Commands)
	writeList(&sb, "Topics", r.AppliesTo.Topics)
	writeList(&sb, "Tags", r.Tags)
	writeList(&sb, "Actions", r.Actions)
	if len(r.Instructions) > 0 {
		fmt.Fprintf(&sb, "Instructions:\n")
		for _, inst := range r.Instructions {
			fmt.Fprintf(&sb, "  - %s\n", inst)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "Created: %s\n", r.CreatedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&sb, "Updated: %s\n", r.UpdatedAt.Format("2006-01-02T15:04:05Z"))
	return sb.String()
}

// RenderTagTable renders tags as a value/count table.
func RenderTagTable(tags []TagFacet) string {
	if len(tags) == 0 {
		return "No tags found.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TAG\tRULES")
	fmt.Fprintln(w, "---\t-----")
	for _, t := range tags {
		fmt.Fprintf(w, "%s\t%d\n", t.Tag, t.Count)
	}
	w.Flush()
	return buf.String()
}

// TagFacet is a tag with its count.
type TagFacet struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// CountTags counts the distinct tags across rules.
func CountTags(records []Rule) []TagFacet {
	index := map[string]*TagFacet{}
	var order []string
	for _, r := range records {
		for _, tag := range r.Tags {
			facet, ok := index[tag]
			if !ok {
				facet = &TagFacet{Tag: tag}
				index[tag] = facet
				order = append(order, tag)
			}
			facet.Count++
		}
	}
	facets := make([]TagFacet, 0, len(order))
	for _, tag := range order {
		facets = append(facets, *index[tag])
	}
	return facets
}

// ShortID returns a compact, human-scannable form of a rule ID.
func ShortID(id string) string {
	s := strings.TrimPrefix(id, "rule_")
	if len(s) > 13 {
		return s[:13]
	}
	return s
}

func ruleDate(r Rule) string {
	t := r.UpdatedAt
	if t.IsZero() {
		t = r.CreatedAt
	}
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func truncate(s string, max int) string {
	s = oneLine(s)
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func writeList(sb *strings.Builder, title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(sb, "%s:\n", title)
	for _, value := range values {
		fmt.Fprintf(sb, "  - %s\n", value)
	}
	sb.WriteString("\n")
}

// RenderOverview prints a human-readable digest of all rules.
func RenderOverview(records []Rule) string {
	if len(records) == 0 {
		return "No rules recorded.\n"
	}
	var sb strings.Builder
	sb.WriteString(overviewHeader(records))
	sb.WriteString("\n")
	for _, r := range records {
		fmt.Fprintf(&sb, "  %-8s %s  %s\n", importanceTag(r.Importance), truncate(r.Summary, 72), tagHashes(r.Tags))
	}
	return sb.String()
}

func overviewHeader(records []Rule) string {
	importance := map[string]int{}
	var latest time.Time
	for _, r := range records {
		importance[strings.ToLower(r.Importance)]++
		t := r.UpdatedAt
		if t.IsZero() {
			t = r.CreatedAt
		}
		if t.After(latest) {
			latest = t
		}
	}
	parts := []string{fmt.Sprintf("%d rules", len(records))}
	for _, level := range []string{"high", "medium", "low"} {
		if importance[level] > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", level, importance[level]))
		}
	}
	line := strings.Join(parts, " · ")
	if !latest.IsZero() {
		line += " · updated through " + latest.Format("2006-01-02")
	}
	return line + "\n"
}

func importanceTag(importance string) string {
	if importance == "" {
		importance = "?"
	}
	return fmt.Sprintf("[%s]", importance)
}

func tagHashes(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tags))
	for _, t := range tags {
		parts = append(parts, "#"+t)
	}
	return truncate(strings.Join(parts, " "), 48)
}
