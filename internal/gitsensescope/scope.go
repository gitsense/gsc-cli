/**
 * Component: GitSense Scope Primitives
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines read scopes, write targets, source provenance, and knowledge-kind types for GitSense storage resolution.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package gitsensescope

import "fmt"

// Scope represents a read scope that determines which storage locations to query.
type Scope string

const (
	ScopeRepo     Scope = "repo"
	ScopePersonal Scope = "personal"
	ScopeAll      Scope = "all"
)

var validScopes = []Scope{ScopeRepo, ScopePersonal, ScopeAll}

// ParseScope parses a string into a Scope. Empty string defaults to ScopeAll.
func ParseScope(value string) (Scope, error) {
	if value == "" {
		return ScopeAll, nil
	}
	s := Scope(value)
	for _, valid := range validScopes {
		if s == valid {
			return s, nil
		}
	}
	return "", fmt.Errorf("invalid scope %q: must be one of repo, personal, all", value)
}

// Target represents a write target that determines where to store new records.
type Target string

const (
	TargetRepo     Target = "repo"
	TargetPersonal Target = "personal"
)

var validTargets = []Target{TargetRepo, TargetPersonal}

// ParseTarget parses a string into a Target. Empty string returns an error.
func ParseTarget(value string) (Target, error) {
	if value == "" {
		return "", fmt.Errorf("write target is required: must be one of repo, personal")
	}
	t := Target(value)
	for _, valid := range validTargets {
		if t == valid {
			return t, nil
		}
	}
	return "", fmt.Errorf("invalid target %q: must be one of repo, personal", value)
}

// Source records where a loaded item originated.
type Source string

const (
	SourceRepo     Source = "repo"
	SourcePersonal Source = "personal"
)

// Kind identifies a knowledge type (notes, rules, lessons).
type Kind string

const (
	KindNotes   Kind = "notes"
	KindRules   Kind = "rules"
	KindLessons Kind = "lessons"
)

var validKinds = []Kind{KindNotes, KindRules, KindLessons}

// IsValidKind checks if the kind is a valid knowledge type.
func IsValidKind(kind Kind) bool {
	for _, valid := range validKinds {
		if kind == valid {
			return true
		}
	}
	return false
}
