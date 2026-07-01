/**
 * Component: Rules Query and Filter
 * Block-UUID: a7b8c9d0-e1f2-3456-abcd-567890123456
 * Parent-UUID: N/A
 * Version: 3.1.0
 * Description: In-memory filtering, glob matching, text search, action filtering, prefix resolution, and structured match provenance over committed rule records. Supports lifecycle event filtering.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Updated-at: 2026-06-24T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), terrchen (v3.1.0)
 * Changelog:
 *   v3.1.0 - Add lifecycle event filtering to query functions
 */


package rules

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MatchedRule is a rule that matched a query, with match context.
type MatchedRule struct {
	Rule         Rule             `json:"rule"`
	MatchReason  string           `json:"match_reason"`
	Match        *MatchProvenance `json:"match,omitempty"`
	RuleHash     string           `json:"ruleHash"`
	TriggerHash  string           `json:"triggerHash,omitempty"`
}

// MatchProvenance captures structured match provenance for frequency tracking.
type MatchProvenance struct {
	Kind  string `json:"kind"`            // "file", "glob", "tag", "topic", "command", "unknown"
	Value string `json:"value"`           // The anchor that caused the match
	File  string `json:"file,omitempty"`  // The queried file (when applicable)
	Action string `json:"action,omitempty"` // The queried action (when applicable)
}

// MatchCandidate represents a possible match anchor with specificity for ordering.
type MatchCandidate struct {
	Kind        string
	Value       string
	Specificity int
}

// ListFilter captures the AND-combined predicates accepted by "gsc rules list".
type ListFilter struct {
	Tag       string
	Topic     string
	Importance string
	Action    string
	Type      string         // "declarative" or "executable"
	Event     LifecycleEvent // Canonical lifecycle event (default: pre_tool_use)
	ToolName  string         // Actual tool name to match against rule's tool_filter pattern
	Command   string         // Actual command to match against rule's command_filter pattern
	Prompt    string         // Actual prompt text to match against rule's prompt_filter pattern
}

// FilterRecords returns the records matching every non-empty predicate in f.
func FilterRecords(records []Rule, f ListFilter) []Rule {
	var out []Rule
	for _, r := range records {
		// Normalize topics for backward compatibility
		rr := r
		rr.NormalizeTopics()
		if f.Tag != "" && !containsFold(rr.Tags, f.Tag) {
			continue
		}
		if f.Topic != "" && !containsFold([]string{rr.Topic}, f.Topic) {
			continue
		}
		if f.Importance != "" && !strings.EqualFold(rr.Importance, f.Importance) {
			continue
		}
		if f.Action != "" && !containsAction(rr.Actions, f.Action) {
			continue
		}
		if f.Type != "" && string(rr.Type) != f.Type {
			// Default empty type to "declarative" for backward compatibility
			if rr.Type == "" && f.Type == "declarative" {
				// ok, include
			} else {
				continue
			}
		}
		// Filter by lifecycle event
		if f.Event != "" && rr.EffectiveEvent() != f.Event {
			continue
		}
		// Filter by tool name: match actual tool name against rule's tool_filter pattern
		// Filter by tool name (evaluate mode)
		if f.ToolName != "" {
			// With --tool: only include rules where tool_filter matches
			if rr.ToolFilter == "" {
				// Skip rules with tool_filter: null (generic match-any)
				continue
			}
			matched, err := path.Match(rr.ToolFilter, f.ToolName)
			if err != nil || !matched {
				continue
			}
		} else if f.ToolName == "" && f.Command == "" && f.Prompt == "" {
			// Only apply evaluate-mode filtering when at least one filter is provided
			// This allows gsc rules list to show all rules
		} else if f.ToolName == "" && f.Command == "" {
			// Without --tool and --command: only include rules with tool_filter: null (match any)
			if rr.ToolFilter != "" {
				continue
			}
		}
		// Filter by command (evaluate mode)
		if f.Command != "" {
			// With --command: only include rules where command_filter matches
			if rr.CommandFilter == "" {
				// Skip rules with command_filter: null (generic match-any)
				continue
			}
			re, err := regexp.Compile(rr.CommandFilter)
			if err != nil {
				continue
			}
			if !re.MatchString(f.Command) {
				continue
			}
		} else if f.ToolName == "" && f.Command == "" && f.Prompt == "" {
			// Only apply evaluate-mode filtering when at least one filter is provided
			// This allows gsc rules list to show all rules
		} else if f.ToolName == "" && f.Command == "" {
			// Without --tool and --command: only include rules with command_filter: null (match any)
			if rr.CommandFilter != "" {
				continue
			}
		}
		// Filter by prompt (evaluate mode)
		if f.Prompt != "" {
			// With --prompt: only include rules where prompt_filter matches
			if rr.PromptFilter == "" {
				// Skip rules with prompt_filter: null (generic match-any)
				continue
			}
			re, err := regexp.Compile(rr.PromptFilter)
			if err != nil {
				continue
			}
			if !re.MatchString(f.Prompt) {
				continue
			}
		} else if f.ToolName == "" && f.Command == "" && f.Prompt == "" {
			// Only apply evaluate-mode filtering when at least one filter is provided
			// This allows gsc rules list to show all rules
		} else if f.ToolName == "" && f.Command == "" && f.Prompt == "" {
			// Without --tool, --command, and --prompt: only include rules with prompt_filter: null (match any)
			if rr.PromptFilter != "" {
				continue
			}
		}
		out = append(out, rr)
	}
	return out
}

