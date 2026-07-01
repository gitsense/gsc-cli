/**
 * Component: Rules Test Command Tests
 * Block-UUID: e2f3a4b5-c6d7-8901-ef01-234567890123
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for gsc rules test command.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"testing"

	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	sessionspkg "github.com/gitsense/gsc-cli/internal/pi/sessions"
)

func TestGetLatestLeaf(t *testing.T) {
	path := "../../pi/sessions/testdata/linear.jsonl"
	leaf, err := sessionspkg.GetLatestLeaf(path)
	if err != nil {
		t.Fatalf("GetLatestLeaf() error = %v", err)
	}
	if leaf != "entry-9" {
		t.Errorf("GetLatestLeaf() = %v, want entry-9", leaf)
	}
}

func TestRuleTestWithMatchingRule(t *testing.T) {
	// Create a rule that matches foo.txt
	rule := rulespkg.Rule{
		ID:           "test_rule",
		Type:         rulespkg.RuleTypeDeclarative,
		Summary:      "Test rule for foo.txt",
		Instructions: []string{"Follow foo.txt conventions"},
		Actions:      []string{"read", "edit"},
		GlobPatterns: []string{"foo.txt"},
		Topic:        "testing",
		Importance:   "medium",
	}

	// Get file references from session
	path := "../../pi/sessions/testdata/linear.jsonl"
	filesResult, err := sessionspkg.ExtractFiles(path, "entry-9")
	if err != nil {
		t.Fatalf("ExtractFiles() error = %v", err)
	}

	// Verify we have the expected files
	if len(filesResult.Files) != 2 {
		t.Fatalf("len(Files) = %v, want 2", len(filesResult.Files))
	}

	// Test matching
	for _, fileRef := range filesResult.Files {
		if !containsAction(rule.Actions, fileRef.Op) {
			t.Errorf("action %s not in rule actions", fileRef.Op)
			continue
		}

		provenance := rulespkg.GetFileMatchProvenance(rule, fileRef.Path)
		if provenance == nil {
			t.Errorf("expected match for %s, got nil", fileRef.Path)
		} else {
			if provenance.Kind != "glob" {
				t.Errorf("match kind = %v, want glob", provenance.Kind)
			}
			if provenance.Value != "foo.txt" {
				t.Errorf("match value = %v, want foo.txt", provenance.Value)
			}
		}
	}
}

func TestRuleTestWithNonMatchingRule(t *testing.T) {
	// Create a rule that doesn't match foo.txt
	rule := rulespkg.Rule{
		ID:           "test_rule",
		Type:         rulespkg.RuleTypeDeclarative,
		Summary:      "Test rule for README",
		Instructions: []string{"Follow README conventions"},
		Actions:      []string{"edit"},
		GlobPatterns: []string{"README.md"},
		Topic:        "testing",
		Importance:   "medium",
	}

	// Get file references from session
	path := "../../pi/sessions/testdata/linear.jsonl"
	filesResult, err := sessionspkg.ExtractFiles(path, "entry-9")
	if err != nil {
		t.Fatalf("ExtractFiles() error = %v", err)
	}

	// Test non-matching
	for _, fileRef := range filesResult.Files {
		if !containsAction(rule.Actions, fileRef.Op) {
			// Expected - action mismatch
			continue
		}

		provenance := rulespkg.GetFileMatchProvenance(rule, fileRef.Path)
		if provenance != nil {
			t.Errorf("expected no match for %s, got %v", fileRef.Path, provenance)
		}
	}
}

func TestRuleTestToolTriggerError(t *testing.T) {
	// Create a tool-trigger rule
	rule := rulespkg.Rule{
		ID:   "test_trigger",
		Type: rulespkg.RuleTypeExecutable,
		Summary: "Test trigger",
	}

	// Verify it's detected as tool-trigger
	if !rule.IsExecutable() {
		t.Error("expected rule to be tool-trigger")
	}
}

func TestContainsAction(t *testing.T) {
	tests := []struct {
		actions []string
		action  string
		want    bool
	}{
		{[]string{"read", "edit"}, "read", true},
		{[]string{"read", "edit"}, "edit", true},
		{[]string{"read", "edit"}, "write", false},
		{[]string{}, "read", false},
		{nil, "read", false},
	}

	for _, tt := range tests {
		got := containsAction(tt.actions, tt.action)
		if got != tt.want {
			t.Errorf("containsAction(%v, %q) = %v, want %v", tt.actions, tt.action, got, tt.want)
		}
	}
}

func TestOpToToolName(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"read", "read"},
		{"edit", "edit"},
		{"write", "write"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := opToToolName(tt.op)
		if got != tt.want {
			t.Errorf("opToToolName(%q) = %q, want %q", tt.op, got, tt.want)
		}
	}
}
