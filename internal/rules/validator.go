/**
 * Component: Rules Validator
 * Block-UUID: f6a7b8c9-d0e1-2345-fabc-456789012345
 * Parent-UUID: N/A
 * Version: 5.0.0
 * Description: Validates rule shape, required fields, glob patterns, actions, bounded text, and tool-trigger configuration. Supports node, python, bash runtimes with lifecycle event validation.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Updated-at: 2026-06-25T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), MiMo-v2.5-pro (v4.0.0), terrchen (v4.1.0), MiMo-v2.5-pro (v5.0.0)
 * Changelog:
 *   v5.0.0 - Redesign action naming: command→bash, exec→tool, add mcp_tool, add tool_filter and command_filter
 */


package rules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
)

// ValidateRule validates a rule's shape and fields.
func ValidateRule(r Rule) []string {
	var errs []string

	// Common validation for all rule types
	errs = append(errs, validateCommon(r)...)

	// Type-specific validation
	switch r.Type {
	case RuleTypeExecutable:
		errs = append(errs, validateExecutable(r)...)
	default:
		// Default to instruction type
		errs = append(errs, validateInstruction(r)...)
	}

	// Event-type constraint validation
	errs = append(errs, validateEventConstraints(r)...)

	return errs
}

// validateCommon validates fields common to all rule types.
func validateCommon(r Rule) []string {
	var errs []string

	if r.Summary == "" {
		errs = append(errs, "summary is required")
	}
	if len(r.Summary) > 240 {
		errs = append(errs, "summary must be 240 characters or fewer")
	}
	if len(r.Details) > 4000 {
		errs = append(errs, "details must be 4000 characters or fewer")
	}

	// Topic is required
	if r.Topic == "" {
		errs = append(errs, "topic is required")
	} else {
		// Validate topic against registry
		registry, regErr := topicstopkg.LoadRegistry()
		if regErr != nil {
			errs = append(errs, fmt.Sprintf("failed to load topic registry: %v", regErr))
		} else if !registry.Exists(r.Topic) {
			errs = append(errs, fmt.Sprintf("topic %q not registered; add with: gsc topics add %s --description \"...\"", r.Topic, r.Topic))
		}
	}

	// Validate related topics
	if len(r.RelatedTopics) > 2 {
		errs = append(errs, "maximum 2 related topics allowed")
	}
	seen := map[string]bool{r.Topic: true}
	for _, rt := range r.RelatedTopics {
		if rt == r.Topic {
			errs = append(errs, "related topic cannot equal primary topic")
		}
		if seen[rt] {
			errs = append(errs, "related topics must be unique")
		}
		seen[rt] = true
	}

	// Validate importance
	switch r.Importance {
	case "low", "medium", "high", "":
	default:
		errs = append(errs, "importance must be one of: low, medium, high")
	}

	// Validate lifecycle event
	if r.Event != "" && !IsValidLifecycleEvent(r.Event) {
		errs = append(errs, fmt.Sprintf("event %q must be one of: session_start, before_agent_start, user_prompt_submit, pre_tool_use, post_tool_use, post_tool_batch, agent_end, session_end", r.Event))
	}

	// Validate tags
	for _, tag := range r.Tags {
		if tag != slugify(tag) {
			errs = append(errs, fmt.Sprintf("tag %q must be a lowercase slug", tag))
		}
	}

	// Validate commands
	for _, command := range r.AppliesTo.Commands {
		if strings.ContainsAny(command, ";&|") {
			errs = append(errs, fmt.Sprintf("command %q must not contain shell control operators", command))
		}
	}

	// Validate tool filter (glob pattern)
	if r.ToolFilter != "" {
		if _, err := path.Match(r.ToolFilter, "test"); err != nil {
			errs = append(errs, fmt.Sprintf("tool_filter %q is not a valid glob pattern: %v", r.ToolFilter, err))
		}
	}
	// Validate command filter (regex pattern)
	if r.CommandFilter != "" {
		pattern := r.CommandFilter
		if r.CommandFilterIgnoreCase {
			pattern = "(?i)" + pattern
		}
		if _, err := regexp.Compile(pattern); err != nil {
			errs = append(errs, fmt.Sprintf("command_filter %q is not a valid regex pattern: %v", r.CommandFilter, err))
		}
	}
	// Validate prompt filter (regex pattern)
	if r.PromptFilter != "" {
		pattern := r.PromptFilter
		if r.PromptFilterIgnoreCase {
			pattern = "(?i)" + pattern
		}
		if _, err := regexp.Compile(pattern); err != nil {
			errs = append(errs, fmt.Sprintf("prompt_filter %q is not a valid regex pattern: %v", r.PromptFilter, err))
		}
	}

	return errs
}