// containsAction checks if an action is in the actions list.
func containsAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

// GetRulesForFile returns rules matching a specific file path, optionally filtered by action and event.
func GetRulesForFile(records []Rule, filePath string, action string, event LifecycleEvent) []MatchedRule {
	var matched []MatchedRule
	for _, r := range records {
		// Filter by event if specified
		if event != "" && r.EffectiveEvent() != event {
			continue
		}
		// Filter by action if specified
		if action != "" && !containsAction(r.Actions, action) {
			continue
		}
		provenance := getFileMatchProvenance(r, filePath)
		if provenance != nil {
			provenance.File = filePath
			provenance.Action = action
			mr := MatchedRule{
				Rule:        r,
				MatchReason: fmt.Sprintf("%s: %s", provenance.Kind, provenance.Value),
				Match:       provenance,
				RuleHash:    r.ComputeRuleHash(),
			}
			if r.IsExecutable() {
				if th, err := r.ComputeTriggerHash(); err == nil {
					mr.TriggerHash = th
				}
			}
			matched = append(matched, mr)
		}
	}
	sortMatchedRules(matched)
	return matched
}

// GetFileMatchProvenance returns structured provenance for file matches.
// Prefers exact file match over glob, and more specific globs over broader ones.
func GetFileMatchProvenance(r Rule, filePath string) *MatchProvenance {
	return getFileMatchProvenance(r, filePath)
}

// getFileMatchProvenance returns structured provenance for file matches.
// Prefers exact file match over glob, and more specific globs over broader ones.
func getFileMatchProvenance(r Rule, filePath string) *MatchProvenance {
	normalized := filepath.ToSlash(filepath.Clean(filePath))

	// Check exact file matches first (highest priority)
	for _, f := range r.AppliesTo.Files {
		if filepath.ToSlash(filepath.Clean(f)) == normalized {
			return &MatchProvenance{
				Kind:  "file",
				Value: f,
			}
		}
	}

	// Check glob patterns, collect all matches
	var globMatches []MatchCandidate
	for _, glob := range r.GlobPatterns {
		if matchGlob(glob, normalized) {
			if !isExcluded(r.ExcludeGlobs, normalized) {
				globMatches = append(globMatches, MatchCandidate{
					Kind:        "glob",
					Value:       glob,
					Specificity: globSpecificity(glob),
				})
			}
		}
	}

	if len(globMatches) > 0 {
		// Return the most specific glob
		best := selectBestMatch(globMatches)
		return &best
	}

	return nil
}

// GetRulesForFileAllActions returns rules matching a specific file path (all actions).
func GetRulesForFileAllActions(records []Rule, filePath string, event LifecycleEvent) []MatchedRule {
	return GetRulesForFile(records, filePath, "", event)
}

