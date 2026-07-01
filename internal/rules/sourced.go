/**
 * Component: Sourced Rule Types
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines sourced wrappers for Rule and MatchedRule that preserve storage provenance (repo vs personal).
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

// SourcedRule pairs a Rule with its storage source.
type SourcedRule struct {
	Source gitsensescope.Source `json:"source"`
	Rule   Rule                 `json:"rule"`
}

// SourcedMatchedRule pairs a MatchedRule with its storage source.
type SourcedMatchedRule struct {
	Source      gitsensescope.Source `json:"source"`
	MatchedRule MatchedRule          `json:"matched_rule"`
}