// validateInstruction validates fields specific to instruction-type rules.
func validateInstruction(r Rule) []string {
	var errs []string

	// At least one instruction required
	if len(r.Instructions) == 0 {
		errs = append(errs, "at least one instruction is required")
	}

	// Validate instructions (now just strings)
	seenInstructions := map[string]bool{}
	for i, inst := range r.Instructions {
		if inst == "" {
			errs = append(errs, fmt.Sprintf("instruction %d: text is required", i+1))
		}
		if len(inst) > 300 {
			errs = append(errs, fmt.Sprintf("instruction %d: text must be 300 characters or fewer", i+1))
		}
		if seenInstructions[inst] {
			errs = append(errs, fmt.Sprintf("instruction %d: duplicate instruction: %s", i+1, inst))
		}
		seenInstructions[inst] = true
	}

	// Validate actions (required, non-empty, valid values)
	if len(r.Actions) == 0 {
		errs = append(errs, "at least one action is required")
	}
	seenActions := map[string]bool{}
	for _, action := range r.Actions {
		if action == "" {
			errs = append(errs, "action must not be empty")
			continue
		}
		if !isValidAction(action) {
			errs = append(errs, fmt.Sprintf("action %q must be one of: read, write, edit, bash, tool, mcp_tool, prompt, agent_end", action))
		}
		if seenActions[action] {
			errs = append(errs, fmt.Sprintf("duplicate action: %s", action))
		}
		seenActions[action] = true
	}

	// Validate that bash action requires --matches flag
	if seenActions["bash"] && r.CommandFilter == "" {
		errs = append(errs, "bash action requires --matches flag (use .* to match all commands)")
	}

	// Validate that prompt action requires --matches flag
	if seenActions["prompt"] && r.PromptFilter == "" {
		errs = append(errs, "prompt action requires --matches flag (use .* to match all prompts)")
	}

	// File/command anchors are optional since topic is required
	// Validate glob patterns
	for _, glob := range r.GlobPatterns {
		if filepath.IsAbs(glob) {
			errs = append(errs, fmt.Sprintf("glob must be relative, not absolute: %s", glob))
		}
		if strings.Contains(glob, "..") {
			errs = append(errs, fmt.Sprintf("glob must not contain ..: %s", glob))
		}
	}

	// Validate exclude globs
	for _, glob := range r.ExcludeGlobs {
		if filepath.IsAbs(glob) {
			errs = append(errs, fmt.Sprintf("exclude glob must be relative, not absolute: %s", glob))
		}
		if strings.Contains(glob, "..") {
			errs = append(errs, fmt.Sprintf("exclude glob must not contain ..: %s", glob))
		}
	}

	return errs
}

