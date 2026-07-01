/**
 * Component: Notes Domain Models
 * Block-UUID: b2c3d4e5-f6a7-8901-bcde-f01234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines note data structures for searchable scratchpad notes.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import "time"

const (
	DatabaseName = "gsc-notes"
)

type Note struct {
	ID             string    `json:"id"`
	SchemaVersion  string    `json:"schema_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Summary        string    `json:"summary"`
	Content        string    `json:"content"`
	Topic          string    `json:"topic"`
	RelatedTopics  []string  `json:"related_topics"`
	GlobPatterns   []string  `json:"glob_patterns,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	LinkedFiles    []string  `json:"linked_files,omitempty"`
	Keywords       []string  `json:"keywords,omitempty"`
	ParentKeywords []string  `json:"parent_keywords,omitempty"`
	Importance     string    `json:"importance,omitempty"`
}

type ValidationResult struct {
	Note   Note
	Errors []string
}

func (r ValidationResult) Valid() bool {
	return len(r.Errors) == 0
}
