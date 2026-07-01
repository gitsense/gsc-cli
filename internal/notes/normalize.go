/**
 * Component: Notes Normalization Helpers
 * Block-UUID: a7b8c9d0-e1f2-3456-abcd-456789012345
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Normalizes note strings, slugs, arrays, globs, and generated keywords.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var slugPartRE = regexp.MustCompile(`[^a-z0-9]+`)

func cleanString(s string) string {
	return strings.TrimSpace(s)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugPartRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func normalizeSlugList(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		slug := slugify(value)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

func normalizeStringList(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		cleaned := cleanString(strings.ReplaceAll(value, "\\", "/"))
		if cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		out = append(out, cleaned)
	}
	sort.Strings(out)
	return out
}

// NormalizeGlob normalizes a glob pattern.
func NormalizeGlob(pattern string) (string, error) {
	cleaned := strings.TrimSpace(pattern)
	if cleaned == "" {
		return "", nil
	}

	cleaned = strings.TrimPrefix(cleaned, "./")

	if filepath.IsAbs(cleaned) {
		root, err := rootDir()
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, cleaned)
		if err != nil {
			return "", err
		}
		cleaned = rel
	}

	cleaned = filepath.ToSlash(cleaned)

	if !strings.Contains(cleaned, "*") && !strings.Contains(cleaned, "?") {
		if strings.HasSuffix(cleaned, "/") {
			cleaned = cleaned + "**"
		}
	}

	return cleaned, nil
}

func normalizeDraft(n Note) Note {
	n.Summary = cleanString(n.Summary)
	n.Content = cleanString(n.Content)
	n.Topic = slugify(n.Topic)
	n.RelatedTopics = normalizeSlugList(n.RelatedTopics)
	n.Importance = slugify(n.Importance)
	n.Tags = normalizeSlugList(n.Tags)
	n.GlobPatterns = normalizeStringList(n.GlobPatterns)
	n.LinkedFiles = normalizeStringList(n.LinkedFiles)
	return n
}

// KeywordsFor returns keywords derived from topic and tags.
func KeywordsFor(n Note) []string {
	values := []string{n.Topic}
	values = append(values, n.RelatedTopics...)
	values = append(values, n.Tags...)
	return normalizeSlugList(values)
}

// ParentKeywordsFor returns parent keywords derived from topic, tags, and glob patterns.
func ParentKeywordsFor(n Note) []string {
	var values []string
	if len(n.GlobPatterns) > 0 || len(n.LinkedFiles) > 0 {
		values = append(values, "file-knowledge")
	}
	if n.Topic != "" {
		values = append(values, "topic-knowledge")
	}
	values = append(values, n.Tags...)
	return normalizeSlugList(values)
}
