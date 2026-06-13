/**
 * Component: Lessons Draft Validator
 * Block-UUID: f340ff18-fff4-4b16-82ba-ea2542358756
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Validates lesson draft JSON shape, required fields, exact repo-relative file anchors, slug fields, command safety, and bounded text.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReadAndValidateDraft(path string) ValidationResult {
	var result ValidationResult

	data, err := os.ReadFile(path)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cannot read draft: %v", err))
		return result
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid JSON: %v", err))
		return result
	}
	if _, ok := raw["id"]; ok {
		result.Errors = append(result.Errors, "draft must not include id; gsc generates lsn_<uuid-v7>")
	}

	var draft Draft
	if err := json.Unmarshal(data, &draft); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid lesson draft shape: %v", err))
		return result
	}
	result.Draft = normalizeDraft(draft)
	result.Errors = append(result.Errors, ValidateDraft(result.Draft)...)
	return result
}

func ValidateDraft(d Draft) []string {
	var errs []string

	if d.Summary == "" {
		errs = append(errs, "summary is required")
	}
	if len(d.Summary) > 240 {
		errs = append(errs, "summary must be 240 characters or fewer")
	}
	if d.Details == "" {
		errs = append(errs, "details is required")
	}
	if len(d.Details) > 4000 {
		errs = append(errs, "details must be 4000 characters or fewer")
	}
	switch d.Importance {
	case "low", "medium", "high":
	default:
		errs = append(errs, "importance must be one of: low, medium, high")
	}

	if len(d.AppliesTo.Files)+len(d.AppliesTo.LinkedFiles)+len(d.AppliesTo.Commands)+len(d.AppliesTo.Topics)+len(d.Tags) == 0 {
		errs = append(errs, "at least one anchor is required: files, linked_files, commands, topics, or tags")
	}

	root, err := rootDir()
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to find repository root: %v", err))
		return errs
	}

	for _, path := range append([]string{}, d.AppliesTo.Files...) {
		errs = append(errs, validateRepoFile(root, "applies_to.files", path)...)
	}
	for _, path := range append([]string{}, d.AppliesTo.LinkedFiles...) {
		errs = append(errs, validateRepoFile(root, "applies_to.linked_files", path)...)
	}

	for _, tag := range d.Tags {
		if tag != slugify(tag) {
			errs = append(errs, fmt.Sprintf("tag %q must be a lowercase slug", tag))
		}
	}
	for _, topic := range d.AppliesTo.Topics {
		if topic != slugify(topic) {
			errs = append(errs, fmt.Sprintf("topic %q must be a lowercase slug", topic))
		}
	}
	for _, command := range d.AppliesTo.Commands {
		if strings.ContainsAny(command, ";&|") {
			errs = append(errs, fmt.Sprintf("command %q must not contain shell control operators", command))
		}
	}
	for _, check := range d.ReviewChecks {
		if len(check) > 300 {
			errs = append(errs, "review checks must be 300 characters or fewer")
		}
	}

	return errs
}

func validateRepoFile(root string, field string, rel string) []string {
	var errs []string
	if rel == "" {
		return []string{field + " contains an empty path"}
	}
	if filepath.IsAbs(rel) {
		errs = append(errs, fmt.Sprintf("%s path %q must be repo-relative, not absolute", field, rel))
	}
	if strings.Contains(rel, "..") {
		errs = append(errs, fmt.Sprintf("%s path %q must not contain ..", field, rel))
	}
	if strings.ContainsAny(rel, "*?[") || strings.Contains(rel, "...") {
		errs = append(errs, fmt.Sprintf("%s path %q must be an exact path, not a glob or shortened path", field, rel))
	}
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if cleaned == "." || strings.HasPrefix(cleaned, "..") {
		errs = append(errs, fmt.Sprintf("%s path %q is not a valid repo-relative file", field, rel))
		return errs
	}
	full := filepath.Join(root, cleaned)
	info, err := os.Stat(full)
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s path %q does not exist in the repository", field, rel))
		return errs
	}
	if info.IsDir() {
		errs = append(errs, fmt.Sprintf("%s path %q must be a file, not a directory", field, rel))
	}
	return errs
}
