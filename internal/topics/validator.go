/**
 * Component: Topics Validator
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000003
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Validates topic slugs, descriptions, and uniqueness constraints.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"fmt"
	"strings"
)

func ValidateTopic(t Topic, existingSlugs []string) []string {
	var errs []string

	// Slug required
	if t.Slug == "" {
		errs = append(errs, "slug is required")
	}

	// Slug must be normalized
	if t.Slug != Slugify(t.Slug) {
		errs = append(errs, "slug must be lowercase hyphenated (e.g., data-layer)")
	}

	// Slug cannot already exist
	for _, existing := range existingSlugs {
		if strings.EqualFold(t.Slug, existing) {
			errs = append(errs, fmt.Sprintf("topic %q already exists", t.Slug))
			break
		}
	}

	// Description required
	if t.Description == "" {
		errs = append(errs, "description is required")
	}

	// Description bounded
	if len(t.Description) > 500 {
		errs = append(errs, "description must be 500 characters or fewer")
	}

	return errs
}

func ValidateTopicSlug(slug string, registry *Registry) []string {
	var errs []string

	if slug == "" {
		errs = append(errs, "topic is required")
		return errs
	}

	if !registry.Exists(slug) {
		errs = append(errs, fmt.Sprintf("topic %q not registered; add with: gsc topics add %s --description \"...\"", slug, slug))
	}

	return errs
}

func ValidateRelatedTopics(topic string, relatedTopics []string, registry *Registry) []string {
	var errs []string

	if len(relatedTopics) > 2 {
		errs = append(errs, "maximum 2 related topics allowed")
	}

	seen := map[string]bool{topic: true}
	for _, rt := range relatedTopics {
		if !registry.Exists(rt) {
			errs = append(errs, fmt.Sprintf("related topic %q not registered", rt))
		}
		if rt == topic {
			errs = append(errs, "related topic cannot equal primary topic")
		}
		if seen[rt] {
			errs = append(errs, "related topics must be unique")
		}
		seen[rt] = true
	}

	return errs
}
