/**
 * Component: Rules Execution Tests
 * Block-UUID: d4e5f6a7-b8c9-0123-defa-345678901234
 * Parent-UUID: b2c3d4e5-f6a7-8901-bcde-f01234567890
 * Version: 1.0.0
 * Description: Tests for rule execution logic, trigger context building, and block determination.
 * Language: Go
 * Created-at: 2026-06-26T16:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"context"
	"testing"
)

func TestExecuteRulesPartitioning(t *testing.T) {
	input := &RulesInput{
		SchemaVersion: 1,
		Rules: []MatchedRuleInput{
			{
				ID:      "rule_1",
				Type:    "declarative",
				Summary: "Declarative rule",
			},
			{
				ID:      "rule_2",
				Type:    "executable",
				Summary: "Executable rule",
				Trigger: &TriggerConfig{
					Runtime: "node",
					Entry:   "test.mjs",
				},
			},
			{
				ID:      "rule_3",
				Type:    "declarative",
				Summary: "Another declarative",
			},
		},
	}

	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name: "pre_tool_use",
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		Session: V1SessionContext{
			ID:   "test-session",
			Path: "/tmp/test.jsonl",
			CWD:  "/tmp",
		},
	}

	result, err := ExecuteRules(context.Background(), input, execCtx, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 matched rules (2 declarative, 1 executable)
	if len(result.MatchedRules) != 3 {
		t.Errorf("expected 3 matched rules, got %d", len(result.MatchedRules))
	}

	// Declarative rules should come first
	if result.MatchedRules[0].Type != "declarative" {
		t.Errorf("first rule should be declarative, got %s", result.MatchedRules[0].Type)
	}
	if result.MatchedRules[1].Type != "declarative" {
		t.Errorf("second rule should be declarative, got %s", result.MatchedRules[1].Type)
	}
	if result.MatchedRules[2].Type != "executable" {
		t.Errorf("third rule should be executable, got %s", result.MatchedRules[2].Type)
	}
}

func TestExecuteRulesDeterministicOrdering(t *testing.T) {
	input := &RulesInput{
		SchemaVersion: 1,
		Rules: []MatchedRuleInput{
			{
				ID:       "rule_c",
				Type:     "executable",
				Summary:  "Rule C",
				Priority: 10,
				Trigger: &TriggerConfig{
					Runtime: "node",
					Entry:   "test.mjs",
				},
			},
			{
				ID:       "rule_a",
				Type:     "executable",
				Summary:  "Rule A",
				Priority: 10,
				Trigger: &TriggerConfig{
					Runtime: "node",
					Entry:   "test.mjs",
				},
			},
			{
				ID:       "rule_b",
				Type:     "executable",
				Summary:  "Rule B",
				Priority: 20,
				Trigger: &TriggerConfig{
					Runtime: "node",
					Entry:   "test.mjs",
				},
			},
		},
	}

	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name: "pre_tool_use",
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		Session: V1SessionContext{
			ID:   "test-session",
			Path: "/tmp/test.jsonl",
			CWD:  "/tmp",
		},
	}

	result, err := ExecuteRules(context.Background(), input, execCtx, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Executable rules should be sorted by priority desc, then ID asc
	// Expected order: rule_b (priority 20), rule_a (priority 10), rule_c (priority 10)
	if len(result.MatchedRules) != 3 {
		t.Fatalf("expected 3 matched rules, got %d", len(result.MatchedRules))
	}

	// All should be executable
	for i, rule := range result.MatchedRules {
		if rule.Type != "executable" {
			t.Errorf("rule %d should be executable, got %s", i, rule.Type)
		}
	}

	// Check ordering: rule_b first (highest priority)
	if result.MatchedRules[0].RuleID != "rule_b" {
		t.Errorf("first rule should be rule_b (priority 20), got %s", result.MatchedRules[0].RuleID)
	}

	// Then rule_a and rule_c (same priority, alphabetical)
	if result.MatchedRules[1].RuleID != "rule_a" {
		t.Errorf("second rule should be rule_a (alphabetical), got %s", result.MatchedRules[1].RuleID)
	}
	if result.MatchedRules[2].RuleID != "rule_c" {
		t.Errorf("third rule should be rule_c (alphabetical), got %s", result.MatchedRules[2].RuleID)
	}
}

func TestExecuteRulesCapabilitiesEnforcement(t *testing.T) {
	input := &RulesInput{
		SchemaVersion: 1,
		Rules: []MatchedRuleInput{
			{
				ID:      "rule_1",
				Type:    "declarative",
				Summary: "Declarative rule",
				Instructions: []string{"Do something"},
			},
		},
	}

	// Context with canBlock=false
	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name: "pre_tool_use",
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: false,
		},
		Session: V1SessionContext{
			ID:   "test-session",
			Path: "/tmp/test.jsonl",
			CWD:  "/tmp",
		},
	}

	result, err := ExecuteRules(context.Background(), input, execCtx, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not block even though declarative rules exist
	if result.Block {
		t.Error("should not block when canBlock=false")
	}

	// Should have notice about block being ignored
	found := false
	for _, notice := range result.Notices {
		if notice == "Block ignored: canBlock=false in context" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected notice about block being ignored")
	}
}

func TestExecuteRulesPreToolUseBlocksOnDeclarative(t *testing.T) {
	input := &RulesInput{
		SchemaVersion: 1,
		Rules: []MatchedRuleInput{
			{
				ID:      "rule_1",
				Type:    "declarative",
				Summary: "Safety rule",
				Instructions: []string{"Check safety"},
				Match: &MatchInfo{
					Kind:  "command",
					Value: "rm -rf",
				},
			},
		},
	}

	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name:    "pre_tool_use",
			Runtime: "pi",
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		Session: V1SessionContext{
			ID:   "test-session",
			Path: "/tmp/test.jsonl",
			CWD:  "/tmp",
		},
		Payload: V1ExecutionContextPayload{
			ToolCall: &V1ToolCallContext{
				ID:       "call-1",
				ToolName: "bash",
				Action:   "bash",
				Command:  strPtr("rm -rf /tmp/test"),
			},
		},
	}

	result, err := ExecuteRules(context.Background(), input, execCtx, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should block for pre_tool_use with declarative rules
	if !result.Block {
		t.Error("should block for pre_tool_use with declarative rules")
	}

	// Should have a reason
	if result.Reason == "" {
		t.Error("expected block reason")
	}
}

func TestExecuteRulesUserPromptSubmitAdvisory(t *testing.T) {
	input := &RulesInput{
		SchemaVersion: 1,
		Rules: []MatchedRuleInput{
			{
				ID:      "rule_1",
				Type:    "declarative",
				Summary: "Prompt rule",
				Instructions: []string{"Check prompt"},
			},
		},
	}

	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name: "user_prompt_submit",
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		Session: V1SessionContext{
			ID:   "test-session",
			Path: "/tmp/test.jsonl",
			CWD:  "/tmp",
		},
	}

	result, err := ExecuteRules(context.Background(), input, execCtx, ExecuteOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not block for user_prompt_submit with declarative rules (advisory only)
	if result.Block {
		t.Error("should not block for user_prompt_submit with declarative rules")
	}
}

func TestExecuteRulesBuildTriggerContext(t *testing.T) {
	rule := MatchedRuleInput{
		ID:      "rule_1",
		Type:    "executable",
		Event:   "pre_tool_use",
		Summary: "Test rule",
		Trigger: &TriggerConfig{
			Runtime: "node",
			Entry:   "test.mjs",
		},
		RuleHash:    "sha256:abc123",
		TriggerHash: "sha256:def456",
	}

	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name:    "pre_tool_use",
			Runtime: "pi",
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		Session: V1SessionContext{
			ID:   "test-session",
			Path: "/tmp/test.jsonl",
			CWD:  "/tmp",
		},
		Conversation: V1ConversationContext{
			LeafID:     "entry-1",
			MessageIDs: []string{"entry-1"},
		},
		Payload: V1ExecutionContextPayload{
			ToolCall: &V1ToolCallContext{
				ID:       "call-1",
				ToolName: "bash",
				Action:   "bash",
				Command:  strPtr("ls -la"),
			},
		},
		Repo: &V1RepoContext{
			Root: "/path/to/repo",
		},
	}

	triggerCtx := buildTriggerContext(rule, execCtx)

	// Verify context
	if triggerCtx.Version != "1" {
		t.Errorf("expected version 1, got %s", triggerCtx.Version)
	}
	if triggerCtx.Event.Name != "pre_tool_use" {
		t.Errorf("expected event pre_tool_use, got %s", triggerCtx.Event.Name)
	}
	if triggerCtx.Event.Runtime != "pi" {
		t.Errorf("expected runtime pi, got %s", triggerCtx.Event.Runtime)
	}
	if triggerCtx.Session.ID != "test-session" {
		t.Errorf("expected session ID test-session, got %s", triggerCtx.Session.ID)
	}
	if triggerCtx.Rule.ID != "rule_1" {
		t.Errorf("expected rule ID rule_1, got %s", triggerCtx.Rule.ID)
	}
	if triggerCtx.Rule.RuleHash != "sha256:abc123" {
		t.Errorf("expected ruleHash sha256:abc123, got %s", triggerCtx.Rule.RuleHash)
	}
	if triggerCtx.Rule.TriggerHash != "sha256:def456" {
		t.Errorf("expected triggerHash sha256:def456, got %s", triggerCtx.Rule.TriggerHash)
	}
	if triggerCtx.Repo == nil {
		t.Error("expected repo context")
	} else if triggerCtx.Repo.Root != "/path/to/repo" {
		t.Errorf("expected repo root /path/to/repo, got %s", triggerCtx.Repo.Root)
	}
}

func TestExecuteRulesRenderInstructionTemplate(t *testing.T) {
	execCtx := &V1ExecutionContext{
		Repo: &V1RepoContext{
			Root: "/path/to/repo",
		},
		Payload: V1ExecutionContextPayload{
			ToolCall: &V1ToolCallContext{
				Action:  "bash",
				Command: strPtr("rm -rf /tmp"),
			},
		},
	}

	rule := MatchedRuleInput{
		ID: "rule_1",
		Match: &MatchInfo{
			Kind:  "command",
			Value: "rm -rf",
		},
	}

	tests := []struct {
		name       string
		template   string
		expected   string
	}{
		{
			name:     "file variable",
			template: "File: {{file}}",
			expected: "File: ",
		},
		{
			name:     "action variable",
			template: "Action: {{action}}",
			expected: "Action: bash",
		},
		{
			name:     "match_kind variable",
			template: "Match: {{match_kind}}: {{match_value}}",
			expected: "Match: command: rm -rf",
		},
		{
			name:     "repo_root variable",
			template: "Repo: {{repo_root}}",
			expected: "Repo: /path/to/repo",
		},
		{
			name:     "rule_id variable",
			template: "Rule: {{rule_id}}",
			expected: "Rule: rule_1",
		},
		{
			name:     "unknown variable",
			template: "Unknown: {{unknown}}",
			expected: "Unknown: {{unknown}}",
		},
		{
			name:     "multiple variables",
			template: "{{action}} at {{repo_root}}",
			expected: "bash at /path/to/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderInstructionTemplate(tt.template, execCtx, rule)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExecuteRulesBuildBlockReason(t *testing.T) {
	execCtx := &V1ExecutionContext{
		Event: V1EventContext{
			Name:    "pre_tool_use",
			Runtime: "pi",
		},
		Payload: V1ExecutionContextPayload{
			ToolCall: &V1ToolCallContext{
				ToolName: "bash",
				Action:   "bash",
				Command:  strPtr("rm -rf /tmp"),
			},
		},
	}

	declarativeRules := []MatchedRuleInput{
		{
			ID:      "rule_1",
			Summary: "Safety check",
			Instructions: []string{"Check safety"},
			Match: &MatchInfo{
				Kind:  "command",
				Value: "rm -rf",
			},
		},
	}

	executableRules := []MatchedRuleInput{
		{
			ID:      "rule_2",
			Summary: "Trigger check",
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test.mjs",
			},
			Match: &MatchInfo{
				Kind:  "command",
				Value: "rm -rf",
			},
		},
	}

	result := &ExecutionResult{
		TriggerResults: []TriggerResultInfo{
			{
				RuleID:  "rule_2",
				Matched: true,
				Block:   true,
				Message: "Blocked by trigger",
			},
		},
	}

	reason := buildBlockReason(execCtx, declarativeRules, executableRules, result)

	// Verify reason contains expected content
	if reason == "" {
		t.Error("expected non-empty reason")
	}
	if !containsString(reason, "GitSense matched repository rules") {
		t.Error("expected reason to contain 'GitSense matched repository rules'")
	}
	if !containsString(reason, "Event: pre_tool_use") {
		t.Error("expected reason to contain event info")
	}
	if !containsString(reason, "Safety check [instruction]") {
		t.Error("expected reason to contain declarative rule")
	}
	if !containsString(reason, "Trigger check [tool-trigger]") {
		t.Error("expected reason to contain executable rule")
	}
}

func TestExecuteRulesNilInput(t *testing.T) {
	execCtx := &V1ExecutionContext{
		Version: "1",
		Event: V1EventContext{
			Name: "pre_tool_use",
		},
	}

	_, err := ExecuteRules(context.Background(), nil, execCtx, ExecuteOptions{})
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestExecuteRulesNilContext(t *testing.T) {
	input := &RulesInput{
		SchemaVersion: 1,
		Rules:         []MatchedRuleInput{},
	}

	_, err := ExecuteRules(context.Background(), input, nil, ExecuteOptions{})
	if err == nil {
		t.Error("expected error for nil context")
	}
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
