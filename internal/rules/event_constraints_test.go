/**
 * Component: Event Constraints Tests
 * Block-UUID: test-event-constraints
 * Parent-UUID: N/A
 * Version: 3.0.0
 * Description: Tests for lifecycle event-type constraints validation and new actions.
 * Language: Go
 * Created-at: 2026-06-24T12:00:00Z
 * Updated-at: 2026-06-25T12:00:00Z
 */


package rules

import (
	"strings"
	"testing"
)

func TestEventConstraintsUserPromptSubmit(t *testing.T) {
	t.Run("instruction rule with prompt action accepted", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test rule",
			Topic:        "agent-lifecycle-rules",
			Event:        EventUserPromptSubmit,
			Instructions: []string{"Do something"},
			Actions:      []string{"prompt"},
			PromptFilter: ".*",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for instruction rule with prompt action, got: %v", result.Errors)
		}
	})

	t.Run("instruction rule without prompt action rejected", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test rule",
			Topic:        "agent-lifecycle-rules",
			Event:        EventUserPromptSubmit,
			Instructions: []string{"Do something"},
			Actions:      []string{"edit"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for instruction rule without prompt action")
		}

		found := false
		for _, err := range result.Errors {
			if err == "user_prompt_submit requires executable triggers or declarative rules with prompt action" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about user_prompt_submit requiring executable triggers or prompt action, got: %v", result.Errors)
		}
	})

	t.Run("tool-trigger rule accepted", func(t *testing.T) {
		rule := Rule{
			Summary: "Test trigger rule",
			Topic:   "agent-lifecycle-rules",
			Event:   EventUserPromptSubmit,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Check something",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with the event constraint error
		for _, err := range result.Errors {
			if err == "user_prompt_submit only supports executable triggers, not deterministic instructions" {
				t.Fatalf("should not have event constraint error, got: %v", result.Errors)
			}
		}
	})

	t.Run("invalid action for user_prompt_submit", func(t *testing.T) {
		rule := Rule{
			Summary: "Test invalid action",
			Topic:   "agent-lifecycle-rules",
			Event:   EventUserPromptSubmit,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Check something",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		// Add an invalid action for user_prompt_submit
		rule.Actions = []string{"bash"}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == `action "bash" is not valid for event user_prompt_submit; valid actions are: prompt` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid action for user_prompt_submit, got: %v", result.Errors)
		}
	})
}

func TestEventConstraintsBeforeAgentStart(t *testing.T) {
	t.Run("instruction rule accepted", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test rule",
			Topic:        "agent-lifecycle-rules",
			Event:        EventBeforeAgentStart,
			Instructions: []string{"Inject context"},
			Actions:      []string{"prompt"},
			PromptFilter: ".*",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for instruction rule with before_agent_start, got: %v", result.Errors)
		}
	})

	t.Run("tool-trigger rule accepted", func(t *testing.T) {
		rule := Rule{
			Summary: "Test trigger rule",
			Topic:   "agent-lifecycle-rules",
			Event:   EventBeforeAgentStart,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Check something",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with the event constraint error
		for _, err := range result.Errors {
			if err == "before_agent_start only supports deterministic instructions, not executable triggers" {
				t.Fatalf("should not have event constraint error, got: %v", result.Errors)
			}
		}
	})

	t.Run("invalid action for before_agent_start", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test invalid action",
			Topic:        "agent-lifecycle-rules",
			Event:        EventBeforeAgentStart,
			Instructions: []string{"Inject context"},
			Actions:      []string{"edit"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == `action "edit" is not valid for event before_agent_start; valid actions are: prompt` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid action for before_agent_start, got: %v", result.Errors)
		}
	})
}

func TestEventConstraintsAgentEnd(t *testing.T) {
	t.Run("instruction rule rejected", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test rule",
			Topic:        "agent-lifecycle-rules",
			Event:        EventAgentEnd,
			Instructions: []string{"Do something"},
			Actions:      []string{"agent_end"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for instruction rule with agent_end")
		}

		found := false
		for _, err := range result.Errors {
			if err == "agent_end only supports executable triggers, not deterministic instructions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about agent_end only supporting triggers, got: %v", result.Errors)
		}
	})

	t.Run("tool-trigger rule accepted", func(t *testing.T) {
		rule := Rule{
			Summary: "Test trigger rule",
			Topic:   "agent-lifecycle-rules",
			Event:   EventAgentEnd,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Check something",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with the event constraint error
		for _, err := range result.Errors {
			if err == "agent_end only supports executable triggers, not deterministic instructions" {
				t.Fatalf("should not have event constraint error, got: %v", result.Errors)
			}
		}
	})

	t.Run("invalid action for agent_end", func(t *testing.T) {
		rule := Rule{
			Summary: "Test invalid action",
			Topic:   "agent-lifecycle-rules",
			Event:   EventAgentEnd,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Check something",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		// Add an invalid action for agent_end
		rule.Actions = []string{"bash"}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == `action "bash" is not valid for event agent_end; valid actions are: agent_end` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid action for agent_end, got: %v", result.Errors)
		}
	})
}

