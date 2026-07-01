package rules

import (
	"fmt"
	"strings"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

const CreatorAgent = "agent"

var validCreatorDeliveryModes = map[string]bool{
	"":              true,
	"steer":         true,
	"followup":      true,
	"passivesteer":  true,
	"passive-steer": true,
	"none":          true,
}

var validCreatorRiskLevels = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

var validCreatorTopicSources = map[string]bool{
	"existing": true,
	"created":  true,
}

// ValidateAgentCreatorChecklist validates the extra structured checklist that
// AI agents must provide when creating or updating rules with --creator agent.
func ValidateAgentCreatorChecklist(r Rule, target gitsensescope.Target) []string {
	var errs []string
	checklist := r.CreatorChecklist
	if checklist == nil {
		return []string{"creatorChecklist is required when --creator agent is used"}
	}

	if cleanString(checklist.Creator) != CreatorAgent {
		errs = append(errs, `creatorChecklist.creator must be "agent"`)
	}
	if cleanString(checklist.Intent) == "" {
		errs = append(errs, "creatorChecklist.intent is required")
	}
	if cleanString(checklist.Scope) == "" {
		errs = append(errs, "creatorChecklist.scope is required")
	} else if cleanString(checklist.Scope) != string(target) {
		errs = append(errs, fmt.Sprintf("creatorChecklist.scope %q must match --target %s", checklist.Scope, target))
	}
	if cleanString(checklist.RuleKind) == "" {
		errs = append(errs, "creatorChecklist.ruleKind is required")
	} else if !ruleKindMatches(checklist.RuleKind, r) {
		errs = append(errs, fmt.Sprintf("creatorChecklist.ruleKind %q does not match rule type %q", checklist.RuleKind, effectiveRuleKind(r)))
	}
	if len(checklist.Unresolved) > 0 {
		errs = append(errs, "creatorChecklist.unresolved must be empty before writing a rule")
	}

	errs = append(errs, validateChecklistTopic(r, checklist)...)
	errs = append(errs, validateChecklistMatching(r, checklist)...)
	errs = append(errs, validateChecklistVerification(r, checklist)...)
	errs = append(errs, validateChecklistRiskAndConfirmation(r, checklist)...)

	return errs
}

func ruleKindMatches(kind string, r Rule) bool {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	switch normalized {
	case "declarative", "instruction", "instruction-rule":
		return !r.IsExecutable()
	case "executable", "trigger", "tool-trigger", "executable-trigger":
		return r.IsExecutable()
	default:
		return false
	}
}

func effectiveRuleKind(r Rule) string {
	if r.IsExecutable() {
		return "executable-trigger"
	}
	return "declarative"
}

func validateChecklistTopic(r Rule, checklist *CreatorChecklist) []string {
	var errs []string
	topic := checklist.Topic
	if cleanString(topic.Slug) == "" {
		errs = append(errs, "creatorChecklist.topic.slug is required")
	} else if cleanString(topic.Slug) != cleanString(r.Topic) {
		errs = append(errs, fmt.Sprintf("creatorChecklist.topic.slug %q must match rule topic %q", topic.Slug, r.Topic))
	}

	source := strings.ToLower(cleanString(topic.Source))
	if source == "" {
		errs = append(errs, "creatorChecklist.topic.source is required")
	} else if !validCreatorTopicSources[source] {
		errs = append(errs, "creatorChecklist.topic.source must be one of: existing, created")
	}

	if cleanString(topic.VerifiedFrom) == "" {
		errs = append(errs, "creatorChecklist.topic.verifiedFrom is required")
	}
	return errs
}

func validateChecklistMatching(r Rule, checklist *CreatorChecklist) []string {
	var errs []string
	matching := checklist.Matching
	if matching.Event == "" {
		errs = append(errs, "creatorChecklist.matching.event is required")
	} else if LifecycleEvent(cleanString(matching.Event)) != r.EffectiveEvent() {
		errs = append(errs, fmt.Sprintf("creatorChecklist.matching.event %q must match rule event %q", matching.Event, r.EffectiveEvent()))
	}

	actions := normalizeChecklistList(matching.Actions, matching.Action)
	for _, action := range actions {
		if !contains(r.Actions, strings.ToLower(strings.TrimSpace(action))) {
			errs = append(errs, fmt.Sprintf("creatorChecklist.matching action %q must be present in rule actions", action))
		}
	}
	for _, action := range r.Actions {
		if !containsNormalized(actions, action) {
			errs = append(errs, fmt.Sprintf("creatorChecklist.matching.actions must include rule action %q", action))
		}
	}

	globs := normalizeChecklistList(matching.Globs, matching.Glob)
	for _, glob := range globs {
		if !contains(r.GlobPatterns, strings.TrimSpace(glob)) {
			errs = append(errs, fmt.Sprintf("creatorChecklist.matching glob %q must be present in rule glob_patterns", glob))
		}
	}
	for _, glob := range r.GlobPatterns {
		if !containsNormalized(globs, glob) {
			errs = append(errs, fmt.Sprintf("creatorChecklist.matching.globs must include rule glob %q", glob))
		}
	}

	files := normalizeChecklistList(matching.Files, matching.File)
	for _, file := range files {
		if !contains(r.AppliesTo.Files, strings.TrimSpace(file)) {
			errs = append(errs, fmt.Sprintf("creatorChecklist.matching file %q must be present in rule applies_to.files", file))
		}
	}
	for _, file := range r.AppliesTo.Files {
		if !containsNormalized(files, file) {
			errs = append(errs, fmt.Sprintf("creatorChecklist.matching.files must include rule file %q", file))
		}
	}

	if matching.Tool != "" {
		if r.ToolFilter == "" {
			errs = append(errs, "creatorChecklist.matching.tool requires rule tool_filter")
		} else if strings.TrimSpace(matching.Tool) != r.ToolFilter {
			errs = append(errs, "creatorChecklist.matching.tool must match rule tool_filter")
		}
	}
	if matching.Matches != "" {
		filter := r.CommandFilter
		if filter == "" {
			filter = r.PromptFilter
		}
		if filter == "" {
			errs = append(errs, "creatorChecklist.matching.matches requires rule command_filter or prompt_filter")
		} else if strings.TrimSpace(matching.Matches) != filter {
			errs = append(errs, "creatorChecklist.matching.matches must match rule command_filter or prompt_filter")
		}
	}

	return errs
}

func normalizeChecklistList(values []string, singular string) []string {
	out := make([]string, 0, len(values)+1)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !containsNormalized(out, value) {
			out = append(out, value)
		}
	}
	singular = strings.TrimSpace(singular)
	if singular != "" && !containsNormalized(out, singular) {
		out = append(out, singular)
	}
	return out
}

