/**
 * Component: Lessons Domain Models
 * Block-UUID: dd456c02-7a9e-4092-8cf3-a428e686b660
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines lesson draft, committed record, applies-to, AI provenance, and validation result data structures.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import "time"

const (
	DatabaseName = "gsc-lessons"
)

type AppliesTo struct {
	Files       []string `json:"files"`
	LinkedFiles []string `json:"linked_files"`
	Commands    []string `json:"commands"`
	Topics      []string `json:"topics,omitempty"` // LEGACY: kept for backward compatibility
}

type AIProvenance struct {
	Provider string `json:"provider"`
	ModelID  string `json:"model_id"`
	Agent    string `json:"agent"`
}

type Draft struct {
	Summary       string       `json:"summary"`
	Details       string       `json:"details"`
	Topic         string       `json:"topic"`
	RelatedTopics []string     `json:"related_topics"`
	AppliesTo     AppliesTo    `json:"applies_to"`
	Tags          []string     `json:"tags"`
	Importance    string       `json:"importance"`
	ReviewChecks  []string     `json:"review_checks"`
	AI            AIProvenance `json:"ai"`
}

type Record struct {
	ID             string       `json:"id"`
	SchemaVersion  string       `json:"schema_version"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	Summary        string       `json:"summary"`
	Details        string       `json:"details"`
	Topic          string       `json:"topic"`
	RelatedTopics  []string     `json:"related_topics"`
	AppliesTo      AppliesTo    `json:"applies_to"`
	Tags           []string     `json:"tags"`
	Keywords       []string     `json:"keywords"`
	ParentKeywords []string     `json:"parent_keywords"`
	Importance     string       `json:"importance"`
	ReviewChecks   []string     `json:"review_checks"`
	AI             AIProvenance `json:"ai"`
	ConfirmedBy    string       `json:"confirmed_by"`
	ConfirmedAt    time.Time    `json:"confirmed_at"`
}

type ValidationResult struct {
	Draft  Draft
	Errors []string
}

func (r ValidationResult) Valid() bool {
	return len(r.Errors) == 0
}

// NormalizeTopics migrates topics from legacy AppliesTo.Topics to top-level Topic/RelatedTopics.
// Call this after loading records to ensure consistent access.
func (r *Record) NormalizeTopics() {
	// If new field is empty, migrate from legacy
	if r.Topic == "" && len(r.AppliesTo.Topics) > 0 {
		r.Topic = r.AppliesTo.Topics[0]
		if len(r.AppliesTo.Topics) > 1 {
			r.RelatedTopics = r.AppliesTo.Topics[1:min(3, len(r.AppliesTo.Topics))]
		}
	}
	// Clear legacy field after migration
	r.AppliesTo.Topics = nil
}

// NormalizeDraftTopics migrates topics from legacy AppliesTo.Topics to top-level Topic/RelatedTopics in a draft.
func (d *Draft) NormalizeDraftTopics() {
	// If new field is empty, migrate from legacy
	if d.Topic == "" && len(d.AppliesTo.Topics) > 0 {
		d.Topic = d.AppliesTo.Topics[0]
		if len(d.AppliesTo.Topics) > 1 {
			d.RelatedTopics = d.AppliesTo.Topics[1:min(3, len(d.AppliesTo.Topics))]
		}
	}
	// Clear legacy field after migration
	d.AppliesTo.Topics = nil
}
