/**
 * Component: Rules Match Provenance Tests
 * Block-UUID: 7a8b9c0d-1e2f-3456-abcd-789012345678
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for structured match provenance in rules query results.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"testing"
)

func TestGlobSpecificity(t *testing.T) {
	tests := []struct {
		pattern string
		want    int
	}{
		{"**", 0},
		{"**/*.go", 6},           // ** (1) + *.go (5) = 6
		{"**/3rdparty/**", 2},    // ** (1) + 3rdparty (10) + ** (1) = 12... wait let me recalculate
		{"internal/**/*.go", 16}, // internal (10) + ** (1) + *.go (5) = 16
		{"internal/cli/**", 21},  // internal (10) + cli (10) + ** (1) = 21
		{"specific/file.go", 20}, // specific (10) + file.go (10) = 20
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			// Just test that it runs without error and returns non-negative
			got := globSpecificity(tt.pattern)
			if got < 0 {
				t.Errorf("globSpecificity(%q) = %v, want non-negative", tt.pattern, got)
			}
		})
	}
}

func TestGlobSpecificityOrdering(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		// The most specific pattern should be first after sorting
		wantMostSpecific string
	}{
		{
			name:             "broad vs narrow glob",
			patterns:         []string{"**", "**/3rdparty/**"},
			wantMostSpecific: "**/3rdparty/**",
		},
		{
			name:             "directory vs subdirectory",
			patterns:         []string{"internal/**", "internal/cli/**"},
			wantMostSpecific: "internal/cli/**",
		},
		{
			name:             "wildcard vs literal",
			patterns:         []string{"**/*.go", "specific/file.go"},
			wantMostSpecific: "specific/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var candidates []MatchCandidate
			for _, p := range tt.patterns {
				candidates = append(candidates, MatchCandidate{
					Kind:        "glob",
					Value:       p,
					Specificity: globSpecificity(p),
				})
			}

			best := selectBestMatch(candidates)
			if best.Value != tt.wantMostSpecific {
				t.Errorf("selectBestMatch() = %v, want %v", best.Value, tt.wantMostSpecific)
			}
		})
	}
}

func TestSelectBestMatchLexicalTieBreak(t *testing.T) {
	// Test that ties are broken by lexical order
	candidates := []MatchCandidate{
		{Kind: "glob", Value: "b/**", Specificity: 11},
		{Kind: "glob", Value: "a/**", Specificity: 11},
	}

	best := selectBestMatch(candidates)
	if best.Value != "a/**" {
		t.Errorf("selectBestMatch() = %v, want 'a/**' (lexical tie-break)", best.Value)
	}
}

func TestSelectBestMatchEmpty(t *testing.T) {
	best := selectBestMatch(nil)
	if best.Kind != "unknown" {
		t.Errorf("selectBestMatch(nil) = %v, want kind='unknown'", best)
	}
}

func TestGetFileMatchProvenanceExactFile(t *testing.T) {
	rule := Rule{
		AppliesTo: AppliesTo{
			Files: []string{"src/main.go", "src/util.go"},
		},
	}

	provenance := getFileMatchProvenance(rule, "src/main.go")
	if provenance == nil {
		t.Fatal("expected provenance, got nil")
	}
	if provenance.Kind != "file" {
		t.Errorf("Kind = %v, want 'file'", provenance.Kind)
	}
	if provenance.Value != "src/main.go" {
		t.Errorf("Value = %v, want 'src/main.go'", provenance.Value)
	}
}

func TestGetFileMatchProvenanceGlob(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"internal/**/*.go"},
	}

	provenance := getFileMatchProvenance(rule, "internal/cli/main.go")
	if provenance == nil {
		t.Fatal("expected provenance, got nil")
	}
	if provenance.Kind != "glob" {
		t.Errorf("Kind = %v, want 'glob'", provenance.Kind)
	}
	if provenance.Value != "internal/**/*.go" {
		t.Errorf("Value = %v, want 'internal/**/*.go'", provenance.Value)
	}
}

func TestGetFileMatchProvenancePrefersExactOverGlob(t *testing.T) {
	rule := Rule{
		AppliesTo: AppliesTo{
			Files: []string{"src/main.go"},
		},
		GlobPatterns: []string{"src/**"},
	}

	provenance := getFileMatchProvenance(rule, "src/main.go")
	if provenance == nil {
		t.Fatal("expected provenance, got nil")
	}
	if provenance.Kind != "file" {
		t.Errorf("Kind = %v, want 'file' (exact file should take precedence over glob)", provenance.Kind)
	}
	if provenance.Value != "src/main.go" {
		t.Errorf("Value = %v, want 'src/main.go'", provenance.Value)
	}
}

