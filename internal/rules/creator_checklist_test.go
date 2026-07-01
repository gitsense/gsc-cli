package rules

import (
	"strings"
	"testing"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

func TestValidateAgentCreatorChecklistRequiresChecklist(t *testing.T) {
	rule := Rule{Summary: "Rule", Topic: "test"}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, "creatorChecklist is required") {
		t.Fatalf("expected missing checklist error, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistAcceptsLowRiskDeclarative(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeDeclarative,
		Summary:      "Run TypeScript checks after edits",
		Topic:        "typescript",
		Event:        EventPreToolUse,
		Instructions: []string{"Run tsc --noEmit after editing TypeScript files."},
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.ts"},
		CreatorChecklist: &CreatorChecklist{
			Creator:  CreatorAgent,
			Intent:   "Ensure agents verify TypeScript compilation after edits.",
			Scope:    "personal",
			RuleKind: "declarative",
			Topic: CreatorChecklistTopic{
				Slug:         "typescript",
				Source:       "existing",
				VerifiedFrom: "gsc topics list",
			},
			Matching: CreatorChecklistMatching{
				Event:  "pre_tool_use",
				Action: "edit",
				Glob:   "**/*.ts",
			},
			Risk: CreatorChecklistRisk{Level: "medium"},
			Verification: CreatorChecklistVerification{
				SyntaxVerifiedFrom: "gsc experts guide rules",
			},
			Unresolved: []string{},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if len(errs) > 0 {
		t.Fatalf("expected checklist to be valid, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistRejectsHighRiskTriggerWithoutConfirmation(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeExecutable,
		Summary:      "TypeScript compile steer",
		Topic:        "typescript",
		Event:        EventPreToolUse,
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.ts"},
		Trigger:      &TriggerConfig{Runtime: "node", Entry: "ts-check.mjs"},
		InstrCfg:     &InstructionConfig{Mode: "inline", Text: "Fix TypeScript errors before continuing."},
		Frequency:    &FrequencyConfig{Mode: FrequencyOncePerFile},
		CreatorChecklist: &CreatorChecklist{
			Creator:  CreatorAgent,
			Intent:   "Block agents until TypeScript errors are fixed.",
			Scope:    "personal",
			RuleKind: "executable-trigger",
			Topic: CreatorChecklistTopic{
				Slug:         "typescript",
				Source:       "existing",
				VerifiedFrom: "gsc topics list",
			},
			Matching: CreatorChecklistMatching{
				Event:  "pre_tool_use",
				Action: "edit",
				Glob:   "**/*.ts",
			},
			Delivery: CreatorChecklistDelivery{
				Mode:                "steer",
				Blocks:              true,
				MessageShownToAgent: "Fix TypeScript errors before continuing.",
			},
			SideEffects: []string{"Runs TypeScript compiler read-only."},
			Risk:        CreatorChecklistRisk{Level: "high"},
			Verification: CreatorChecklistVerification{
				LifecycleSupportVerifiedFrom: "gsc experts guide rules",
				SyntaxVerifiedFrom:           "gsc rules trigger template",
				DeliveryModeVerifiedFrom:     "gsc experts guide rules",
				ValidationPlan:               []string{"gsc rules trigger validate <id>"},
			},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, "confirmation.userConfirmed") {
		t.Fatalf("expected missing confirmation error, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistRejectsUnresolvedAndScopeMismatch(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeDeclarative,
		Summary:      "Rule",
		Topic:        "test",
		Event:        EventPreToolUse,
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.go"},
		CreatorChecklist: &CreatorChecklist{
			Creator:      CreatorAgent,
			Intent:       "Test intent",
			Scope:        "repo",
			RuleKind:     "declarative",
			Topic:        CreatorChecklistTopic{Slug: "test", Source: "existing", VerifiedFrom: "gsc topics list"},
			Matching:     CreatorChecklistMatching{Event: "pre_tool_use", Action: "edit", Glob: "**/*.go"},
			Risk:         CreatorChecklistRisk{Level: "medium"},
			Verification: CreatorChecklistVerification{SyntaxVerifiedFrom: "gsc experts guide rules"},
			Unresolved:   []string{"Need explicit confirmation."},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, "must match --target personal") {
		t.Fatalf("expected scope mismatch error, got: %#v", errs)
	}
	if !hasErrorContaining(errs, "unresolved must be empty") {
		t.Fatalf("expected unresolved error, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistRejectsInvalidDeliveryModeAndRiskLevel(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeDeclarative,
		Summary:      "Rule",
		Topic:        "test",
		Event:        EventPreToolUse,
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.go"},
		CreatorChecklist: &CreatorChecklist{
			Creator:  CreatorAgent,
			Intent:   "Test intent",
			Scope:    "personal",
			RuleKind: "declarative",
			Topic:    CreatorChecklistTopic{Slug: "test", Source: "existing", VerifiedFrom: "gsc topics list"},
			Matching: CreatorChecklistMatching{
				Event:  "pre_tool_use",
				Action: "edit",
				Glob:   "**/*.go",
			},
			Delivery: CreatorChecklistDelivery{Mode: "eventually"},
			Risk:     CreatorChecklistRisk{Level: "dangerous"},
			Verification: CreatorChecklistVerification{
				SyntaxVerifiedFrom: "gsc experts guide rules",
			},
			Unresolved: []string{},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, "delivery.mode must be one of") {
		t.Fatalf("expected invalid delivery mode error, got: %#v", errs)
	}
	if !hasErrorContaining(errs, "risk.level must be one of") {
		t.Fatalf("expected invalid risk level error, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistRequiresTopicPreflight(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeDeclarative,
		Summary:      "Rule",
		Topic:        "test",
		Event:        EventPreToolUse,
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.go"},
		CreatorChecklist: &CreatorChecklist{
			Creator:      CreatorAgent,
			Intent:       "Test intent",
			Scope:        "personal",
			RuleKind:     "declarative",
			Matching:     CreatorChecklistMatching{Event: "pre_tool_use", Action: "edit", Glob: "**/*.go"},
			Risk:         CreatorChecklistRisk{Level: "medium"},
			Verification: CreatorChecklistVerification{SyntaxVerifiedFrom: "gsc experts guide rules"},
			Unresolved:   []string{},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, "topic.slug is required") {
		t.Fatalf("expected missing topic slug error, got: %#v", errs)
	}
	if !hasErrorContaining(errs, "topic.source is required") {
		t.Fatalf("expected missing topic source error, got: %#v", errs)
	}
	if !hasErrorContaining(errs, "topic.verifiedFrom is required") {
		t.Fatalf("expected missing topic verification error, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistRequiresAllMatchingGlobs(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeExecutable,
		Summary:      "TypeScript compile steer",
		Topic:        "typescript",
		Event:        EventPostToolUse,
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.ts", "**/*.tsx"},
		Trigger:      &TriggerConfig{Runtime: "node", Entry: "ts-check.mjs"},
		InstrCfg:     &InstructionConfig{Mode: "inline", Text: "Fix TypeScript errors before continuing."},
		Frequency:    &FrequencyConfig{Mode: FrequencyOncePerFile},
		CreatorChecklist: &CreatorChecklist{
			Creator:  CreatorAgent,
			Intent:   "Block agents until TypeScript errors are fixed.",
			Scope:    "personal",
			RuleKind: "executable-trigger",
			Topic:    CreatorChecklistTopic{Slug: "typescript", Source: "existing", VerifiedFrom: "gsc topics list"},
			Matching: CreatorChecklistMatching{
				Event:  "post_tool_use",
				Action: "edit",
				Glob:   "**/*.ts",
			},
			Delivery: CreatorChecklistDelivery{
				Mode:                "steer",
				Blocks:              true,
				MessageShownToAgent: "Fix TypeScript errors before continuing.",
			},
			SideEffects: []string{"Runs TypeScript compiler read-only."},
			Risk:        CreatorChecklistRisk{Level: "high"},
			Verification: CreatorChecklistVerification{
				LifecycleSupportVerifiedFrom: "gsc experts guide rules",
				SyntaxVerifiedFrom:           "gsc rules trigger template",
				DeliveryModeVerifiedFrom:     "gsc experts guide rules",
				ValidationPlan:               []string{"gsc rules trigger validate <id>"},
			},
			Confirmation: CreatorChecklistConfirmation{
				Required:      true,
				UserConfirmed: true,
				ConfirmedText: "confirm",
			},
			Unresolved: []string{},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, `matching.globs must include rule glob "**/*.tsx"`) {
		t.Fatalf("expected missing glob coverage error, got: %#v", errs)
	}
}

func TestValidateAgentCreatorChecklistRejectsTopicMismatch(t *testing.T) {
	rule := Rule{
		Type:         RuleTypeDeclarative,
		Summary:      "Rule",
		Topic:        "typescript",
		Event:        EventPreToolUse,
		Actions:      []string{"edit"},
		GlobPatterns: []string{"**/*.ts"},
		CreatorChecklist: &CreatorChecklist{
			Creator:  CreatorAgent,
			Intent:   "Test intent",
			Scope:    "personal",
			RuleKind: "declarative",
			Topic:    CreatorChecklistTopic{Slug: "wrong-topic", Source: "existing", VerifiedFrom: "gsc topics list"},
			Matching: CreatorChecklistMatching{Event: "pre_tool_use", Action: "edit", Glob: "**/*.ts"},
			Risk:     CreatorChecklistRisk{Level: "medium"},
			Verification: CreatorChecklistVerification{SyntaxVerifiedFrom: "gsc experts guide rules"},
			Unresolved:   []string{},
		},
	}

	errs := ValidateAgentCreatorChecklist(rule, gitsensescope.TargetPersonal)
	if !hasErrorContaining(errs, `topic.slug "wrong-topic" must match rule topic "typescript"`) {
		t.Fatalf("expected topic mismatch error, got: %#v", errs)
	}
}

func hasErrorContaining(errs []string, needle string) bool {
	for _, err := range errs {
		if strings.Contains(err, needle) {
			return true
		}
	}
	return false
}