func TestEventConstraintsToolEvents(t *testing.T) {
	t.Run("pre_tool_use accepts tool actions", func(t *testing.T) {
		// Test instruction rule with bash action
		instructionRule := Rule{
			Summary:        "Test instruction rule",
			Topic:          "agent-lifecycle-rules",
			Event:          EventPreToolUse,
			Instructions:   []string{"Do something"},
			Actions:        []string{"bash"},
			CommandFilter:  ".*",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(instructionRule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for instruction rule with pre_tool_use and bash action, got: %v", result.Errors)
		}

		// Test tool-trigger rule with read action
		triggerRule := Rule{
			Summary: "Test trigger rule",
			Topic:   "agent-lifecycle-rules",
			Event:   EventPreToolUse,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Check something",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result = ValidateAndNormalize(triggerRule)
		// Check that it doesn't fail with event constraint errors
		for _, err := range result.Errors {
			if err == "pre_tool_use only supports executable triggers, not deterministic instructions" ||
				err == "pre_tool_use only supports deterministic instructions, not executable triggers" {
				t.Fatalf("should not have event constraint error for pre_tool_use, got: %v", result.Errors)
			}
		}
	})

	t.Run("post_tool_use accepts tool actions", func(t *testing.T) {
		rule := Rule{
			Summary:        "Test instruction rule",
			Topic:          "agent-lifecycle-rules",
			Event:          EventPostToolUse,
			Instructions:   []string{"Do something"},
			Actions:        []string{"edit", "bash"},
			CommandFilter:  ".*",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for instruction rule with post_tool_use, got: %v", result.Errors)
		}
	})

	t.Run("invalid action for pre_tool_use", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test invalid action",
			Topic:        "agent-lifecycle-rules",
			Event:        EventPreToolUse,
			Instructions: []string{"Do something"},
			Actions:      []string{"prompt"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == `action "prompt" is not valid for event pre_tool_use; valid actions are: read, write, edit, bash, tool, mcp_tool` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid action for pre_tool_use, got: %v", result.Errors)
		}
	})
}

func TestEventConstraintsSessionEvents(t *testing.T) {
	t.Run("session_start rejects actions", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test rule",
			Topic:        "agent-lifecycle-rules",
			Event:        EventSessionStart,
			Instructions: []string{"Do something"},
			Actions:      []string{"read"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == "session_start is notification only and does not support actions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about session_start being notification only, got: %v", result.Errors)
		}
	})

	t.Run("session_end rejects actions", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test rule",
			Topic:        "agent-lifecycle-rules",
			Event:        EventSessionEnd,
			Instructions: []string{"Do something"},
			Actions:      []string{"read"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == "session_end is notification only and does not support actions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about session_end being notification only, got: %v", result.Errors)
		}
	})
}

func TestEventConstraintsAgentStart(t *testing.T) {
	t.Run("agent_start rejects actions", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test agent_start",
			Topic:        "agent-lifecycle-rules",
			Event:        EventAgentStart,
			Instructions: []string{"Do something"},
			Actions:      []string{"read"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for agent_start with actions")
		}

		found := false
		for _, err := range result.Errors {
			if err == "agent_start is notification only and does not support actions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about agent_start being notification only, got: %v", result.Errors)
		}
	})

	t.Run("agent_start rejects declarative rules", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test agent_start",
			Topic:        "agent-lifecycle-rules",
			Event:        EventAgentStart,
			Instructions: []string{"Do something"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for agent_start with declarative rule")
		}

		found := false
		for _, err := range result.Errors {
			if err == "agent_start only supports executable triggers for side effects" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about agent_start only supporting executable triggers, got: %v", result.Errors)
		}
	})

	t.Run("agent_start accepts executable triggers", func(t *testing.T) {
		rule := Rule{
			Summary: "Test agent_start trigger",
			Topic:   "agent-lifecycle-rules",
			Event:   EventAgentStart,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Agent started",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with event constraint errors
		for _, err := range result.Errors {
			if err == "agent_start is notification only and does not support actions" ||
				err == "agent_start only supports executable triggers for side effects" {
				t.Fatalf("should not have event constraint error for agent_start, got: %v", result.Errors)
			}
		}
	})
}

