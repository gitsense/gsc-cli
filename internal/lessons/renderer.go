/**
 * Component: Lessons Renderer
 * Block-UUID: 2a4efe5a-437f-44ab-b8ef-5152a1f55135
 * Parent-UUID: N/A
 * Version: 1.5.0
 * Description: Added blank lines after Summary and Details in RenderRecord for readability.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v1.1.0), claude-opus-4-8 (v1.2.0), claude-opus-4-8 (v1.3.0), claude-opus-4-8 (v1.4.0), claude-opus-4-8 (v1.5.0)
 */


package lessons

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
)

func RenderDraftReview(result ValidationResult, path string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Lesson draft: %s\n\n", path)
	d := result.Draft
	if d.Summary != "" {
		fmt.Fprintf(&sb, "Summary:\n  %s\n\n", d.Summary)
	}
	if d.Details != "" {
		fmt.Fprintf(&sb, "Details:\n  %s\n\n", d.Details)
	}
	writeList(&sb, "Files", d.AppliesTo.Files)
	writeList(&sb, "Linked files", d.AppliesTo.LinkedFiles)
	writeList(&sb, "Commands", d.AppliesTo.Commands)
	writeList(&sb, "Topics", d.AppliesTo.Topics)
	writeList(&sb, "Tags", d.Tags)
	writeList(&sb, "Review checks", d.ReviewChecks)
	if d.Importance != "" {
		fmt.Fprintf(&sb, "Importance:\n  %s\n\n", d.Importance)
	}
	fmt.Fprintf(&sb, "AI provenance:\n  provider=%s model_id=%s agent=%s\n\n", d.AI.Provider, d.AI.ModelID, d.AI.Agent)

	if result.Valid() {
		sb.WriteString("Validation:\n  OK draft is valid\n")
		return sb.String()
	}
	sb.WriteString("Validation:\n")
	for _, err := range result.Errors {
		fmt.Fprintf(&sb, "  ERROR %s\n", err)
	}
	return sb.String()
}

func RenderRecord(record Record) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s [%s]\n", record.ID, record.Importance)
	fmt.Fprintf(&sb, "Summary: %s\n\n", record.Summary)
	if record.Details != "" {
		fmt.Fprintf(&sb, "Details: %s\n\n", record.Details)
	}
	writeList(&sb, "Files", record.AppliesTo.Files)
	writeList(&sb, "Linked files", record.AppliesTo.LinkedFiles)
	writeList(&sb, "Commands", record.AppliesTo.Commands)
	writeList(&sb, "Topics", record.AppliesTo.Topics)
	writeList(&sb, "Tags", record.Tags)
	writeList(&sb, "Review checks", record.ReviewChecks)
	fmt.Fprintf(&sb, "AI: provider=%s model_id=%s agent=%s\n", record.AI.Provider, record.AI.ModelID, record.AI.Agent)
	fmt.Fprintf(&sb, "Created: %s\n", record.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	return sb.String()
}

// RenderRecordsTable renders records as an aligned scan/filter table for
// "gsc lessons list" and "gsc lessons search".
func RenderRecordsTable(records []Record) string {
	if len(records) == 0 {
		return "No lessons match.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tIMP\tUPDATED\tTAGS\tSUMMARY")
	fmt.Fprintln(w, "--\t---\t-------\t----\t-------")
	for _, r := range records {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			ShortID(r.ID),
			r.Importance,
			recordDate(r),
			truncate(strings.Join(r.Tags, ","), 28),
			truncate(r.Summary, 64),
		)
	}
	w.Flush()
	return buf.String()
}

