/**
 * Component: Topics Domain Models
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000001
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines topic data structures for the shared topic registry.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import "time"

const (
	DatabaseName = "gsc-topics"
)

type Topic struct {
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ValidationResult struct {
	Topic  Topic
	Errors []string
}

func (r ValidationResult) Valid() bool {
	return len(r.Errors) == 0
}