// validateExecutable validates fields specific to tool-trigger rules.
func validateExecutable(r Rule) []string {
	var errs []string

	// Trigger configuration is required
	if r.Trigger == nil {
		errs = append(errs, "trigger configuration is required for tool-trigger rules")
	} else {
		// Validate trigger runtime
		if r.Trigger.Runtime == "" {
			errs = append(errs, "trigger.runtime is required")
		} else if !IsValidRuntime(r.Trigger.Runtime) {
			errs = append(errs, fmt.Sprintf("trigger.runtime %q not supported; must be one of: node, python, bash", r.Trigger.Runtime))
		}

		// Validate trigger entry
		if r.Trigger.Entry == "" {
			errs = append(errs, "trigger.entry is required")
		} else {
			// Validate entry path doesn't contain ..
			if strings.Contains(r.Trigger.Entry, "..") {
				errs = append(errs, "trigger.entry must not contain ..")
			}

			// Security: verify resolved path stays under .gitsense/rules/triggers/
			triggersDir, dirErr := TriggersDir()
			if dirErr != nil {
				errs = append(errs, fmt.Sprintf("failed to resolve triggers directory: %v", dirErr))
			} else {
				triggerPath, pathErr := TriggerPath(r.Trigger.Entry)
				if pathErr != nil {
					errs = append(errs, fmt.Sprintf("trigger.entry path error: %v", pathErr))
				} else {
					absTriggersDir, _ := filepath.Abs(triggersDir)
					absTriggerPath, _ := filepath.Abs(triggerPath)
					if absTriggersDir != "" && absTriggerPath != "" {
						if !strings.HasPrefix(absTriggerPath, absTriggersDir+string(filepath.Separator)) && absTriggerPath != absTriggersDir {
							errs = append(errs, "trigger.entry escapes triggers directory")
						}
					}

					// Check that trigger file exists
					if _, statErr := os.Stat(triggerPath); os.IsNotExist(statErr) {
						errs = append(errs, fmt.Sprintf("trigger file does not exist: %s (create the file in .gitsense/rules/triggers/ or copy it there)", triggerPath))
					}
				}
			}
		}

		// Validate timeout (positive, bounded at 60 seconds)
		if r.Trigger.TimeoutMs < 0 {
			errs = append(errs, "trigger.timeoutMs must be non-negative")
		} else if r.Trigger.TimeoutMs > 60000 {
			errs = append(errs, "trigger.timeoutMs must not exceed 60000 (60 seconds)")
		}
	}

	// Instruction configuration is optional for tool-trigger (can use trigger message only)
	if r.InstrCfg != nil {
		// Validate instruction mode
		switch r.InstrCfg.Mode {
		case "inline":
			if r.InstrCfg.Text == "" {
				errs = append(errs, "instruction.text is required when mode is \"inline\"")
			}
		case "query":
			if r.InstrCfg.Query == "" {
				errs = append(errs, "instruction.query is required when mode is \"query\"")
			}
		case "":
			errs = append(errs, "instruction.mode is required")
		default:
			errs = append(errs, fmt.Sprintf("instruction.mode %q must be one of: inline, query", r.InstrCfg.Mode))
		}
	}

	// Frequency configuration is required
	if r.Frequency == nil {
		errs = append(errs, "frequency configuration is required for tool-trigger rules")
	} else {
		// Validate frequency mode
		if r.Frequency.Mode == "" {
			errs = append(errs, "frequency.mode is required")
		} else if !isValidFrequencyMode(r.Frequency.Mode) {
			errs = append(errs, fmt.Sprintf("frequency.mode %q must be one of: always, once-per-turn, once-per-context, once-per-session, once-per-branch, once-per-file, once-per-rule-hash", r.Frequency.Mode))
		}
	}

	return errs
}

// isValidFrequencyMode checks if a frequency mode is valid.
func isValidFrequencyMode(mode FrequencyMode) bool {
	for _, valid := range ValidFrequencyModes {
		if mode == valid {
			return true
		}
	}
	return false
}

// isValidAction checks if an action is in the valid actions list.
func isValidAction(action string) bool {
	for _, valid := range ValidActions {
		if action == valid {
			return true
		}
	}
	return false
}