// RenderFacetTable renders facets (e.g. topics) as a value/count/short-IDs table.
// header is the column label for the facet value (e.g. "TOPIC").
func RenderFacetTable(facets []Facet, header string) string {
	if len(facets) == 0 {
		return "No " + strings.ToLower(header) + "s found.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "%s\tLESSONS\tIDS\n", header)
	fmt.Fprintf(w, "%s\t-------\t---\n", strings.Repeat("-", len(header)))
	for _, f := range facets {
		shortIDs := make([]string, 0, len(f.LessonIDs))
		for _, id := range f.LessonIDs {
			shortIDs = append(shortIDs, ShortID(id))
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", f.Value, f.Count, truncate(strings.Join(shortIDs, ","), 40))
	}
	w.Flush()
	return buf.String()
}

// RenderOverview prints a human-readable digest of all lessons: header stats,
// the tag vocabulary, and each lesson's title annotated with its tags.
func RenderOverview(records []Record) string {
	if len(records) == 0 {
		return "No lessons recorded.\n"
	}
	var sb strings.Builder
	sb.WriteString(overviewHeader(records))
	sb.WriteString("\n")
	sb.WriteString(tagSummaryLine(records))
	sb.WriteString("\n\n")
	for _, r := range records {
		fmt.Fprintf(&sb, "  %s %s  %s\n", importanceTag(r.Importance), truncate(r.Summary, 72), tagHashes(r.Tags))
	}
	return sb.String()
}

// RenderOverviewByTag prints the digest with lessons clustered under each tag
// that connects them. A lesson appears under every tag it carries.
func RenderOverviewByTag(records []Record) string {
	if len(records) == 0 {
		return "No lessons recorded.\n"
	}
	byID := make(map[string]Record, len(records))
	for _, r := range records {
		byID[r.ID] = r
	}
	var sb strings.Builder
	sb.WriteString(overviewHeader(records))
	sb.WriteString("\n")
	for _, f := range CountFacet(records, "tags") {
		fmt.Fprintf(&sb, "%s (%d)\n", f.Value, f.Count)
		for _, id := range f.LessonIDs {
			r := byID[id]
			fmt.Fprintf(&sb, "  %s %s\n", importanceTag(r.Importance), truncate(r.Summary, 72))
		}
		sb.WriteString("\n")
	}
	var untagged []Record
	for _, r := range records {
		if len(r.Tags) == 0 {
			untagged = append(untagged, r)
		}
	}
	if len(untagged) > 0 {
		sb.WriteString("(untagged)\n")
		for _, r := range untagged {
			fmt.Fprintf(&sb, "  %s %s\n", importanceTag(r.Importance), truncate(r.Summary, 72))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func overviewHeader(records []Record) string {
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
	parts := []string{fmt.Sprintf("%d lessons", len(records))}
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

func tagSummaryLine(records []Record) string {
	facets := CountFacet(records, "tags")
	if len(facets) == 0 {
		return "Tags: (none)"
	}
	const maxShown = 6
	var parts []string
	for i, f := range facets {
		if i >= maxShown {
			break
		}
		if f.Count > 1 {
			parts = append(parts, fmt.Sprintf("%s (%d)", f.Value, f.Count))
		} else {
			parts = append(parts, f.Value)
		}
	}
	line := "Tags: " + strings.Join(parts, " · ")
	if len(facets) > maxShown {
		line += fmt.Sprintf(" · +%d more", len(facets)-maxShown)
	}
	return line
}

func importanceTag(importance string) string {
	if importance == "" {
		importance = "?"
	}
	return fmt.Sprintf("%-8s", "["+importance+"]")
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

// ShortID returns a compact, human-scannable form of a lesson ID. The full ID
// (or this prefix) remains accepted by show/delete via ResolveRecord.
func ShortID(id string) string {
	s := strings.TrimPrefix(id, "lsn_")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func recordDate(r Record) string {
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

// RenderUpdateReview shows a staged replacement as an old → new comparison of
// the target record against the validated new content.
func RenderUpdateReview(original Record, result ValidationResult, path string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Staged update for %s\n", original.ID)
	fmt.Fprintf(&sb, "  stage: %s\n\n", path)

	d := result.Draft
	writeDiffLine(&sb, "Summary", original.Summary, d.Summary)
	writeDiffLine(&sb, "Details", original.Details, d.Details)
	writeDiffLine(&sb, "Importance", original.Importance, d.Importance)
	writeDiffList(&sb, "Tags", original.Tags, d.Tags)
	writeDiffList(&sb, "Files", original.AppliesTo.Files, d.AppliesTo.Files)
	writeDiffList(&sb, "Linked files", original.AppliesTo.LinkedFiles, d.AppliesTo.LinkedFiles)
	writeDiffList(&sb, "Commands", original.AppliesTo.Commands, d.AppliesTo.Commands)
	writeDiffList(&sb, "Topics", original.AppliesTo.Topics, d.AppliesTo.Topics)
	writeDiffList(&sb, "Review checks", original.ReviewChecks, d.ReviewChecks)

	if result.Valid() {
		sb.WriteString("\nValidation:\n  OK staged update is valid\n")
		return sb.String()
	}
	sb.WriteString("\nValidation:\n")
	for _, err := range result.Errors {
		fmt.Fprintf(&sb, "  ERROR %s\n", err)
	}
	return sb.String()
}

func writeDiffLine(sb *strings.Builder, label, oldV, newV string) {
	if oldV == newV {
		if newV == "" {
			return
		}
		fmt.Fprintf(sb, "%s (unchanged): %s\n", label, truncate(newV, 80))
		return
	}
	fmt.Fprintf(sb, "%s:\n  - old: %s\n  + new: %s\n", label, truncate(oldV, 80), truncate(newV, 80))
}

func writeDiffList(sb *strings.Builder, label string, oldV, newV []string) {
	oldJoined := strings.Join(oldV, ", ")
	newJoined := strings.Join(newV, ", ")
	if oldJoined == newJoined {
		if newJoined == "" {
			return
		}
		fmt.Fprintf(sb, "%s (unchanged): %s\n", label, newJoined)
		return
	}
	fmt.Fprintf(sb, "%s:\n  - old: %s\n  + new: %s\n", label, oldJoined, newJoined)
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
