/**
 * Component: Sourced Lesson Types
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines sourced wrappers for Record that preserve storage provenance (repo vs personal).
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package lessons

import (
	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

// SourcedLesson pairs a Record with its storage source.
type SourcedLesson struct {
	Source gitsensescope.Source `json:"source"`
	Lesson Record               `json:"lesson"`
}
