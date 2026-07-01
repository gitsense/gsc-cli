/**
 * Component: Rules Normalization Tests
 * Block-UUID: e5f6a7b8-c9d0-1234-ef01-234567890123
 * Parent-UUID: d4e5f6a7-b8c9-0123-defa-234567890123
 * Version: 1.0.0
 * Description: Tests for slug normalization and tag matching functions.
 * Language: Go
 * Created-at: 2026-06-26T15:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"safety", "safety"},
		{"Safety", "safety"},
		{"My Tag", "my-tag"},
		{"my-tag", "my-tag"},
		{"my_tag", "my-tag"},
		{"  spaces  ", "spaces"},
		{"special!@#chars", "special-chars"},
		{"", ""},
		{"---", ""},
		{"a-b-c", "a-b-c"},
		{"UPPER CASE", "upper-case"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMatchesTag(t *testing.T) {
	tests := []struct {
		name      string
		ruleTag   string
		filterTag string
		expected  bool
	}{
		// Exact match
		{
			name:      "exact match",
			ruleTag:   "safety",
			filterTag: "safety",
			expected:  true,
		},
		// Case insensitive
		{
			name:      "case insensitive - filter uppercase",
			ruleTag:   "safety",
			filterTag: "Safety",
			expected:  true,
		},
		{
			name:      "case insensitive - rule uppercase",
			ruleTag:   "Safety",
			filterTag: "safety",
			expected:  true,
		},
		{
			name:      "case insensitive - both uppercase",
			ruleTag:   "SAFETY",
			filterTag: "SAFETY",
			expected:  true,
		},
		// Substring match
		{
			name:      "substring match - prefix",
			ruleTag:   "safety",
			filterTag: "safe",
			expected:  true,
		},
		{
			name:      "substring match - suffix",
			ruleTag:   "safety",
			filterTag: "ety",
			expected:  true,
		},
		{
			name:      "substring match - middle",
			ruleTag:   "safety",
			filterTag: "afe",
			expected:  true,
		},
		// Special characters
		{
			name:      "special chars - space in rule",
			ruleTag:   "My Tag",
			filterTag: "my-tag",
			expected:  true,
		},
		{
			name:      "special chars - space in filter",
			ruleTag:   "my-tag",
			filterTag: "My Tag",
			expected:  true,
		},
		{
			name:      "special chars - underscore in rule",
			ruleTag:   "my_tag",
			filterTag: "my-tag",
			expected:  true,
		},
		{
			name:      "special chars - mixed",
			ruleTag:   "My Special Tag!",
			filterTag: "my-special-tag",
			expected:  true,
		},
		// No match
		{
			name:      "no match - different tag",
			ruleTag:   "safety",
			filterTag: "security",
			expected:  false,
		},
		{
			name:      "no match - filter is longer",
			ruleTag:   "safe",
			filterTag: "safety",
			expected:  false,
		},
		// Empty filter
		{
			name:      "empty filter",
			ruleTag:   "safety",
			filterTag: "",
			expected:  false,
		},
		{
			name:      "empty both",
			ruleTag:   "",
			filterTag: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesTag(tt.ruleTag, tt.filterTag)
			if result != tt.expected {
				t.Errorf("MatchesTag(%q, %q) = %v, want %v", tt.ruleTag, tt.filterTag, result, tt.expected)
			}
		})
	}
}