// GetRulesForGlob returns rules matching a glob pattern, optionally filtered by event.
func GetRulesForGlob(records []Rule, pattern string, event LifecycleEvent) []MatchedRule {
	normalized, err := NormalizeGlob(pattern)
	if err != nil {
		return nil
	}
	var matched []MatchedRule
	for _, r := range records {
		// Filter by event if specified
		if event != "" && r.EffectiveEvent() != event {
			continue
		}
		provenance := getGlobMatchProvenance(r, normalized)
		if provenance != nil {
			mr := MatchedRule{
				Rule:        r,
				MatchReason: fmt.Sprintf("%s: %s", provenance.Kind, provenance.Value),
				Match:       provenance,
				RuleHash:    r.ComputeRuleHash(),
			}
			if r.IsExecutable() {
				if th, err := r.ComputeTriggerHash(); err == nil {
					mr.TriggerHash = th
				}
			}
			matched = append(matched, mr)
		}
	}
	sortMatchedRules(matched)
	return matched
}

// getGlobMatchProvenance returns structured provenance for glob matches.
func getGlobMatchProvenance(r Rule, pattern string) *MatchProvenance {
	// Check if rule's glob patterns overlap with the query pattern
	var globMatches []MatchCandidate
	for _, glob := range r.GlobPatterns {
		if globsOverlap(glob, pattern) {
			globMatches = append(globMatches, MatchCandidate{
				Kind:        "glob",
				Value:       glob,
				Specificity: globSpecificity(glob),
			})
		}
	}

	if len(globMatches) > 0 {
		best := selectBestMatch(globMatches)
		return &best
	}

	// Check if any applies_to.files match the query pattern
	for _, f := range r.AppliesTo.Files {
		if matchGlob(pattern, filepath.ToSlash(filepath.Clean(f))) {
			return &MatchProvenance{
				Kind:  "file",
				Value: f,
			}
		}
	}

	return nil
}

// GetRulesForTag returns rules with a specific tag, optionally filtered by event.
// Uses MatchesTag for consistent slug-based normalization and substring matching.
func GetRulesForTag(records []Rule, tag string, event LifecycleEvent) []MatchedRule {
	slug := slugify(tag)
	var matched []MatchedRule
	for _, r := range records {
		// Filter by event if specified
		if event != "" && r.EffectiveEvent() != event {
			continue
		}
		hasMatch := false
		for _, ruleTag := range r.Tags {
			if MatchesTag(ruleTag, tag) {
				hasMatch = true
				break
			}
		}
		if hasMatch {
			mr := MatchedRule{
				Rule:        r,
				MatchReason: fmt.Sprintf("tag: %s", slug),
				Match: &MatchProvenance{
					Kind:  "tag",
					Value: slug,
				},
				RuleHash: r.ComputeRuleHash(),
			}
			if r.IsExecutable() {
				if th, err := r.ComputeTriggerHash(); err == nil {
					mr.TriggerHash = th
				}
			}
			matched = append(matched, mr)
		}
	}
	sortMatchedRules(matched)
	return matched
}

// GetRulesForAction returns rules that match a specific action, optionally filtered by event.
// This is useful for querying rules for bash commands where no file path is available.
func GetRulesForAction(records []Rule, action string, event LifecycleEvent) []MatchedRule {
	var matched []MatchedRule
	for _, r := range records {
		// Filter by event if specified
		if event != "" && r.EffectiveEvent() != event {
			continue
		}
		// Check if rule has the specified action
		hasAction := false
		for _, a := range r.Actions {
			if a == action {
				hasAction = true
				break
			}
		}
		
		// Also match if rule has no actions specified (applies to all)
		if !hasAction && len(r.Actions) > 0 {
			continue
		}
		
		// Match rules by action
		if hasAction || len(r.Actions) == 0 {
			mr := MatchedRule{
				Rule:        r,
				MatchReason: fmt.Sprintf("action: %s", action),
				Match: &MatchProvenance{
					Kind:  "action",
					Value: action,
				},
				RuleHash: r.ComputeRuleHash(),
			}
			if r.IsExecutable() {
				if th, err := r.ComputeTriggerHash(); err == nil {
					mr.TriggerHash = th
				}
			}
			matched = append(matched, mr)
		}
	}
	sortMatchedRules(matched)
	return matched
}