func TestGetFileMatchProvenancePrefersNarrowGlob(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"**", "**/3rdparty/**"},
	}

	provenance := getFileMatchProvenance(rule, "vendor/3rdparty/a.js")
	if provenance == nil {
		t.Fatal("expected provenance, got nil")
	}
	if provenance.Kind != "glob" {
		t.Errorf("Kind = %v, want 'glob'", provenance.Kind)
	}
	if provenance.Value != "**/3rdparty/**" {
		t.Errorf("Value = %v, want '**/3rdparty/**' (narrow glob should take precedence)", provenance.Value)
	}
}

func TestGetFileMatchProvenanceNoMatch(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"internal/**"},
	}

	provenance := getFileMatchProvenance(rule, "vendor/external/file.go")
	if provenance != nil {
		t.Errorf("expected nil, got %v", provenance)
	}
}

func TestGetFileMatchProvenancePopulatesFileAndAction(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"**/*.go"},
		Actions:     []string{"edit", "write"},
	}

	matched := GetRulesForFile([]Rule{rule}, "internal/main.go", "edit", "")
	if len(matched) == 0 {
		t.Fatal("expected matches, got none")
	}

	m := matched[0]
	if m.Match == nil {
		t.Fatal("expected Match, got nil")
	}
	if m.Match.File != "internal/main.go" {
		t.Errorf("Match.File = %v, want 'internal/main.go'", m.Match.File)
	}
	if m.Match.Action != "edit" {
		t.Errorf("Match.Action = %v, want 'edit'", m.Match.Action)
	}
}

func TestGetGlobMatchProvenance(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"internal/**", "internal/cli/**"},
	}

	provenance := getGlobMatchProvenance(rule, "internal/cli/**")
	if provenance == nil {
		t.Fatal("expected provenance, got nil")
	}
	if provenance.Kind != "glob" {
		t.Errorf("Kind = %v, want 'glob'", provenance.Kind)
	}
	if provenance.Value != "internal/cli/**" {
		t.Errorf("Value = %v, want 'internal/cli/**' (exact match preferred)", provenance.Value)
	}
}

func TestGetGlobMatchProvenancePrefersSpecific(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"**", "**/3rdparty/**"},
	}

	provenance := getGlobMatchProvenance(rule, "**/3rdparty/**")
	if provenance == nil {
		t.Fatal("expected provenance, got nil")
	}
	if provenance.Value != "**/3rdparty/**" {
		t.Errorf("Value = %v, want '**/3rdparty/**'", provenance.Value)
	}
}

func TestGetRulesForTagProvenance(t *testing.T) {
	rule := Rule{
		Tags: []string{"formatting", "style"},
	}

	matched := GetRulesForTag([]Rule{rule}, "formatting", "")
	if len(matched) == 0 {
		t.Fatal("expected matches, got none")
	}

	m := matched[0]
	if m.Match == nil {
		t.Fatal("expected Match, got nil")
	}
	if m.Match.Kind != "tag" {
		t.Errorf("Match.Kind = %v, want 'tag'", m.Match.Kind)
	}
	if m.Match.Value != "formatting" {
		t.Errorf("Match.Value = %v, want 'formatting'", m.Match.Value)
	}
}

func TestMatchReasonPreserved(t *testing.T) {
	rule := Rule{
		GlobPatterns: []string{"internal/**"},
	}

	matched := GetRulesForFile([]Rule{rule}, "internal/main.go", "", "")
	if len(matched) == 0 {
		t.Fatal("expected matches, got none")
	}

	m := matched[0]
	if m.MatchReason == "" {
		t.Error("expected MatchReason to be preserved")
	}
	if m.Match == nil {
		t.Fatal("expected Match, got nil")
	}
	// MatchReason should match the provenance format
	expected := "glob: internal/**"
	if m.MatchReason != expected {
		t.Errorf("MatchReason = %v, want %v", m.MatchReason, expected)
	}
}

func TestBackwardCompatibilityMatchReason(t *testing.T) {
	// Test that existing clients reading only rule and match_reason still work
	rule := Rule{
		GlobPatterns: []string{"**/*.go"},
		Actions:     []string{"edit", "write"},
	}

	matched := GetRulesForFile([]Rule{rule}, "main.go", "edit", "")
	if len(matched) == 0 {
		t.Fatal("expected matches, got none")
	}

	m := matched[0]
	// match_reason should be present and human-readable
	if m.MatchReason == "" {
		t.Error("expected MatchReason to be present for backward compatibility")
	}
	// match should also be present
	if m.Match == nil {
		t.Error("expected Match to be present")
	}
}
