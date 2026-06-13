/**
 * Component: Lessons Normalization Helpers
 * Block-UUID: d5a2614d-dcd9-4147-a9d0-71c5f6842772
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Normalizes lesson draft strings, slugs, arrays, generated keywords, and parent keyword metadata.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
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

func normalizeDraft(d Draft) Draft {
	d.Summary = cleanString(d.Summary)
	d.Details = cleanString(d.Details)
	d.Importance = slugify(d.Importance)
	d.Tags = normalizeSlugList(d.Tags)
	d.ReviewChecks = normalizeStringList(d.ReviewChecks)
	d.AppliesTo.Files = normalizeStringList(d.AppliesTo.Files)
	d.AppliesTo.LinkedFiles = normalizeStringList(d.AppliesTo.LinkedFiles)
	d.AppliesTo.Commands = normalizeStringList(d.AppliesTo.Commands)
	d.AppliesTo.Topics = normalizeSlugList(d.AppliesTo.Topics)
	d.AI.Provider = cleanString(d.AI.Provider)
	d.AI.ModelID = cleanString(d.AI.ModelID)
	d.AI.Agent = cleanString(d.AI.Agent)
	return d
}

func keywordsFor(d Draft) []string {
	values := append([]string{}, d.Tags...)
	values = append(values, d.AppliesTo.Topics...)
	for _, command := range d.AppliesTo.Commands {
		values = append(values, slugify(command))
	}
	return normalizeSlugList(values)
}

func parentKeywordsFor(d Draft) []string {
	var values []string
	if len(d.AppliesTo.Files) > 0 || len(d.AppliesTo.LinkedFiles) > 0 {
		values = append(values, "file-knowledge")
	}
	if len(d.AppliesTo.Commands) > 0 {
		values = append(values, "command-knowledge")
	}
	if len(d.AppliesTo.Topics) > 0 {
		values = append(values, "topic-knowledge")
	}
	values = append(values, d.Tags...)
	return normalizeSlugList(values)
}
