/**
 * Component: Notes Renderer
 * Block-UUID: c9d0e1f2-a3b4-5678-cdef-789012345678
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Renders notes as human-readable tables, JSON, and detailed views.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Updated-at: 2026-06-24T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), terrchen (v1.1.0)
 * Changelog:
 *   v1.1.0 - Expand ShortID from 8 to 12 chars; replace TAGS column with TOPICS
 */


package notes

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"
)

// RenderNotesTable renders notes as an aligned table for "gsc notes list".
func RenderNotesTable(notes []Note) string {
	if len(notes) == 0 {
		return "No notes match.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tIMP\tUPDATED\tTOPICS\tSUMMARY")
	fmt.Fprintln(w, "--\t---\t-------\t------\t-------")
	for _, n := range notes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			ShortID(n.ID),
			n.Importance,
			noteDate(n),
			truncate(topicDisplay(n), 28),
			truncate(n.Summary, 64),
		)
	}
	w.Flush()
	return buf.String()
}

// RenderMatchedNotesTable renders matched notes as a table for "gsc notes get".
func RenderMatchedNotesTable(notes []MatchedNote) string {
	if len(notes) == 0 {
		return "No notes match.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tIMP\tMATCH\tSUMMARY")
	fmt.Fprintln(w, "--\t---\t-----\t-------")
	for _, mn := range notes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			ShortID(mn.Note.ID),
			mn.Note.Importance,
			truncate(mn.MatchReason, 20),
			truncate(mn.Note.Summary, 48),
		)
	}
	w.Flush()
	return buf.String()
}

// RenderNoteDetail renders a single note in detail.
func RenderNoteDetail(n Note) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s [%s]\n", n.ID, n.Importance)
	fmt.Fprintf(&sb, "Summary: %s\n\n", n.Summary)
	if n.Content != "" {
		fmt.Fprintf(&sb, "Content:\n  %s\n\n", n.Content)
	}
	writeList(&sb, "Glob Patterns", n.GlobPatterns)
	writeList(&sb, "Linked Files", n.LinkedFiles)
	writeList(&sb, "Tags", n.Tags)
	writeList(&sb, "Topics", formatTopics(n))
	fmt.Fprintf(&sb, "Created: %s\n", n.CreatedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&sb, "Updated: %s\n", n.UpdatedAt.Format("2006-01-02T15:04:05Z"))
	return sb.String()
}

func formatTopics(n Note) []string {
	var topics []string
	if n.Topic != "" {
		topics = append(topics, n.Topic)
	}
	topics = append(topics, n.RelatedTopics...)
	return topics
}

// RenderTagTable renders tags as a value/count table.
func RenderTagTable(tags []TagFacet) string {
	if len(tags) == 0 {
		return "No tags found.\n"
	}
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TAG\tNOTES")
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

// CountTags counts the distinct tags across notes.
func CountTags(records []Note) []TagFacet {
	index := map[string]*TagFacet{}
	var order []string
	for _, n := range records {
		for _, tag := range n.Tags {
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

// ShortID returns a compact, human-scannable form of a note ID.
// Uses 13 characters to reduce collisions while staying scannable.
func ShortID(id string) string {
	s := strings.TrimPrefix(id, "note_")
	if len(s) > 13 {
		return s[:13]
	}
	return s
}

func noteDate(n Note) string {
	t := n.UpdatedAt
	if t.IsZero() {
		t = n.CreatedAt
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

// topicDisplay returns the topic and related topics for display.
func topicDisplay(n Note) string {
	topics := []string{n.Topic}
	topics = append(topics, n.RelatedTopics...)
	return strings.Join(topics, ",")
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

// RenderOverview prints a human-readable digest of all notes.
func RenderOverview(records []Note) string {
	if len(records) == 0 {
		return "No notes recorded.\n"
	}
	var sb strings.Builder
	sb.WriteString(overviewHeader(records))
	sb.WriteString("\n")
	for _, n := range records {
		fmt.Fprintf(&sb, "  %-8s %s  %s\n", importanceTag(n.Importance), truncate(n.Summary, 72), tagHashes(n.Tags))
	}
	return sb.String()
}

func overviewHeader(records []Note) string {
	importance := map[string]int{}
	var latest time.Time
	for _, n := range records {
		importance[strings.ToLower(n.Importance)]++
		t := n.UpdatedAt
		if t.IsZero() {
			t = n.CreatedAt
		}
		if t.After(latest) {
			latest = t
		}
	}
	parts := []string{fmt.Sprintf("%d notes", len(records))}
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
