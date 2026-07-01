/**
 * Component: Notes Validator
 * Block-UUID: b8c9d0e1-f2a3-4567-bcde-567890123456
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Validates note shape, required fields, glob patterns, and bounded text.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import (
	"fmt"
	"path/filepath"
	"strings"

	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
)

func ValidateNote(n Note) []string {
	var errs []string

	if n.Summary == "" {
		errs = append(errs, "summary is required")
	}
	if len(n.Summary) > 240 {
		errs = append(errs, "summary must be 240 characters or fewer")
	}
	if len(n.Content) > 10000 {
		errs = append(errs, "content must be 10000 characters or fewer")
	}

	// Topic is required
	if n.Topic == "" {
		errs = append(errs, "topic is required")
	} else {
		// Validate topic against registry
		registry, regErr := topicstopkg.LoadRegistry()
		if regErr != nil {
			errs = append(errs, fmt.Sprintf("failed to load topic registry: %v", regErr))
		} else if !registry.Exists(n.Topic) {
			errs = append(errs, fmt.Sprintf("topic %q not registered; add with: gsc topics add %s --description \"...\"", n.Topic, n.Topic))
		}
	}

	// Validate related topics
	if len(n.RelatedTopics) > 2 {
		errs = append(errs, "maximum 2 related topics allowed")
	}
	seen := map[string]bool{n.Topic: true}
	for _, rt := range n.RelatedTopics {
		if rt == n.Topic {
			errs = append(errs, "related topic cannot equal primary topic")
		}
		if seen[rt] {
			errs = append(errs, "related topics must be unique")
		}
		seen[rt] = true
	}

	// Validate glob patterns
	for _, glob := range n.GlobPatterns {
		if filepath.IsAbs(glob) {
			errs = append(errs, fmt.Sprintf("glob must be relative, not absolute: %s", glob))
		}
		if strings.Contains(glob, "..") {
			errs = append(errs, fmt.Sprintf("glob must not contain ..: %s", glob))
		}
	}

	// Validate linked files
	for _, file := range n.LinkedFiles {
		if filepath.IsAbs(file) {
			errs = append(errs, fmt.Sprintf("linked file must be relative, not absolute: %s", file))
		}
		if strings.Contains(file, "..") {
			errs = append(errs, fmt.Sprintf("linked file must not contain ..: %s", file))
		}
	}

	// Validate importance
	switch n.Importance {
	case "low", "medium", "high", "":
	default:
		errs = append(errs, "importance must be one of: low, medium, high")
	}

	// Validate tags
	for _, tag := range n.Tags {
		if tag != slugify(tag) {
			errs = append(errs, fmt.Sprintf("tag %q must be a lowercase slug", tag))
		}
	}

	return errs
}

// ValidateAndNormalize validates and normalizes a note, returning a ValidationResult.
func ValidateAndNormalize(n Note) ValidationResult {
	normalized := normalizeDraft(n)
	errs := ValidateNote(normalized)
	return ValidationResult{
		Note:   normalized,
		Errors: errs,
	}
}