// ValidateEventConstraints validates that the rule type is allowed for the given lifecycle event.
// Enforces:
//   - user_prompt_submit: executable triggers or declarative rules with prompt action
//   - before_agent_start: both types allowed, action must be "prompt"
//   - agent_start: notification only, executable triggers only
//   - agent_end: executable triggers only, action must be "agent_end"
//   - pre_tool_use/post_tool_use: actions must be read, write, edit, bash, tool, mcp_tool
//   - context: message injection only, executable triggers only
//   - session_before_compact: cancel/customize only, executable triggers only
//   - session_compact: notification only, executable triggers only
//   - session_start/session_end: notification only, no actions
func validateEventConstraints(r Rule) []string {
	var errs []string
	event := r.EffectiveEvent()

	// Define valid actions for each event
	validActionsByEvent := map[LifecycleEvent][]string{
		EventUserPromptSubmit:     {"prompt"},
		EventBeforeAgentStart:     {"prompt"},
		EventAgentStart:           {}, // notification only
		EventAgentEnd:             {"agent_end"},
		EventPreToolUse:           {"read", "write", "edit", "bash", "tool", "mcp_tool"},
		EventPostToolUse:          {"read", "write", "edit", "bash", "tool", "mcp_tool"},
		EventContext:              {}, // message injection only
		EventSessionBeforeCompact: {}, // cancel/customize only
		EventSessionCompact:       {}, // notification only
		EventSessionStart:         {}, // notification only
		EventSessionEnd:           {}, // notification only
		EventPostToolBatch:        {"read", "write", "edit", "bash", "tool", "mcp_tool"},
	}

	// Check rule type constraints
	switch event {
	case EventUserPromptSubmit:
		// user_prompt_submit supports both executable triggers and declarative rules with prompt action
		if !r.IsExecutable() && !contains(r.Actions, "prompt") {
			errs = append(errs, "user_prompt_submit requires executable triggers or declarative rules with prompt action")
		}
	case EventAgentEnd:
		// agent_end only supports executable triggers, not deterministic instructions
		if !r.IsExecutable() {
			errs = append(errs, "agent_end only supports executable triggers, not deterministic instructions")
		}
	case EventAgentStart:
		// agent_start is notification only - no actions allowed, executable triggers only
		if len(r.Actions) > 0 {
			errs = append(errs, "agent_start is notification only and does not support actions")
		}
		if !r.IsExecutable() {
			errs = append(errs, "agent_start only supports executable triggers for side effects")
		}
	case EventContext:
		// context is message injection only - no actions allowed, executable triggers only
		if len(r.Actions) > 0 {
			errs = append(errs, "context is message injection only and does not support actions")
		}
		if !r.IsExecutable() {
			errs = append(errs, "context only supports executable triggers for knowledge injection")
		}
	case EventSessionBeforeCompact:
		// session_before_compact is cancel/customize only - no actions allowed, executable triggers only
		if len(r.Actions) > 0 {
			errs = append(errs, "session_before_compact is cancel/customize only and does not support actions")
		}
		if !r.IsExecutable() {
			errs = append(errs, "session_before_compact only supports executable triggers for context preservation")
		}
	case EventSessionCompact:
		// session_compact is notification only - no actions allowed, executable triggers only
		if len(r.Actions) > 0 {
			errs = append(errs, "session_compact is notification only and does not support actions")
		}
		if !r.IsExecutable() {
			errs = append(errs, "session_compact only supports executable triggers for context refresh")
		}
	case EventSessionStart, EventSessionEnd:
		// session_start and session_end are notification only - no actions allowed
		if len(r.Actions) > 0 {
			errs = append(errs, fmt.Sprintf("%s is notification only and does not support actions", event))
		}
	}

	// Validate actions against event compatibility
	if validActions, ok := validActionsByEvent[event]; ok && len(validActions) > 0 {
		for _, action := range r.Actions {
			if !contains(validActions, action) {
				errs = append(errs, fmt.Sprintf("action %q is not valid for event %s; valid actions are: %s", action, event, strings.Join(validActions, ", ")))
			}
		}
	}

	return errs
}

// contains checks if a string is in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ValidateAndNormalize validates and normalizes a rule, returning a ValidationResult.
func ValidateAndNormalize(r Rule) ValidationResult {
	normalized := normalizeDraft(r)
	errs := ValidateRule(normalized)
	return ValidationResult{
		Rule:   normalized,
		Errors: errs,
	}
}
