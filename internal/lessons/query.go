/**
 * Component: Lessons Query and Filter
 * Block-UUID: 4d2c8f6a-1b7e-4a3c-9f21-7c5e0a9b4d83
 * Parent-UUID: N/A
 * Version: 1.2.0
 * Description: In-memory filtering, text search, prefix resolution, faceting, limiting, and input validation (search fields, importance) over committed lesson records.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0), claude-opus-4-8 (v1.1.0), claude-opus-4-8 (v1.2.0)
 */


package lessons

import (
	"fmt"
	"sort"
	"strings"
)

// ListFilter captures the AND-combined predicates accepted by "gsc lessons list".
// Empty fields are ignored. Tag/Topic/File match case-insensitively as substrings;
// Importance matches case-insensitively as a whole value.
type ListFilter struct {
	Tag        string
	Topic      string
	File       string
	Importance string
}

// FilterRecords returns the records matching every non-empty predicate in f.
func FilterRecords(records []Record, f ListFilter) []Record {
	var out []Record
	for _, r := range records {
		if f.Tag != "" && !containsFold(r.Tags, f.Tag) {
			continue
		}
		if f.Topic != "" && !containsFold(r.AppliesTo.Topics, f.Topic) {
			continue
		}
		if f.File != "" && !containsFold(r.AppliesTo.Files, f.File) {
			continue
		}
		if f.Importance != "" && !strings.EqualFold(r.Importance, f.Importance) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// DefaultSearchFields are the fields searched when none are specified.
var DefaultSearchFields = []string{"summary", "details", "tags", "topics", "keywords"}

// ValidSearchFields are the field names accepted by --fields.
var ValidSearchFields = []string{"summary", "details", "tags", "topics", "keywords", "files", "commands"}

// ValidImportanceValues are the importance levels a lesson may declare.
var ValidImportanceValues = []string{"low", "medium", "high"}

// ValidateSearchFields returns an error if any requested field is not searchable.
func ValidateSearchFields(fields []string) error {
	for _, field := range fields {
		if !equalsFoldAny(strings.TrimSpace(field), ValidSearchFields) {
			return fmt.Errorf("unknown search field %q (valid: %s)", field, strings.Join(ValidSearchFields, ", "))
		}
	}
	return nil
}

func equalsFoldAny(value string, options []string) bool {
	for _, option := range options {
		if strings.EqualFold(value, option) {
			return true
		}
	}
	return false
}

// ValidateImportance returns an error if v is non-empty and not a known level.
func ValidateImportance(v string) error {
	if v == "" {
		return nil
	}
	for _, level := range ValidImportanceValues {
		if strings.EqualFold(v, level) {
			return nil
		}
	}
	return fmt.Errorf("unknown importance %q (valid: %s)", v, strings.Join(ValidImportanceValues, ", "))
}

// SearchRecords returns records where query (case-insensitive substring) appears
// in any of the requested fields. Empty fields defaults to DefaultSearchFields.
func SearchRecords(records []Record, query string, fields []string) []Record {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return records
	}
	if len(fields) == 0 {
		fields = DefaultSearchFields
	}
	var out []Record
	for _, r := range records {
		if recordMatches(r, q, fields) {
			out = append(out, r)
		}
	}
	return out
}

// LimitRecords truncates records to at most limit entries. A limit <= 0 means unlimited.
func LimitRecords(records []Record, limit int) []Record {
	if limit > 0 && len(records) > limit {
		return records[:limit]
	}
	return records
}

// Facet is one distinct value of a multi-valued field (such as a topic or tag),
// the number of lessons that use it, and the IDs of those lessons.
type Facet struct {
	Value     string   `json:"value"`
	Count     int      `json:"count"`
	LessonIDs []string `json:"lesson_ids"`
}

// CountFacet tallies the distinct values of a field across records, sorted by
// descending count then value. LessonIDs holds the full IDs of contributing
// lessons so callers can connect a facet back to its lessons.
func CountFacet(records []Record, field string) []Facet {
	index := map[string]*Facet{}
	var order []string
	for _, r := range records {
		for _, value := range recordFieldValues(r, field) {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			facet, ok := index[value]
			if !ok {
				facet = &Facet{Value: value}
				index[value] = facet
				order = append(order, value)
			}
			facet.Count++
			facet.LessonIDs = append(facet.LessonIDs, r.ID)
		}
	}
	facets := make([]Facet, 0, len(order))
	for _, value := range order {
		facets = append(facets, *index[value])
	}
	sort.SliceStable(facets, func(i, j int) bool {
		if facets[i].Count != facets[j].Count {
			return facets[i].Count > facets[j].Count
		}
		return facets[i].Value < facets[j].Value
	})
	return facets
}

// LimitFacets truncates facets to at most limit entries. A limit <= 0 means unlimited.
func LimitFacets(facets []Facet, limit int) []Facet {
	if limit > 0 && len(facets) > limit {
		return facets[:limit]
	}
	return facets
}

// ResolveRecord finds a record by its exact ID, or by a unique substring/prefix
// (such as the short ID shown in list/search tables). Returns nil if nothing
// matches, and an error if the value is ambiguous.
func ResolveRecord(idOrPrefix string) (*Record, error) {
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
	var matches []Record
	for _, r := range records {
		if strings.Contains(r.ID, q) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous lesson id %q matches %d lessons; use a longer prefix", idOrPrefix, len(matches))
	}
}

func recordMatches(r Record, lowerQuery string, fields []string) bool {
	for _, field := range fields {
		for _, value := range recordFieldValues(r, field) {
			if strings.Contains(strings.ToLower(value), lowerQuery) {
				return true
			}
		}
	}
	return false
}

func recordFieldValues(r Record, field string) []string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "summary":
		return []string{r.Summary}
	case "details":
		return []string{r.Details}
	case "tags":
		return r.Tags
	case "topics":
		return r.AppliesTo.Topics
	case "keywords":
		return r.Keywords
	case "files":
		return r.AppliesTo.Files
	case "commands":
		return r.AppliesTo.Commands
	default:
		return nil
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
