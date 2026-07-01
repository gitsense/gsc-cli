/**
 * Component: Experts Guide Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for expert guide topic resolution and content.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package experts

import (
	"strings"
	"testing"
)

func TestResolveGuideTopicRuleAuthoring(t *testing.T) {
	tests := []struct {
		topic string
		want  string
		found bool
	}{
		{"rule-authoring", "GSC_RULE_AUTHORING_GUIDE.md", true},
		{"safe-rules", "GSC_RULE_AUTHORING_GUIDE.md", true},
		{"knowledge-authoring", "GSC_RULE_AUTHORING_GUIDE.md", true},
		{"unknown-topic", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			got, found := resolveGuideTopic(tt.topic)
			if found != tt.found {
				t.Errorf("resolveGuideTopic(%q) found = %v, want %v", tt.topic, found, tt.found)
			}
			if found && got != tt.want {
				t.Errorf("resolveGuideTopic(%q) = %q, want %q", tt.topic, got, tt.want)
			}
		})
	}
}

func TestGuideTopicsListIncludesRuleAuthoring(t *testing.T) {
	found := false
	for _, gt := range guideTopics {
		if gt.name == "rule-authoring" {
			found = true
			if !strings.Contains(gt.description, "scope") && !strings.Contains(gt.description, "target") {
				t.Errorf("rule-authoring guide description should mention scope/target, got: %s", gt.description)
			}
			break
		}
	}
	if !found {
		t.Error("guideTopics should include rule-authoring")
	}
}

func TestLoadRuleAuthoringGuide(t *testing.T) {
	// Test that the guide template loads successfully
	content, err := loadExpertTemplate("GSC_RULE_AUTHORING_GUIDE.md")
	if err != nil {
		t.Fatalf("loadExpertTemplate(GSC_RULE_AUTHORING_GUIDE.md) error: %v", err)
	}

	text := string(content)

	// Verify key sections exist
	requiredSections := []string{
		"Decision Framework",
		"Scope Selection",
		"Rule Safety Checklist",
		"Trigger Safety",
		"Post-Write Reporting",
		"--target",
		"--scope",
	}

	for _, section := range requiredSections {
		if !strings.Contains(text, section) {
			t.Errorf("Rule authoring guide missing section/content: %q", section)
		}
	}
}

func TestLoadRuleAuthoringGuideContainsSafetyGuidance(t *testing.T) {
	content, err := loadExpertTemplate("GSC_RULE_AUTHORING_GUIDE.md")
	if err != nil {
		t.Fatal(err)
	}

	text := string(content)

	// Verify trigger safety requirements
	triggerSafetyItems := []string{
		"Runtime is supported",
		"Entry path is relative",
		"traversal",
		"Side effects",
		"rollback",
		"Confirm",
	}

	for _, item := range triggerSafetyItems {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(item)) {
			t.Errorf("Rule authoring guide missing trigger safety item: %q", item)
		}
	}
}