func TestEventConstraintsContextEvents(t *testing.T) {
	t.Run("context rejects actions", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test context",
			Topic:        "agent-lifecycle-rules",
			Event:        EventContext,
			Instructions: []string{"Inject knowledge"},
			Actions:      []string{"read"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for context with actions")
		}

		found := false
		for _, err := range result.Errors {
			if err == "context is message injection only and does not support actions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about context being message injection only, got: %v", result.Errors)
		}
	})

	t.Run("context rejects declarative rules", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test context",
			Topic:        "agent-lifecycle-rules",
			Event:        EventContext,
			Instructions: []string{"Inject knowledge"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for context with declarative rule")
		}

		found := false
		for _, err := range result.Errors {
			if err == "context only supports executable triggers for knowledge injection" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about context only supporting executable triggers, got: %v", result.Errors)
		}
	})

	t.Run("context accepts executable triggers", func(t *testing.T) {
		rule := Rule{
			Summary: "Test context trigger",
			Topic:   "agent-lifecycle-rules",
			Event:   EventContext,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Inject knowledge",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with event constraint errors
		for _, err := range result.Errors {
			if err == "context is message injection only and does not support actions" ||
				err == "context only supports executable triggers for knowledge injection" {
				t.Fatalf("should not have event constraint error for context, got: %v", result.Errors)
			}
		}
	})
}

func TestEventConstraintsSessionBeforeCompact(t *testing.T) {
	t.Run("session_before_compact rejects actions", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test session_before_compact",
			Topic:        "agent-lifecycle-rules",
			Event:        EventSessionBeforeCompact,
			Instructions: []string{"Preserve context"},
			Actions:      []string{"read"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for session_before_compact with actions")
		}

		found := false
		for _, err := range result.Errors {
			if err == "session_before_compact is cancel/customize only and does not support actions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about session_before_compact being cancel/customize only, got: %v", result.Errors)
		}
	})

	t.Run("session_before_compact rejects declarative rules", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test session_before_compact",
			Topic:        "agent-lifecycle-rules",
			Event:        EventSessionBeforeCompact,
			Instructions: []string{"Preserve context"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for session_before_compact with declarative rule")
		}

		found := false
		for _, err := range result.Errors {
			if err == "session_before_compact only supports executable triggers for context preservation" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about session_before_compact only supporting executable triggers, got: %v", result.Errors)
		}
	})

	t.Run("session_before_compact accepts executable triggers", func(t *testing.T) {
		rule := Rule{
			Summary: "Test session_before_compact trigger",
			Topic:   "agent-lifecycle-rules",
			Event:   EventSessionBeforeCompact,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Preserve context",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with event constraint errors
		for _, err := range result.Errors {
			if err == "session_before_compact is cancel/customize only and does not support actions" ||
				err == "session_before_compact only supports executable triggers for context preservation" {
				t.Fatalf("should not have event constraint error for session_before_compact, got: %v", result.Errors)
			}
		}
	})
}

func TestEventConstraintsSessionCompact(t *testing.T) {
	t.Run("session_compact rejects actions", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test session_compact",
			Topic:        "agent-lifecycle-rules",
			Event:        EventSessionCompact,
			Instructions: []string{"Refresh context"},
			Actions:      []string{"read"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for session_compact with actions")
		}

		found := false
		for _, err := range result.Errors {
			if err == "session_compact is notification only and does not support actions" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about session_compact being notification only, got: %v", result.Errors)
		}
	})

	t.Run("session_compact rejects declarative rules", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test session_compact",
			Topic:        "agent-lifecycle-rules",
			Event:        EventSessionCompact,
			Instructions: []string{"Refresh context"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for session_compact with declarative rule")
		}

		found := false
		for _, err := range result.Errors {
			if err == "session_compact only supports executable triggers for context refresh" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about session_compact only supporting executable triggers, got: %v", result.Errors)
		}
	})

	t.Run("session_compact accepts executable triggers", func(t *testing.T) {
		rule := Rule{
			Summary: "Test session_compact trigger",
			Topic:   "agent-lifecycle-rules",
			Event:   EventSessionCompact,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Refresh context",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with event constraint errors
		for _, err := range result.Errors {
			if err == "session_compact is notification only and does not support actions" ||
				err == "session_compact only supports executable triggers for context refresh" {
				t.Fatalf("should not have event constraint error for session_compact, got: %v", result.Errors)
			}
		}
	})
}

