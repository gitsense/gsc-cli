/**
 * Component: Topics Normalization Helpers
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000002
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Normalizes topic slugs and descriptions.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"regexp"
	"strings"
)

var slugPartRE = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugPartRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func NormalizeTopic(t Topic) Topic {
	t.Slug = Slugify(t.Slug)
	t.Description = strings.TrimSpace(t.Description)
	return t
}
