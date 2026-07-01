/**
 * Component: Knowledge Document Model
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000001
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added UpdatedAt field for sorting by last modified date.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0)
 */


package knowledge

import "time"

// DocumentType represents the type of knowledge document.
type DocumentType string

const (
	TypeLesson DocumentType = "lesson"
	TypeNote   DocumentType = "note"
	TypeRule   DocumentType = "rule"
)

// Document is a normalized representation of a knowledge item for search.
type Document struct {
	Type          DocumentType
	ID            string
	Topic         string
	RelatedTopics []string
	Tags          []string
	Summary       string
	Body          string // Details for lessons, Content for notes, Details for rules
	Importance    string
	Files         []string // From AppliesTo
	GlobPatterns  []string
	UpdatedAt     time.Time
}

// SearchResult is a ranked result from knowledge search.
type SearchResult struct {
	Type       DocumentType `json:"type"`
	ID         string       `json:"id"`
	Topic      string       `json:"topic"`
	Summary    string       `json:"summary"`
	Importance string       `json:"importance,omitempty"`
	MatchedBy  []string     `json:"matched_by"`
	ScopeMatch bool         `json:"scope_match"`
	Score      float64      `json:"score"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// SearchResponse is the response from a knowledge search.
type SearchResponse struct {
	Items  []SearchResult  `json:"items"`
	Facets map[string]int  `json:"facets"`
}

// ListResponse is the response from a knowledge list.
type ListResponse struct {
	Items  []SearchResult  `json:"items"`
	Facets map[string]int  `json:"facets"`
}