func TestNewActions(t *testing.T) {
	t.Run("bash action is valid", func(t *testing.T) {
		rule := Rule{
			Summary:        "Test bash action",
			Topic:          "agent-lifecycle-rules",
			Instructions:   []string{"Handle bash commands"},
			Actions:        []string{"bash"},
			CommandFilter:  ".*",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for bash action, got: %v", result.Errors)
		}
	})

	t.Run("tool action is valid", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test tool action",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Handle tool execution"},
			Actions:      []string{"tool"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for tool action, got: %v", result.Errors)
		}
	})

	t.Run("mcp_tool action is valid", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test mcp_tool action",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Handle MCP tools"},
			Actions:      []string{"mcp_tool"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for mcp_tool action, got: %v", result.Errors)
		}
	})

	t.Run("prompt action is valid", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test prompt action",
			Topic:        "agent-lifecycle-rules",
			Event:        EventBeforeAgentStart,
			Instructions: []string{"Handle user prompt"},
			Actions:      []string{"prompt"},
			PromptFilter: ".*",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for prompt action, got: %v", result.Errors)
		}
	})

	t.Run("agent_end action is valid", func(t *testing.T) {
		rule := Rule{
			Summary: "Test agent_end action",
			Topic:   "agent-lifecycle-rules",
			Event:   EventAgentEnd,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Handle agent end",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Note: This will fail with "trigger file does not exist" which is expected
		// We're specifically checking that it doesn't fail with the action constraint error
		for _, err := range result.Errors {
			if err == `action "agent_end" is not valid for event agent_end; valid actions are: agent_end` {
				t.Fatalf("should not have action constraint error, got: %v", result.Errors)
			}
		}
	})

	t.Run("old command action is rejected", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test old command action",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Handle commands"},
			Actions:      []string{"command"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for old command action")
		}

		found := false
		for _, err := range result.Errors {
			if err == `action "command" must be one of: read, write, edit, bash, tool, mcp_tool, prompt, agent_end` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about old command action, got: %v", result.Errors)
		}
	})

	t.Run("old exec action is rejected", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test old exec action",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Handle execution"},
			Actions:      []string{"exec"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for old exec action")
		}

		found := false
		for _, err := range result.Errors {
			if err == `action "exec" must be one of: read, write, edit, bash, tool, mcp_tool, prompt, agent_end` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about old exec action, got: %v", result.Errors)
		}
	})

	t.Run("invalid action is rejected", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test invalid action",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Do something"},
			Actions:      []string{"invalid_action"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for invalid action")
		}

		found := false
		for _, err := range result.Errors {
			if err == `action "invalid_action" must be one of: read, write, edit, bash, tool, mcp_tool, prompt, agent_end` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid action, got: %v", result.Errors)
		}
	})
}

