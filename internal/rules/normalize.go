/**
 * Component: Rules Normalization Helpers
 * Block-UUID: d4e5f6a7-b8c9-0123-defa-234567890123
 * Parent-UUID: N/A
 * Version: 3.0.0
 * Description: Normalizes rule strings, slugs, arrays, globs, actions, keywords, and tool-trigger configuration.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0)
 */


package rules

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

// MatchesTag returns true if ruleTag matches filterTag using slug normalization and substring matching.
// Both tags are normalized via slugify (lowercase + replace non-alphanumeric with hyphens).
// The filter tag is matched as a substring of the rule tag.
func MatchesTag(ruleTag, filterTag string) bool {
	normalizedRule := slugify(ruleTag)
	normalizedFilter := slugify(filterTag)
	if normalizedFilter == "" {
		return false
	}
	return strings.Contains(normalizedRule, normalizedFilter)
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

// NormalizeGlob normalizes a glob pattern:
// - Strips leading ./
// - Converts absolute paths to repo-relative
// - Converts directory paths to recursive globs (dir/ -> dir/**)
func NormalizeGlob(pattern string) (string, error) {
	cleaned := strings.TrimSpace(pattern)
	if cleaned == "" {
		return "", nil
	}

	// Strip leading ./
	cleaned = strings.TrimPrefix(cleaned, "./")

	// Convert absolute path to repo-relative if possible
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

	// Normalize path separators
	cleaned = filepath.ToSlash(cleaned)

	// Convert directory paths to recursive globs
	if !strings.Contains(cleaned, "*") && !strings.Contains(cleaned, "?") {
		// Check if it looks like a directory (ends with /)
		if strings.HasSuffix(cleaned, "/") {
			cleaned = cleaned + "**"
		}
	}

	return cleaned, nil
}

func normalizeDraft(r Rule) Rule {
	r.Summary = cleanString(r.Summary)
	r.Details = cleanString(r.Details)
	// Migrate legacy topics first
	r.NormalizeTopics()
	r.Topic = slugify(r.Topic)
	r.RelatedTopics = normalizeSlugList(r.RelatedTopics)
	r.Importance = slugify(r.Importance)
	r.Owner = cleanString(r.Owner)
	r.Tags = normalizeSlugList(r.Tags)
	r.GlobPatterns = normalizeStringList(r.GlobPatterns)
	r.ExcludeGlobs = normalizeStringList(r.ExcludeGlobs)
	r.AppliesTo.Files = normalizeStringList(r.AppliesTo.Files)
	r.AppliesTo.LinkedFiles = normalizeStringList(r.AppliesTo.LinkedFiles)
	r.AppliesTo.Commands = normalizeStringList(r.AppliesTo.Commands)
	r.AI.Provider = cleanString(r.AI.Provider)
	r.AI.ModelID = cleanString(r.AI.ModelID)
	r.AI.Agent = cleanString(r.AI.Agent)
	// Normalize instructions (now just strings)
	for i := range r.Instructions {
		r.Instructions[i] = cleanString(r.Instructions[i])
	}
	// Normalize actions
	r.Actions = normalizeActions(r.Actions)
	r.Contact = normalizeStringList(r.Contact)

	// Normalize executable fields
	if r.Type == "" {
		r.Type = RuleTypeDeclarative
	}
	if r.Trigger != nil {
		r.Trigger.Entry = cleanString(r.Trigger.Entry)
		r.Trigger.Entry = strings.ReplaceAll(r.Trigger.Entry, "\\", "/")
		r.Trigger.Runtime = cleanString(r.Trigger.Runtime)
	}
	if r.InstrCfg != nil {
		r.InstrCfg.Mode = cleanString(r.InstrCfg.Mode)
		r.InstrCfg.Text = cleanString(r.InstrCfg.Text)
		r.InstrCfg.Query = cleanString(r.InstrCfg.Query)
	}
	if r.Frequency != nil {
		r.Frequency.Key = cleanString(r.Frequency.Key)
	}

	return r
}

// normalizeActions normalizes and deduplicates actions.
func normalizeActions(actions []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, action := range actions {
		a := strings.ToLower(strings.TrimSpace(action))
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// KeywordsFor returns keywords derived from topic and tags.
func KeywordsFor(r Rule) []string {
	values := append([]string{}, r.Tags...)
	values = append(values, r.Topic)
	values = append(values, r.RelatedTopics...)
	for _, command := range r.AppliesTo.Commands {
		values = append(values, slugify(command))
	}
	return normalizeSlugList(values)
}

// ParentKeywordsFor returns parent keywords derived from anchors and tags.
func ParentKeywordsFor(r Rule) []string {
	var values []string
	if len(r.GlobPatterns) > 0 || len(r.AppliesTo.Files) > 0 || len(r.AppliesTo.LinkedFiles) > 0 {
		values = append(values, "file-knowledge")
	}
	if len(r.AppliesTo.Commands) > 0 {
		values = append(values, "command-knowledge")
	}
	if r.Topic != "" {
		values = append(values, "topic-knowledge")
	}
	values = append(values, r.Tags...)
	return normalizeSlugList(values)
}