// matchesFile checks if a rule matches a specific file path.
// Returns the match reason or empty string if no match.
func matchesFile(r Rule, filePath string) string {
	normalized := filepath.ToSlash(filepath.Clean(filePath))

	// Check exact file matches in applies_to.files
	for _, f := range r.AppliesTo.Files {
		if filepath.ToSlash(filepath.Clean(f)) == normalized {
			return fmt.Sprintf("file: %s", f)
		}
	}

	// Check glob patterns
	for _, glob := range r.GlobPatterns {
		if matchGlob(glob, normalized) {
			// Check exclusions
			if isExcluded(r.ExcludeGlobs, normalized) {
				continue
			}
			return fmt.Sprintf("glob: %s", glob)
		}
	}

	return ""
}

// matchesGlob checks if a rule matches a glob pattern.
// Returns the match reason or empty string if no match.
func matchesGlob(r Rule, pattern string) string {
	// Check if rule's glob patterns overlap with the query pattern
	for _, glob := range r.GlobPatterns {
		if globsOverlap(glob, pattern) {
			return fmt.Sprintf("glob: %s", glob)
		}
	}

	// Check if any applies_to.files match the query pattern
	for _, f := range r.AppliesTo.Files {
		if matchGlob(pattern, filepath.ToSlash(filepath.Clean(f))) {
			return fmt.Sprintf("file: %s", f)
		}
	}

	return ""
}

// matchGlob checks if a path matches a glob pattern.
// Supports ** for recursive directory matching.
func matchGlob(pattern, path string) bool {
	// Handle ** patterns
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, path)
	}
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// matchDoubleStar handles ** patterns for recursive directory matching.
// Supports patterns like:
//   - ** (match everything)
//   - **/*.go (match any .go file)
//   - dir/** (match everything under dir)
//   - **/dir/** (match everything under any dir)
//   - dir/**/*.go (match .go files under dir)
func matchDoubleStar(pattern, path string) bool {
	// Normalize separators
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// If pattern is just **, match everything
	if pattern == "**" {
		return true
	}

	// Split pattern and path into segments
	patSegs := strings.Split(pattern, "/")
	pathSegs := strings.Split(path, "/")

	return matchSegments(patSegs, pathSegs)
}

// matchSegments recursively matches pattern segments against path segments.
func matchSegments(patSegs, pathSegs []string) bool {
	// Base cases
	if len(patSegs) == 0 {
		return len(pathSegs) == 0
	}

	// Current pattern segment
	pat := patSegs[0]

	if pat == "**" {
		// ** can match zero or more path segments
		// Try matching ** with 0, 1, 2, ... segments
		for i := 0; i <= len(pathSegs); i++ {
			if matchSegments(patSegs[1:], pathSegs[i:]) {
				return true
			}
		}
		return false
	}

	// Non-** segment: must match exactly one path segment
	if len(pathSegs) == 0 {
		return false
	}

	matched, err := filepath.Match(pat, pathSegs[0])
	if err != nil || !matched {
		return false
	}

	return matchSegments(patSegs[1:], pathSegs[1:])
}

// globsOverlap checks if two glob patterns could match the same files.
// This is a simple heuristic: if one pattern is a prefix of the other, they overlap.
func globsOverlap(pattern1, pattern2 string) bool {
	// Simple case: exact match
	if pattern1 == pattern2 {
		return true
	}

	// Check if one is a prefix of the other
	p1 := strings.TrimSuffix(pattern1, "**")
	p2 := strings.TrimSuffix(pattern2, "**")

	if strings.HasPrefix(p1, p2) || strings.HasPrefix(p2, p1) {
		return true
	}

	return false
}

// isExcluded checks if a path matches any exclusion patterns.
func isExcluded(excludeGlobs []string, path string) bool {
	for _, glob := range excludeGlobs {
		if matchGlob(glob, path) {
			return true
		}
	}
	return false
}

