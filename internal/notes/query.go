/**
 * Component: Notes Query and Filter
 * Block-UUID: f6a7b8c9-d0e1-2345-fabc-345678901234
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: In-memory filtering, glob matching, text search, and prefix resolution over committed note records.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// MatchedNote is a note that matched a query, with match context.
type MatchedNote struct {
	Note        Note   `json:"note"`
	MatchReason string `json:"match_reason"`
}

// ListFilter captures the AND-combined predicates accepted by "gsc notes list".
type ListFilter struct {
	Tag        string
	Topic      string
	Importance string
}

// FilterRecords returns the records matching every non-empty predicate in f.
func FilterRecords(records []Note, f ListFilter) []Note {
	var out []Note
	for _, r := range records {
		if f.Tag != "" && !containsFold(r.Tags, f.Tag) {
			continue
		}
		if f.Topic != "" && !containsFold([]string{r.Topic}, f.Topic) {
			continue
		}
		if f.Importance != "" && !strings.EqualFold(r.Importance, f.Importance) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// GetNotesForFile returns notes matching a specific file path.
func GetNotesForFile(records []Note, filePath string) []MatchedNote {
	var matched []MatchedNote
	for _, n := range records {
		if reason := matchesFile(n, filePath); reason != "" {
			matched = append(matched, MatchedNote{
				Note:        n,
				MatchReason: reason,
			})
		}
	}
	sortMatchedNotes(matched)
	return matched
}

// GetNotesForGlob returns notes matching a glob pattern.
func GetNotesForGlob(records []Note, pattern string) []MatchedNote {
	normalized, err := NormalizeGlob(pattern)
	if err != nil {
		return nil
	}
	var matched []MatchedNote
	for _, n := range records {
		if reason := matchesGlob(n, normalized); reason != "" {
			matched = append(matched, MatchedNote{
				Note:        n,
				MatchReason: reason,
			})
		}
	}
	sortMatchedNotes(matched)
	return matched
}

// GetNotesForTag returns notes with a specific tag.
func GetNotesForTag(records []Note, tag string) []MatchedNote {
	slug := slugify(tag)
	var matched []MatchedNote
	for _, n := range records {
		if containsFold(n.Tags, slug) {
			matched = append(matched, MatchedNote{
				Note:        n,
				MatchReason: fmt.Sprintf("tag: %s", slug),
			})
		}
	}
	sortMatchedNotes(matched)
	return matched
}

// matchesFile checks if a note matches a specific file path.
func matchesFile(n Note, filePath string) string {
	normalized := filepath.ToSlash(filepath.Clean(filePath))

	// Check linked files
	for _, f := range n.LinkedFiles {
		if filepath.ToSlash(filepath.Clean(f)) == normalized {
			return fmt.Sprintf("file: %s", f)
		}
	}

	// Check glob patterns
	for _, glob := range n.GlobPatterns {
		if matchGlob(glob, normalized) {
			return fmt.Sprintf("glob: %s", glob)
		}
	}

	return ""
}

// matchesGlob checks if a note matches a glob pattern.
func matchesGlob(n Note, pattern string) string {
	// Check if note's glob patterns overlap with the query pattern
	for _, glob := range n.GlobPatterns {
		if globsOverlap(glob, pattern) {
			return fmt.Sprintf("glob: %s", glob)
		}
	}

	// Check if any linked files match the query pattern
	for _, f := range n.LinkedFiles {
		if matchGlob(pattern, filepath.ToSlash(filepath.Clean(f))) {
			return fmt.Sprintf("file: %s", f)
		}
	}

	return ""
}

// matchGlob checks if a path matches a glob pattern.
func matchGlob(pattern, path string) bool {
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, path)
	}
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// matchDoubleStar handles ** patterns.
func matchDoubleStar(pattern, path string) bool {
	parts := strings.Split(pattern, "**")
	if len(parts) == 0 {
		return false
	}
	if len(parts) == 1 && parts[0] == "" {
		return true
	}
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		return matchSuffix(suffix, path)
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return strings.HasPrefix(path, prefix)
	}
	if len(parts) == 2 {
		prefix := parts[0]
		suffix := parts[1]
		if strings.HasPrefix(path, prefix) {
			rest := strings.TrimPrefix(path, prefix)
			return matchSuffix(suffix, rest)
		}
	}
	return false
}

// matchSuffix checks if a path matches a suffix pattern.
func matchSuffix(suffix, path string) bool {
	if suffix == "" {
		return true
	}
	suffix = strings.TrimPrefix(suffix, "/")
	components := strings.Split(path, "/")
	for _, comp := range components {
		matched, err := filepath.Match(suffix, comp)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// globsOverlap checks if two glob patterns could match the same files.
func globsOverlap(pattern1, pattern2 string) bool {
	if pattern1 == pattern2 {
		return true
	}
	p1 := strings.TrimSuffix(pattern1, "**")
	p2 := strings.TrimSuffix(pattern2, "**")
	if strings.HasPrefix(p1, p2) || strings.HasPrefix(p2, p1) {
		return true
	}
	return false
}

// sortMatchedNotes sorts matched notes by importance then by created_at (newest first).
func sortMatchedNotes(notes []MatchedNote) {
	sort.SliceStable(notes, func(i, j int) bool {
		pi := importancePriority(notes[i].Note.Importance)
		pj := importancePriority(notes[j].Note.Importance)
		if pi != pj {
			return pi > pj
		}
		return notes[i].Note.CreatedAt.After(notes[j].Note.CreatedAt)
	})
}

func importancePriority(importance string) int {
	switch strings.ToLower(importance) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// SearchRecords returns records where query appears in summary, content, tags, or keywords.
func SearchRecords(records []Note, query string) []Note {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return records
	}
	var out []Note
	for _, n := range records {
		if noteMatches(n, q) {
			out = append(out, n)
		}
	}
	return out
}

func noteMatches(n Note, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(n.Summary), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(n.Content), lowerQuery) {
		return true
	}
	for _, tag := range n.Tags {
		if strings.Contains(strings.ToLower(tag), lowerQuery) {
			return true
		}
	}
	for _, kw := range n.Keywords {
		if strings.Contains(strings.ToLower(kw), lowerQuery) {
			return true
		}
	}
	return false
}

// ResolveRecord finds a record by its exact ID, or by a unique substring/prefix.
func ResolveRecord(idOrPrefix string) (*Note, error) {
	records, err := LoadRecords()
	if err != nil {
		return nil, err
	}
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}
	for i := range records {
		if records[i].ID == q {
			return &records[i], nil
		}
	}
	var matches []Note
	for _, n := range records {
		if strings.Contains(n.ID, q) {
			matches = append(matches, n)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous note id %q matches %d notes; use a longer prefix", idOrPrefix, len(matches))
	}
}

func containsFold(values []string, query string) bool {
	q := strings.ToLower(query)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), q) {
			return true
		}
	}
	return false
}
