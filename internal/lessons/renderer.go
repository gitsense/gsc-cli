/**
 * Component: Lessons Renderer
 * Block-UUID: 2a4efe5a-437f-44ab-b8ef-5152a1f55135
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Renders lesson drafts and committed records into human-readable review output.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"fmt"
	"strings"
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
	fmt.Fprintf(&sb, "Summary: %s\n", record.Summary)
	if record.Details != "" {
		fmt.Fprintf(&sb, "Details: %s\n", record.Details)
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