func TestEventActionCombinations(t *testing.T) {
	t.Run("user_prompt_submit with prompt action", func(t *testing.T) {
		rule := Rule{
			Summary: "Test prompt event with prompt action",
			Topic:   "agent-lifecycle-rules",
			Event:   EventUserPromptSubmit,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			Actions: []string{"prompt"},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Handle prompt",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Should not have event constraint errors
		for _, err := range result.Errors {
			if err == "user_prompt_submit only supports executable triggers, not deterministic instructions" {
				t.Fatalf("should not have event constraint error, got: %v", result.Errors)
			}
			if err == `action "prompt" is not valid for event user_prompt_submit; valid actions are: prompt` {
				t.Fatalf("should not have action constraint error, got: %v", result.Errors)
			}
		}
	})

	t.Run("before_agent_start with prompt action", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test before_agent_start with prompt action",
			Topic:        "agent-lifecycle-rules",
			Event:        EventBeforeAgentStart,
			Instructions: []string{"Inject context"},
			Actions:      []string{"prompt"},
			PromptFilter: ".*",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for before_agent_start with prompt action, got: %v", result.Errors)
		}
	})

	t.Run("pre_tool_use with bash action", func(t *testing.T) {
		rule := Rule{
			Summary:        "Test pre_tool_use with bash action",
			Topic:          "agent-lifecycle-rules",
			Event:          EventPreToolUse,
			Instructions:   []string{"Handle bash commands"},
			Actions:        []string{"bash"},
			CommandFilter:  ".*",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for pre_tool_use with bash action, got: %v", result.Errors)
		}
	})

	t.Run("pre_tool_use with mcp_tool action", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test pre_tool_use with mcp_tool action",
			Topic:        "agent-lifecycle-rules",
			Event:        EventPreToolUse,
			Instructions: []string{"Handle MCP tools"},
			Actions:      []string{"mcp_tool"},
			ToolFilter:   "github.*",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for pre_tool_use with mcp_tool action, got: %v", result.Errors)
		}
	})

	t.Run("agent_end with agent_end action", func(t *testing.T) {
		rule := Rule{
			Summary: "Test agent_end with agent_end action",
			Topic:   "agent-lifecycle-rules",
			Event:   EventAgentEnd,
			Type:    RuleTypeExecutable,
			Trigger: &TriggerConfig{
				Runtime: "node",
				Entry:   "test-trigger.mjs",
			},
			Actions: []string{"agent_end"},
			InstrCfg: &InstructionConfig{
				Mode: "inline",
				Text: "Handle agent end",
			},
			Frequency: &FrequencyConfig{
				Mode: FrequencyAlways,
			},
		}

		result := ValidateAndNormalize(rule)
		// Should not have event constraint errors
		for _, err := range result.Errors {
			if err == "agent_end only supports executable triggers, not deterministic instructions" {
				t.Fatalf("should not have event constraint error, got: %v", result.Errors)
			}
			if err == `action "agent_end" is not valid for event agent_end; valid actions are: agent_end` {
				t.Fatalf("should not have action constraint error, got: %v", result.Errors)
			}
		}
	})
}

func TestBashActionRequiresMatches(t *testing.T) {
	t.Run("bash action without matches should fail", func(t *testing.T) {
		rule := Rule{
			Summary:      "Block rm -rf",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Do not run rm -rf"},
			Actions:      []string{"bash"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for bash action without --matches")
		}

		found := false
		for _, err := range result.Errors {
			if err == "bash action requires --matches flag (use .* to match all commands)" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about bash action requiring --matches, got: %v", result.Errors)
		}
	})

	t.Run("bash action with matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:        "Block rm -rf",
			Topic:          "agent-lifecycle-rules",
			Instructions:   []string{"Do not run rm -rf"},
			Actions:        []string{"bash"},
			CommandFilter:  "rm -rf",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for bash action with --matches, got: %v", result.Errors)
		}
	})

	t.Run("bash action with wildcard matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:        "Log all commands",
			Topic:          "agent-lifecycle-rules",
			Instructions:   []string{"Log command for audit"},
			Actions:        []string{"bash"},
			CommandFilter:  ".*",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for bash action with .* matches, got: %v", result.Errors)
		}
	})

	t.Run("bash action with case-insensitive matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:                 "Block rm -rf",
			Topic:                   "agent-lifecycle-rules",
			Instructions:            []string{"Do not run rm -rf"},
			Actions:                 []string{"bash"},
			CommandFilter:           "RM -RF",
			CommandFilterIgnoreCase: true,
			GlobPatterns:            []string{"**/*.go"},
			Importance:              "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for bash action with case-insensitive --matches, got: %v", result.Errors)
		}
	})

	t.Run("non-bash action without matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:      "Edit files",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Edit files carefully"},
			Actions:      []string{"edit"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for non-bash action without --matches, got: %v", result.Errors)
		}
	})

	t.Run("mixed actions with bash but no matches should fail", func(t *testing.T) {
		rule := Rule{
			Summary:      "Edit and run commands",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Edit files and run commands"},
			Actions:      []string{"edit", "bash"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for mixed actions with bash but no --matches")
		}

		found := false
		for _, err := range result.Errors {
			if err == "bash action requires --matches flag (use .* to match all commands)" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about bash action requiring --matches, got: %v", result.Errors)
		}
	})
}

