/**
 * Component: Rules Trigger Tests
 * Block-UUID: 6f7a8b9c-0d1e-2345-fabc-567890123456
 * Parent-UUID: N/A
 * Version: 3.0.0
 * Description: Tests for trigger execution, validation, and aggregation (V2 schema with matched/notice/level/details).
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0)
 */


package rules

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTriggerResultParsingV1(t *testing.T) {
	// Test V1 backward compatibility
	tests := []struct {
		name    string
		input   string
		want    TriggerResult
		wantErr bool
	}{
		{
			name:  "v1 block with message",
			input: `{"block": true, "message": "test message", "frequencyKey": "key1"}`,
			want: TriggerResult{
				Block:        true,
				Message:      "test message",
				FrequencyKey: "key1",
			},
		},
		{
			name:  "v1 allow",
			input: `{"block": false}`,
			want: TriggerResult{
				Block: false,
			},
		},
		{
			name:    "v1 invalid json",
			input:   `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TriggerResult
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Block != tt.want.Block {
					t.Errorf("Block = %v, want %v", got.Block, tt.want.Block)
				}
				if got.Message != tt.want.Message {
					t.Errorf("Message = %v, want %v", got.Message, tt.want.Message)
				}
				if got.FrequencyKey != tt.want.FrequencyKey {
					t.Errorf("FrequencyKey = %v, want %v", got.FrequencyKey, tt.want.FrequencyKey)
				}
			}
		})
	}
}

func TestTriggerResultParsingV2(t *testing.T) {
	// Test V2 schema with matched, notice, level, details
	tests := []struct {
		name    string
		input   string
		want    TriggerResult
		wantErr bool
	}{
		{
			name: "v2 blocking with matched",
			input: `{
				"schemaVersion": 2,
				"matched": true,
				"block": true,
				"message": "Run gsc rules get first.",
				"notice": "Blocked edit: README.md has guidance.",
				"level": "warning",
				"frequencyKey": "README.md"
			}`,
			want: TriggerResult{
				SchemaVersion: 2,
				Matched:       boolPtr(true),
				Block:         true,
				Message:       "Run gsc rules get first.",
				Notice:        "Blocked edit: README.md has guidance.",
				Level:         "warning",
				FrequencyKey:  "README.md",
			},
		},
		{
			name: "v2 non-match diagnostic",
			input: `{
				"schemaVersion": 2,
				"matched": false,
				"block": false,
				"notice": "No match: not the target file.",
				"level": "info",
				"details": {
					"checked": true,
					"applies": false
				}
			}`,
			want: TriggerResult{
				SchemaVersion: 2,
				Matched:       boolPtr(false),
				Block:         false,
				Notice:        "No match: not the target file.",
				Level:         "info",
				Details: map[string]any{
					"checked": true,
					"applies": false,
				},
			},
		},
		{
			name: "v2 advisory injection with matched",
			input: `{
				"schemaVersion": 2,
				"matched": true,
				"block": false,
				"message": "This tool often fails unless the path is repo-relative.",
				"notice": "Injected advisory guidance for path-sensitive tool usage.",
				"level": "info",
				"frequencyKey": "path-sensitive-tool"
			}`,
			want: TriggerResult{
				SchemaVersion: 2,
				Matched:       boolPtr(true),
				Block:         false,
				Message:       "This tool often fails unless the path is repo-relative.",
				Notice:        "Injected advisory guidance for path-sensitive tool usage.",
				Level:         "info",
				FrequencyKey:  "path-sensitive-tool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got TriggerResult
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.SchemaVersion != tt.want.SchemaVersion {
					t.Errorf("SchemaVersion = %v, want %v", got.SchemaVersion, tt.want.SchemaVersion)
				}
				if got.Block != tt.want.Block {
					t.Errorf("Block = %v, want %v", got.Block, tt.want.Block)
				}
				if got.Message != tt.want.Message {
					t.Errorf("Message = %v, want %v", got.Message, tt.want.Message)
				}
				if got.Notice != tt.want.Notice {
					t.Errorf("Notice = %v, want %v", got.Notice, tt.want.Notice)
				}
				if got.Level != tt.want.Level {
					t.Errorf("Level = %v, want %v", got.Level, tt.want.Level)
				}
				if got.FrequencyKey != tt.want.FrequencyKey {
					t.Errorf("FrequencyKey = %v, want %v", got.FrequencyKey, tt.want.FrequencyKey)
				}
				// Compare matched pointer
				if tt.want.Matched != nil {
					if got.Matched == nil {
						t.Error("Matched = nil, want non-nil")
					} else if *got.Matched != *tt.want.Matched {
						t.Errorf("Matched = %v, want %v", *got.Matched, *tt.want.Matched)
					}
				}
				// Compare details (JSON numbers are float64)
				if tt.want.Details != nil {
					if got.Details == nil {
						t.Error("Details = nil, want non-nil")
					} else {
						for k, v := range tt.want.Details {
							if got.Details[k] != v {
								t.Errorf("Details[%s] = %v, want %v", k, got.Details[k], v)
							}
						}
					}
				}
			}
		})
	}
}

func TestNormalizeTriggerResult(t *testing.T) {
	tests := []struct {
		name        string
		input       TriggerResult
		wantLevel   string
		wantSchema  int
		wantMatched bool
	}{
		{
			name: "v1 block defaults to matched",
			input: TriggerResult{
				Block:   true,
				Message: "test",
			},
			wantLevel:   "info",
			wantSchema:  2,
			wantMatched: true,
		},
		{
			name: "v1 message defaults to matched",
			input: TriggerResult{
				Block:   false,
				Message: "advisory",
			},
			wantLevel:   "info",
			wantSchema:  2,
			wantMatched: true,
		},
		{
			name: "v1 notice defaults to matched",
			input: TriggerResult{
				Block:  false,
				Notice: "diagnostic",
			},
			wantLevel:   "info",
			wantSchema:  2,
			wantMatched: true,
		},
		{
			name: "v1 details only defaults to not matched",
			input: TriggerResult{
				Block:   false,
				Details: map[string]any{"key": "value"},
			},
			wantLevel:   "info",
			wantSchema:  2,
			wantMatched: false,
		},
		{
			name: "v2 preserves level",
			input: TriggerResult{
				SchemaVersion: 2,
				Block:         true,
				Message:       "test",
				Level:         "warning",
			},
			wantLevel:   "warning",
			wantSchema:  2,
			wantMatched: true,
		},
		{
			name: "explicit matched preserved",
			input: TriggerResult{
				SchemaVersion: 2,
				Matched:       boolPtr(false),
				Block:         false,
				Notice:        "not relevant",
			},
			wantLevel:   "info",
			wantSchema:  2,
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input
			NormalizeTriggerResult(&result)
			if result.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", result.Level, tt.wantLevel)
			}
			if result.SchemaVersion != tt.wantSchema {
				t.Errorf("SchemaVersion = %v, want %v", result.SchemaVersion, tt.wantSchema)
			}
			if result.Matched == nil {
				t.Error("Matched = nil after normalization, want non-nil")
			} else if *result.Matched != tt.wantMatched {
				t.Errorf("Matched = %v, want %v", *result.Matched, tt.wantMatched)
			}
		})
	}
}

func TestIsMatched(t *testing.T) {
	tests := []struct {
		name   string
		result TriggerResult
		want   bool
	}{
		{
			name:   "explicit matched true",
			result: TriggerResult{Matched: boolPtr(true), Block: false},
			want:   true,
		},
		{
			name:   "explicit matched false",
			result: TriggerResult{Matched: boolPtr(false), Block: true},
			want:   false,
		},
		{
			name:   "nil matched with block true",
			result: TriggerResult{Block: true},
			want:   true,
		},
		{
			name:   "nil matched with message",
			result: TriggerResult{Message: "advisory"},
			want:   true,
		},
		{
			name:   "nil matched with notice",
			result: TriggerResult{Notice: "diagnostic"},
			want:   true,
		},
		{
			name:   "nil matched with only details",
			result: TriggerResult{Details: map[string]any{"key": "value"}},
			want:   false,
		},
		{
			name:   "nil matched with nothing",
			result: TriggerResult{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsMatched(); got != tt.want {
				t.Errorf("IsMatched() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasMeaningfulEffect(t *testing.T) {
	tests := []struct {
		name   string
		result TriggerResult
		want   bool
	}{
		{
			name:   "block only",
			result: TriggerResult{Block: true},
			want:   true,
		},
		{
			name:   "message only",
			result: TriggerResult{Message: "advisory"},
			want:   true,
		},
		{
			name:   "notice only",
			result: TriggerResult{Notice: "info for user"},
			want:   true,
		},
		{
			name:   "details only",
			result: TriggerResult{Details: map[string]any{"key": "value"}},
			want:   true,
		},
		{
			name:   "no effect",
			result: TriggerResult{},
			want:   false,
		},
		{
			name:   "block false with no other fields",
			result: TriggerResult{Block: false},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasMeaningfulEffect(); got != tt.want {
				t.Errorf("HasMeaningfulEffect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidTriggerLevel(t *testing.T) {
	tests := []struct {
		level string
		want  bool
	}{
		{"info", true},
		{"warning", true},
		{"error", true},
		{"debug", false},
		{"", false},
		{"INFO", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			if got := isValidTriggerLevel(tt.level); got != tt.want {
				t.Errorf("isValidTriggerLevel(%q) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestTriggerContextMarshaling(t *testing.T) {
	file := "/repo/test.go"
	normalizedFile := "test.go"
	repoRoot := "/repo"
	
	ctx := V1TriggerContext{
		Version: "1",
		Session: V1SessionContext{
			ID:   "session_123",
			Path: "/sessions/example.jsonl",
			CWD:  "/repo",
		},
		Conversation: V1ConversationContext{
			LeafID:     "entry-1",
			MessageIDs: []string{"entry-1", "entry-2"},
		},
		Model: &V1ModelContext{
			Provider:      "xiaomi-token-plan-sgp",
			ID:            "mimo-v2.5-pro",
			ThinkingLevel: "medium",
		},
		ToolCall: &V1ToolCallContext{
			ID:       "call_123",
			ToolName: "edit",
			Action:   "edit",
			File:     &file,
			Input:    json.RawMessage(`{"path": "test.go"}`),
		},
		Repo: &V1RepoContext{
			Root:           repoRoot,
			NormalizedFile: &normalizedFile,
		},
		Rule: V1RuleContext{
			ID:          "rule_abc",
			Summary:     "Test rule",
			Type:        "tool-trigger",
			RuleHash:    "sha256:abc123",
			TriggerHash: "sha256:def456",
		},
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var unmarshaled V1TriggerContext
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if unmarshaled.Version != "1" {
		t.Errorf("Version = %v, want 1", unmarshaled.Version)
	}
	if unmarshaled.ToolCall.ToolName != "edit" {
		t.Errorf("ToolCall.ToolName = %v, want edit", unmarshaled.ToolCall.ToolName)
	}
	if unmarshaled.ToolCall.Action != "edit" {
		t.Errorf("ToolCall.Action = %v, want edit", unmarshaled.ToolCall.Action)
	}
	if unmarshaled.Repo.NormalizedFile == nil || *unmarshaled.Repo.NormalizedFile != "test.go" {
		t.Errorf("Repo.NormalizedFile = %v, want test.go", unmarshaled.Repo.NormalizedFile)
	}
}

func TestRunTriggerInvalidRuleType(t *testing.T) {
	rule := Rule{
		ID:   "test_rule",
		Type: RuleTypeDeclarative,
	}

	ctx := context.Background()
	triggerCtx := V1TriggerContext{}

	_, err := RunTrigger(ctx, rule, triggerCtx)
	if err == nil {
		t.Error("expected error for non-tool-trigger rule")
	}
}

func TestRunTriggerMissingTriggerConfig(t *testing.T) {
	rule := Rule{
		ID:   "test_rule",
		Type: RuleTypeExecutable,
	}

	ctx := context.Background()
	triggerCtx := V1TriggerContext{}

	_, err := RunTrigger(ctx, rule, triggerCtx)
	if err == nil {
		t.Error("expected error for missing trigger config")
	}
}

func TestValidateTriggerFile(t *testing.T) {
	// Test runtime validation
	invalidRuntimes := []string{"ruby", "java", "go", ""}
	for _, runtime := range invalidRuntimes {
		errs := ValidateTriggerFile("test.mjs", runtime)
		if len(errs) == 0 {
			t.Errorf("ValidateTriggerFile(test.mjs, %q) expected errors for invalid runtime", runtime)
		}
	}

	// Test valid runtimes
	validRuntimes := []string{"node", "python", "bash"}
	for _, runtime := range validRuntimes {
		errs := ValidateTriggerFile("nonexistent.mjs", runtime)
		// Should have error about missing file, not invalid runtime
		hasRuntimeError := false
		for _, err := range errs {
			if strings.Contains(err, "runtime") {
				hasRuntimeError = true
			}
		}
		if hasRuntimeError {
			t.Errorf("ValidateTriggerFile(nonexistent.mjs, %q) should not have runtime error", runtime)
		}
	}

	// Test empty entry
	errs := ValidateTriggerFile("", "node")
	if len(errs) == 0 {
		t.Error("ValidateTriggerFile('', 'node') expected errors for empty entry")
	}
}

func TestComputeRuleHash(t *testing.T) {
	// Basic test: different instructions produce different hashes
	rule1 := Rule{
		ID:      "test_rule",
		Type:    RuleTypeDeclarative,
		Summary: "Test rule",
		Instructions: []string{"Do X"},
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.go"},
		Topic:        "testing",
		Importance:   "medium",
	}

	rule2 := Rule{
		ID:      "test_rule",
		Type:    RuleTypeDeclarative,
		Summary: "Test rule",
		Instructions: []string{"Do Y"},
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.go"},
		Topic:        "testing",
		Importance:   "medium",
	}

	hash1 := rule1.ComputeRuleHash()
	hash2 := rule2.ComputeRuleHash()

	if hash1 == "" {
		t.Error("expected non-empty hash")
	}
	if hash1 == hash2 {
		t.Error("expected different hashes for different instructions")
	}

	// Same rule should produce same hash
	hash1Again := rule1.ComputeRuleHash()
	if hash1 != hash1Again {
		t.Error("expected same hash for same rule")
	}
}

func TestComputeRuleHashCanonicalization(t *testing.T) {
	// Test that unordered arrays are sorted
	rule1 := Rule{
		ID:           "test_rule",
		Type:         RuleTypeDeclarative,
		Summary:      "Test",
		Instructions: []string{"instruction"},
		Actions:      []string{"edit", "write"},
		GlobPatterns: []string{"b/**", "a/**"},
		Topic:        "testing",
	}

	rule2 := Rule{
		ID:           "test_rule",
		Type:         RuleTypeDeclarative,
		Summary:      "Test",
		Instructions: []string{"instruction"},
		Actions:      []string{"write", "edit"}, // Different order
		GlobPatterns: []string{"a/**", "b/**"}, // Different order
		Topic:        "testing",
	}

	hash1 := rule1.ComputeRuleHash()
	hash2 := rule2.ComputeRuleHash()

	if hash1 != hash2 {
		t.Errorf("unordered arrays should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestComputeRuleHashInstructionOrderMatters(t *testing.T) {
	// Test that instruction order is preserved
	rule1 := Rule{
		ID:           "test_rule",
		Instructions: []string{"first", "second"},
	}

	rule2 := Rule{
		ID:           "test_rule",
		Instructions: []string{"second", "first"},
	}

	hash1 := rule1.ComputeRuleHash()
	hash2 := rule2.ComputeRuleHash()

	if hash1 == hash2 {
		t.Error("instruction order should matter")
	}
}

func TestComputeRuleHashMetadataIgnored(t *testing.T) {
	// Test that metadata fields don't affect hash
	rule1 := Rule{
		ID:           "test_rule",
		Type:         RuleTypeDeclarative,
		Summary:      "Test",
		Instructions: []string{"instruction"},
		Topic:        "testing",
		CreatedAt:    time.Now().Add(-24 * time.Hour),
		UpdatedAt:    time.Now(),
		Keywords:     []string{"keyword1"},
		AI: AIProvenance{
			Provider: "anthropic",
		},
	}

	rule2 := Rule{
		ID:           "test_rule",
		Type:         RuleTypeDeclarative,
		Summary:      "Test",
		Instructions: []string{"instruction"},
		Topic:        "testing",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now().Add(24 * time.Hour),
		Keywords:     []string{"keyword2"},
		AI: AIProvenance{
			Provider: "openai",
		},
	}

	hash1 := rule1.ComputeRuleHash()
	hash2 := rule2.ComputeRuleHash()

	if hash1 != hash2 {
		t.Errorf("metadata should not affect hash: %s != %s", hash1, hash2)
	}
}

func TestComputeRuleHashNullArraysNormalized(t *testing.T) {
	// Test that nil arrays and empty arrays produce the same hash
	rule1 := Rule{
		ID:           "test_rule",
		GlobPatterns: nil,
		ExcludeGlobs: nil,
	}

	rule2 := Rule{
		ID:           "test_rule",
		GlobPatterns: []string{},
		ExcludeGlobs: []string{},
	}

	hash1 := rule1.ComputeRuleHash()
	hash2 := rule2.ComputeRuleHash()

	if hash1 != hash2 {
		t.Errorf("nil and empty arrays should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{
			name:    "nil defaults to true",
			enabled: nil,
			want:    true,
		},
		{
			name:    "explicitly true",
			enabled: boolPtr(true),
			want:    true,
		},
		{
			name:    "explicitly false",
			enabled: boolPtr(false),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{Enabled: tt.enabled}
			if got := rule.IsEnabled(); got != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEffectiveTimeoutMs(t *testing.T) {
	tests := []struct {
		name    string
		trigger *TriggerConfig
		want    int
	}{
		{
			name:    "nil trigger defaults to 5000",
			trigger: nil,
			want:    5000,
		},
		{
			name:    "zero timeout defaults to 5000",
			trigger: &TriggerConfig{TimeoutMs: 0},
			want:    5000,
		},
		{
			name:    "custom timeout",
			trigger: &TriggerConfig{TimeoutMs: 10000},
			want:    10000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{Trigger: tt.trigger}
			if got := rule.EffectiveTimeoutMs(); got != tt.want {
				t.Errorf("EffectiveTimeoutMs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetInstructionText(t *testing.T) {
	tests := []struct {
		name       string
		triggerMsg string
		instrCfg   *InstructionConfig
		want       string
	}{
		{
			name:       "trigger message takes precedence",
			triggerMsg: "from trigger",
			instrCfg: &InstructionConfig{
				Text: "from rule",
			},
			want: "from trigger",
		},
		{
			name:       "falls back to stored instruction",
			triggerMsg: "",
			instrCfg: &InstructionConfig{
				Text: "from rule",
			},
			want: "from rule",
		},
		{
			name:       "empty when no instruction",
			triggerMsg: "",
			instrCfg:   nil,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{InstrCfg: tt.instrCfg}
			if got := rule.GetInstructionText(tt.triggerMsg); got != tt.want {
				t.Errorf("GetInstructionText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchedTriggerV2Fields(t *testing.T) {
	// Test that MatchedTrigger includes V2 fields
	matched := MatchedTrigger{
		RuleID:  "rule_abc",
		Block:   true,
		Message: "Run gsc rules get first.",
		Notice:  "Blocked edit: README.md has guidance.",
		Level:   "warning",
		DeliveryMode: "passiveSteer",
		Details: map[string]any{
			"checkedPreviousAttempts": 3,
		},
		Frequency: FrequencyConfig{
			Mode: FrequencyOncePerContext,
			Key:  "README.md",
		},
		Priority: 100,
		RuleHash: "sha256:abc123",
	}

	data, err := json.Marshal(matched)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var unmarshaled MatchedTrigger
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if unmarshaled.Notice != "Blocked edit: README.md has guidance." {
		t.Errorf("Notice = %v, want 'Blocked edit: README.md has guidance.'", unmarshaled.Notice)
	}
	if unmarshaled.Level != "warning" {
		t.Errorf("Level = %v, want 'warning'", unmarshaled.Level)
	}
	if unmarshaled.DeliveryMode != "passiveSteer" {
		t.Errorf("DeliveryMode = %v, want 'passiveSteer'", unmarshaled.DeliveryMode)
	}
	if unmarshaled.Details == nil {
		t.Error("Details = nil, want non-nil")
	}
}

func TestAggregateResultIncludesEvaluated(t *testing.T) {
	// Test that AggregateResult includes evaluated count
	result := AggregateResult{
		SchemaVersion: 2,
		Evaluated:     5,
		Matched:       make([]MatchedTrigger, 0),
		Errors:        make([]TriggerError, 0),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var unmarshaled AggregateResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if unmarshaled.Evaluated != 5 {
		t.Errorf("Evaluated = %v, want 5", unmarshaled.Evaluated)
	}
}

func TestMatchedFalseWithNoticeIsValid(t *testing.T) {
	// Test that matched=false with only notice is valid (non-match diagnostic)
	result := TriggerResult{
		Matched: boolPtr(false),
		Block:   false,
		Notice:  "Not relevant to this event.",
		Level:   "info",
	}

	// Should have meaningful effect (notice is present)
	if !result.HasMeaningfulEffect() {
		t.Error("expected HasMeaningfulEffect() to be true for notice-only result")
	}

	// Should not be matched
	if result.IsMatched() {
		t.Error("expected IsMatched() to be false for explicit matched=false")
	}
}

func TestMatchedTrueWithNoticeAppearsInAggregate(t *testing.T) {
	// Test that matched=true with notice appears in aggregate matched
	result := TriggerResult{
		Matched: boolPtr(true),
		Block:   false,
		Notice:  "Relevant diagnostic for this event.",
		Level:   "info",
	}

	// Should be matched
	if !result.IsMatched() {
		t.Error("expected IsMatched() to be true for explicit matched=true")
	}

	// Should have meaningful effect
	if !result.HasMeaningfulEffect() {
		t.Error("expected HasMeaningfulEffect() to be true for notice result")
	}
}

func TestMatchedOmittedWithBlockTrueDefaultsToMatched(t *testing.T) {
	// Test that omitted matched with block=true defaults to matched
	result := TriggerResult{
		Block:   true,
		Message: "Block reason",
	}

	NormalizeTriggerResult(&result)

	if result.Matched == nil {
		t.Error("expected Matched to be set after normalization")
	} else if !*result.Matched {
		t.Error("expected Matched to default to true when block=true")
	}
}

func TestMatchedOmittedWithMessageDefaultsToMatched(t *testing.T) {
	// Test that omitted matched with message defaults to matched
	result := TriggerResult{
		Block:   false,
		Message: "Advisory message",
	}

	NormalizeTriggerResult(&result)

	if result.Matched == nil {
		t.Error("expected Matched to be set after normalization")
	} else if !*result.Matched {
		t.Error("expected Matched to default to true when message is present")
	}
}

func TestMatchedOmittedWithDetailsOnlyDefaultsToNotMatched(t *testing.T) {
	// Test that omitted matched with only details defaults to not matched
	result := TriggerResult{
		Block:   false,
		Details: map[string]any{"key": "value"},
	}

	NormalizeTriggerResult(&result)

	if result.Matched == nil {
		t.Error("expected Matched to be set after normalization")
	} else if *result.Matched {
		t.Error("expected Matched to default to false when only details present")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