func containsNormalized(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

func validateChecklistVerification(r Rule, checklist *CreatorChecklist) []string {
	var errs []string
	verification := checklist.Verification
	deliveryMode := strings.ToLower(cleanString(checklist.Delivery.Mode))
	if !validCreatorDeliveryModes[deliveryMode] {
		errs = append(errs, `creatorChecklist.delivery.mode must be one of: steer, followUp, passiveSteer, none`)
	}
	if cleanString(verification.SyntaxVerifiedFrom) == "" {
		errs = append(errs, "creatorChecklist.verification.syntaxVerifiedFrom is required")
	}
	if r.IsExecutable() {
		if cleanString(verification.LifecycleSupportVerifiedFrom) == "" {
			errs = append(errs, "creatorChecklist.verification.lifecycleSupportVerifiedFrom is required for executable triggers")
		}
		if cleanString(verification.DeliveryModeVerifiedFrom) == "" && deliveryMode != "" {
			errs = append(errs, "creatorChecklist.verification.deliveryModeVerifiedFrom is required when delivery.mode is set")
		}
		if len(verification.ValidationPlan) == 0 {
			errs = append(errs, "creatorChecklist.verification.validationPlan is required for executable triggers")
		}
		if len(checklist.SideEffects) == 0 {
			errs = append(errs, "creatorChecklist.sideEffects is required for executable triggers")
		}
		if checklist.Delivery.Blocks && cleanString(checklist.Delivery.MessageShownToAgent) == "" {
			errs = append(errs, "creatorChecklist.delivery.messageShownToAgent is required when delivery.blocks is true")
		}
	}
	return errs
}

func validateChecklistRiskAndConfirmation(r Rule, checklist *CreatorChecklist) []string {
	var errs []string
	riskLevel := strings.ToLower(strings.TrimSpace(checklist.Risk.Level))
	if riskLevel == "" {
		errs = append(errs, "creatorChecklist.risk.level is required")
	} else if !validCreatorRiskLevels[riskLevel] {
		errs = append(errs, "creatorChecklist.risk.level must be one of: low, medium, high")
	}
	highRisk := riskLevel == "high" || r.IsExecutable() || checklist.Delivery.Blocks
	if highRisk {
		if !checklist.Confirmation.Required {
			errs = append(errs, "creatorChecklist.confirmation.required must be true for high-risk agent-created rules")
		}
		if !checklist.Confirmation.UserConfirmed {
			errs = append(errs, "creatorChecklist.confirmation.userConfirmed must be true for high-risk agent-created rules")
		}
		if cleanString(checklist.Confirmation.ConfirmedText) == "" {
			errs = append(errs, "creatorChecklist.confirmation.confirmedText is required for high-risk agent-created rules")
		}
	}
	return errs
}
