/**
 * Component: Sourced Note Types
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines sourced wrappers for Note and MatchedNote that preserve storage provenance (repo vs personal).
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import (
	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

// SourcedNote pairs a Note with its storage source.
type SourcedNote struct {
	Source gitsensescope.Source `json:"source"`
	Note   Note                 `json:"note"`
}

// SourcedMatchedNote pairs a MatchedNote with its storage source.
type SourcedMatchedNote struct {
	Source      gitsensescope.Source `json:"source"`
	MatchedNote MatchedNote          `json:"matched_note"`
}