func TestPromptActionRequiresMatches(t *testing.T) {
	t.Run("prompt action without matches should fail", func(t *testing.T) {
		rule := Rule{
			Summary:      "Block AWS keys",
			Topic:        "agent-lifecycle-rules",
			Event:        EventUserPromptSubmit,
			Instructions: []string{"Do not share AWS keys"},
			Actions:      []string{"prompt"},
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if result.Valid() {
			t.Fatal("expected validation to fail for prompt action without --matches")
		}

		found := false
		for _, err := range result.Errors {
			if err == "prompt action requires --matches flag (use .* to match all prompts)" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about prompt action requiring --matches, got: %v", result.Errors)
		}
	})

	t.Run("prompt action with matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:        "Block AWS keys",
			Topic:          "agent-lifecycle-rules",
			Event:          EventUserPromptSubmit,
			Instructions:   []string{"Do not share AWS keys"},
			Actions:        []string{"prompt"},
			PromptFilter:   "AKIA[0-9A-Z]{16}",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for prompt action with --matches, got: %v", result.Errors)
		}
	})

	t.Run("prompt action with wildcard matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:        "Log all prompts",
			Topic:          "agent-lifecycle-rules",
			Event:          EventUserPromptSubmit,
			Instructions:   []string{"Log prompt for audit"},
			Actions:        []string{"prompt"},
			PromptFilter:   ".*",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for prompt action with .* matches, got: %v", result.Errors)
		}
	})

	t.Run("prompt action with case-insensitive matches should pass", func(t *testing.T) {
		rule := Rule{
			Summary:                 "Block production credentials",
			Topic:                   "agent-lifecycle-rules",
			Event:                   EventUserPromptSubmit,
			Instructions:            []string{"Do not ask about production credentials"},
			Actions:                 []string{"prompt"},
			PromptFilter:            "PRODUCTION CREDENTIALS",
			PromptFilterIgnoreCase:  true,
			GlobPatterns:            []string{"**/*.go"},
			Importance:              "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for prompt action with case-insensitive --matches, got: %v", result.Errors)
		}
	})
}

func TestToolAndCommandFilters(t *testing.T) {
	t.Run("valid tool filter", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test tool filter",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Handle MCP tools"},
			Actions:      []string{"mcp_tool"},
			ToolFilter:   "github.*",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for valid tool filter, got: %v", result.Errors)
		}
	})

	t.Run("invalid tool filter", func(t *testing.T) {
		rule := Rule{
			Summary:      "Test invalid tool filter",
			Topic:        "agent-lifecycle-rules",
			Instructions: []string{"Handle MCP tools"},
			Actions:      []string{"mcp_tool"},
			ToolFilter:   "[invalid",
			GlobPatterns: []string{"**/*.go"},
			Importance:   "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if err == `tool_filter "[invalid" is not a valid glob pattern: syntax error in pattern` {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid tool filter, got: %v", result.Errors)
		}
	})

	t.Run("valid command filter", func(t *testing.T) {
		rule := Rule{
			Summary:        "Test command filter",
			Topic:          "agent-lifecycle-rules",
			Instructions:   []string{"Block dangerous commands"},
			Actions:        []string{"bash"},
			CommandFilter:  "rm -rf|chmod -R",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for valid command filter, got: %v", result.Errors)
		}
	})

	t.Run("invalid command filter", func(t *testing.T) {
		rule := Rule{
			Summary:        "Test invalid command filter",
			Topic:          "agent-lifecycle-rules",
			Instructions:   []string{"Block dangerous commands"},
			Actions:        []string{"bash"},
			CommandFilter:  "[invalid",
			GlobPatterns:   []string{"**/*.go"},
			Importance:     "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if strings.Contains(err, "is not a valid regex pattern") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid command filter, got: %v", result.Errors)
		}
	})

	t.Run("case-insensitive command filter", func(t *testing.T) {
		rule := Rule{
			Summary:                 "Test case-insensitive command filter",
			Topic:                   "agent-lifecycle-rules",
			Instructions:            []string{"Block dangerous commands"},
			Actions:                 []string{"bash"},
			CommandFilter:           "RM -RF",
			CommandFilterIgnoreCase: true,
			GlobPatterns:            []string{"**/*.go"},
			Importance:              "medium",
		}

		result := ValidateAndNormalize(rule)
		if !result.Valid() {
			t.Fatalf("expected validation to pass for case-insensitive command filter, got: %v", result.Errors)
		}
	})

	t.Run("case-insensitive invalid command filter", func(t *testing.T) {
		rule := Rule{
			Summary:                 "Test case-insensitive invalid command filter",
			Topic:                   "agent-lifecycle-rules",
			Instructions:            []string{"Block dangerous commands"},
			Actions:                 []string{"bash"},
			CommandFilter:           "[invalid",
			CommandFilterIgnoreCase: true,
			GlobPatterns:            []string{"**/*.go"},
			Importance:              "medium",
		}

		result := ValidateAndNormalize(rule)
		found := false
		for _, err := range result.Errors {
			if strings.Contains(err, "is not a valid regex pattern") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error about invalid command filter, got: %v", result.Errors)
		}
	})
}