// globSpecificity computes a specificity score for a glob pattern.
// Higher scores indicate more specific patterns.
// Score is based on:
//   - Exact path segments (no wildcards) score highest
//   - More path segments = higher score
//   - ** wildcards reduce specificity
//   - * wildcards are more specific than **
func globSpecificity(pattern string) int {
	if pattern == "**" {
		return 0
	}

	// Normalize separators
	pattern = filepath.ToSlash(pattern)
	score := 0

	// Split into segments
	segments := strings.Split(pattern, "/")
	for _, seg := range segments {
		if seg == "**" {
			score += 1 // Low score for **
		} else if strings.Contains(seg, "*") || strings.Contains(seg, "?") {
			score += 5 // Medium score for single * or ?
		} else {
			score += 10 // High score for literal segments
		}
	}

	return score
}

// selectBestMatch selects the most specific match from a list of candidates.
// Ties are broken by stable lexical order of the value.
func selectBestMatch(candidates []MatchCandidate) MatchProvenance {
	if len(candidates) == 0 {
		return MatchProvenance{Kind: "unknown"}
	}
	if len(candidates) == 1 {
		return MatchProvenance{
			Kind:  candidates[0].Kind,
			Value: candidates[0].Value,
		}
	}

	// Sort by specificity (descending), then by value (ascending) for stable ordering
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Specificity != candidates[j].Specificity {
			return candidates[i].Specificity > candidates[j].Specificity
		}
		return candidates[i].Value < candidates[j].Value
	})

	return MatchProvenance{
		Kind:  candidates[0].Kind,
		Value: candidates[0].Value,
	}
}

// sortMatchedRules sorts matched rules by importance (high > medium > low) then by created_at (newest first).
func sortMatchedRules(rules []MatchedRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		pi := importancePriority(rules[i].Rule.Importance)
		pj := importancePriority(rules[j].Rule.Importance)
		if pi != pj {
			return pi > pj
		}
		return rules[i].Rule.CreatedAt.After(rules[j].Rule.CreatedAt)
	})
}

func importancePriority(importance string) int {
	switch strings.ToLower(importance) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// DefaultSearchFields are the fields searched when none are specified.
var DefaultSearchFields = []string{"summary", "details", "instructions", "tags", "keywords"}

// ValidSearchFields are the field names accepted by --fields.
var ValidSearchFields = []string{"summary", "details", "instructions", "tags", "keywords", "owner"}

// SearchRecords returns records where query (case-insensitive substring) appears
// in any of the requested fields.
func SearchRecords(records []Rule, query string, fields []Rule) []Rule {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return records
	}
	if len(fields) == 0 {
		fields = records
	}
	var out []Rule
	for _, r := range records {
		if ruleMatches(r, q) {
			out = append(out, r)
		}
	}
	return out
}

func ruleMatches(r Rule, lowerQuery string) bool {
	if strings.Contains(strings.ToLower(r.Summary), lowerQuery) {
		return true
	}
	if strings.Contains(strings.ToLower(r.Details), lowerQuery) {
		return true
	}
	for _, inst := range r.Instructions {
		if strings.Contains(strings.ToLower(inst), lowerQuery) {
			return true
		}
	}
	for _, tag := range r.Tags {
		if strings.Contains(strings.ToLower(tag), lowerQuery) {
			return true
		}
	}
	for _, kw := range r.Keywords {
		if strings.Contains(strings.ToLower(kw), lowerQuery) {
			return true
		}
	}
	return false
}

// ResolveRecord finds a record by its exact ID, or by a unique substring/prefix.
func ResolveRecord(idOrPrefix string) (*Rule, error) {
	records, err := LoadRecords()
	if err != nil {
		return nil, err
	}
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}
	for i := range records {
		if records[i].ID == q {
			return &records[i], nil
		}
	}
	var matches []Rule
	for _, r := range records {
		if strings.Contains(r.ID, q) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous rule id %q matches %d rules; use a longer prefix", idOrPrefix, len(matches))
	}
}

func containsFold(values []string, query string) bool {
	q := strings.ToLower(query)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), q) {
			return true
		}
	}
	return false
}
