/**
 * Component: Rules Domain Models
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
 * Parent-UUID: N/A
 * Version: 3.1.0
 * Description: Defines rule, actions, applies-to, AI provenance, changelog, and tool-trigger data structures. Supports instruction and tool-trigger rule types with lifecycle event binding.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Updated-at: 2026-06-24T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), terrchen (v3.1.0)
 * Changelog:
 *   v3.1.0 - Add canonical lifecycle events and event binding to rules
 */

package rules

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	DatabaseName = "gsc-rules"
)

// LifecycleEvent represents a canonical lifecycle event.
type LifecycleEvent string

// Canonical lifecycle events - GitSense owns these names.
// Runtime adapters map platform-specific events to these canonical names.
const (
	EventSessionStart         LifecycleEvent = "session_start"
	EventBeforeAgentStart     LifecycleEvent = "before_agent_start"
	EventUserPromptSubmit     LifecycleEvent = "user_prompt_submit"
	EventAgentStart           LifecycleEvent = "agent_start"
	EventPreToolUse           LifecycleEvent = "pre_tool_use"
	EventPostToolUse          LifecycleEvent = "post_tool_use"
	EventPostToolBatch        LifecycleEvent = "post_tool_batch"
	EventContext              LifecycleEvent = "context"
	EventSessionBeforeCompact LifecycleEvent = "session_before_compact"
	EventSessionCompact       LifecycleEvent = "session_compact"
	EventAgentEnd             LifecycleEvent = "agent_end"
	EventSessionEnd           LifecycleEvent = "session_end"
)

// DefaultLifecycleEvent is the default event for rules without an explicit event.
// This preserves backwards compatibility with existing rules.
const DefaultLifecycleEvent = EventPreToolUse

// ValidLifecycleEvents is the list of all valid canonical lifecycle events.
var ValidLifecycleEvents = []LifecycleEvent{
	EventSessionStart,
	EventBeforeAgentStart,
	EventUserPromptSubmit,
	EventAgentStart,
	EventPreToolUse,
	EventPostToolUse,
	EventPostToolBatch,
	EventContext,
	EventSessionBeforeCompact,
	EventSessionCompact,
	EventAgentEnd,
	EventSessionEnd,
}

// IsValidLifecycleEvent checks if the event is a valid canonical lifecycle event.
func IsValidLifecycleEvent(event LifecycleEvent) bool {
	for _, valid := range ValidLifecycleEvents {
		if event == valid {
			return true
		}
	}
	return false
}

// ValidActions is the list of valid action values.
var ValidActions = []string{"read", "write", "edit", "bash", "tool", "mcp_tool", "prompt", "agent_end"}

// RuleType represents the type of rule.
type RuleType string

const (
	// RuleTypeDeclarative is a declarative rule that matches by criteria (file, glob, action, event).
	RuleTypeDeclarative RuleType = "declarative"
	// RuleTypeExecutable is an executable rule with code that evaluates runtime context.
	RuleTypeExecutable RuleType = "executable"
	// RuleTypeToolTrigger is the legacy name for executable rules (backward compatible).
	RuleTypeToolTrigger RuleType = "tool-trigger"
)

// ValidRuleTypes is the list of valid rule types.
var ValidRuleTypes = []RuleType{RuleTypeDeclarative, RuleTypeExecutable, RuleTypeToolTrigger}

// TriggerConfig defines the executable trigger for a tool-trigger rule.
type TriggerConfig struct {
	Runtime   string `json:"runtime"`             // "node", "python", "bash"
	Entry     string `json:"entry"`               // path to trigger file relative to .gitsense/rules/triggers/
	TimeoutMs int    `json:"timeoutMs,omitempty"` // default: 5000
}

// ValidTriggerRuntimes is the list of supported trigger runtimes.
var ValidTriggerRuntimes = []string{"node", "python", "bash"}

// IsValidRuntime checks if the runtime is supported.
func IsValidRuntime(runtime string) bool {
	for _, valid := range ValidTriggerRuntimes {
		if runtime == valid {
			return true
		}
	}
	return false
}

// RuntimeCommand returns the command to execute for the runtime.
func RuntimeCommand(runtime string) (string, error) {
	switch runtime {
	case "node":
		return "node", nil
	case "python":
		return "python3", nil
	case "bash":
		return "bash", nil
	default:
		return "", fmt.Errorf("unsupported runtime: %s", runtime)
	}
}

// FrequencyMode controls how often a trigger instruction is delivered.
type FrequencyMode string

const (
	FrequencyAlways          FrequencyMode = "always"
	FrequencyOncePerTurn     FrequencyMode = "once-per-turn"
	FrequencyOncePerContext  FrequencyMode = "once-per-context"
	FrequencyOncePerSession  FrequencyMode = "once-per-session"
	FrequencyOncePerBranch   FrequencyMode = "once-per-branch"
	FrequencyOncePerFile     FrequencyMode = "once-per-file"
	FrequencyOncePerRuleHash FrequencyMode = "once-per-rule-hash"
)

// ValidFrequencyModes is the list of valid frequency modes.
var ValidFrequencyModes = []FrequencyMode{
	FrequencyAlways,
	FrequencyOncePerTurn,
	FrequencyOncePerContext,
	FrequencyOncePerSession,
	FrequencyOncePerBranch,
	FrequencyOncePerFile,
	FrequencyOncePerRuleHash,
}

// FrequencyConfig defines how often a trigger instruction should be delivered.
type FrequencyConfig struct {
	Mode FrequencyMode `json:"mode"`
	Key  string        `json:"key,omitempty"` // Optional key for scoping (e.g., file path)
}

// InstructionConfig defines the instruction for a tool-trigger rule.
type InstructionConfig struct {
	Mode  string `json:"mode"`            // "inline" | "query"
	Text  string `json:"text,omitempty"`  // for inline mode
	Query string `json:"query,omitempty"` // for query mode (e.g., gsc knowledge search ...)
}

type AppliesTo struct {
	Files       []string `json:"files"`
	LinkedFiles []string `json:"linked_files"`
	Commands    []string `json:"commands"`
	Topics      []string `json:"topics,omitempty"` // LEGACY: kept for backward compatibility
}

type AIProvenance struct {
	Provider string `json:"provider"`
	ModelID  string `json:"model_id"`
	Agent    string `json:"agent"`
}

type ChangelogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// InstructionObject is used for backward compatibility with old format.
type InstructionObject struct {
	Text    string    `json:"text"`
	Tags    []string  `json:"tags,omitempty"`
	AddedAt time.Time `json:"added_at,omitempty"`
}

// CreatorChecklist captures the plan an AI agent reviewed before creating or
// updating a rule. It is required only when CLI callers pass --creator agent.
type CreatorChecklist struct {
	Creator      string                       `json:"creator"`
	Intent       string                       `json:"intent"`
	Scope        string                       `json:"scope"`
	RuleKind     string                       `json:"ruleKind"`
	Topic        CreatorChecklistTopic        `json:"topic"`
	Matching     CreatorChecklistMatching     `json:"matching"`
	Delivery     CreatorChecklistDelivery     `json:"delivery"`
	SideEffects  []string                     `json:"sideEffects"`
	Risk         CreatorChecklistRisk         `json:"risk"`
	Verification CreatorChecklistVerification `json:"verification"`
	Confirmation CreatorChecklistConfirmation `json:"confirmation"`
	Unresolved   []string                     `json:"unresolved"`
}

type CreatorChecklistTopic struct {
	Slug         string `json:"slug,omitempty"`
	Source       string `json:"source,omitempty"`
	VerifiedFrom string `json:"verifiedFrom,omitempty"`
}

type CreatorChecklistMatching struct {
	Event   string   `json:"event,omitempty"`
	Action  string   `json:"action,omitempty"`
	Actions []string `json:"actions,omitempty"`
	Tool    string   `json:"tool,omitempty"`
	File    string   `json:"file,omitempty"`
	Files   []string `json:"files,omitempty"`
	Glob    string   `json:"glob,omitempty"`
	Globs   []string `json:"globs,omitempty"`
	Matches string   `json:"matches,omitempty"`
}

type CreatorChecklistDelivery struct {
	Mode                string `json:"mode,omitempty"`
	Blocks              bool   `json:"blocks,omitempty"`
	MessageShownToAgent string `json:"messageShownToAgent,omitempty"`
}

type CreatorChecklistRisk struct {
	Level   string   `json:"level,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type CreatorChecklistVerification struct {
	LifecycleSupportVerifiedFrom string   `json:"lifecycleSupportVerifiedFrom,omitempty"`
	SyntaxVerifiedFrom           string   `json:"syntaxVerifiedFrom,omitempty"`
	DeliveryModeVerifiedFrom     string   `json:"deliveryModeVerifiedFrom,omitempty"`
	ValidationPlan               []string `json:"validationPlan,omitempty"`
}

type CreatorChecklistConfirmation struct {
	Required      bool   `json:"required,omitempty"`
	UserConfirmed bool   `json:"userConfirmed,omitempty"`
	ConfirmedText string `json:"confirmedText,omitempty"`
}

type Rule struct {
	ID                      string            `json:"id"`
	SchemaVersion           string            `json:"schema_version"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
	Owner                   string            `json:"owner,omitempty"`
	Contact                 []string          `json:"contact,omitempty"`
	Summary                 string            `json:"summary"`
	Details                 string            `json:"details"`
	Topic                   string            `json:"topic"`
	RelatedTopics           []string          `json:"related_topics"`
	Event                   LifecycleEvent    `json:"event,omitempty"` // Canonical lifecycle event (default: pre_tool_use)
	Instructions            []string          `json:"instructions"`
	Actions                 []string          `json:"actions"`
	ToolFilter              string            `json:"tool_filter,omitempty"`                // Glob pattern for tool name matching (e.g., "github.*")
	CommandFilter           string            `json:"command_filter,omitempty"`             // Regex pattern for bash command matching (e.g., "rm -rf|chmod -R")
	CommandFilterIgnoreCase bool              `json:"command_filter_ignore_case,omitempty"` // Case-insensitive command matching
	PromptFilter            string            `json:"prompt_filter,omitempty"`              // Regex pattern for prompt content matching (e.g., "AKIA[0-9A-Z]{16}")
	PromptFilterIgnoreCase  bool              `json:"prompt_filter_ignore_case,omitempty"`  // Case-insensitive prompt matching
	GlobPatterns            []string          `json:"glob_patterns"`
	ExcludeGlobs            []string          `json:"exclude_globs"`
	AppliesTo               AppliesTo         `json:"applies_to"`
	Tags                    []string          `json:"tags"`
	Keywords                []string          `json:"keywords"`
	ParentKeywords          []string          `json:"parent_keywords"`
	Importance              string            `json:"importance"`
	AI                      AIProvenance      `json:"ai"`
	ConfirmedBy             string            `json:"confirmed_by,omitempty"`
	ConfirmedAt             time.Time         `json:"confirmed_at,omitempty"`
	Changelog               []ChangelogEntry  `json:"changelog,omitempty"`
	CreatorChecklist        *CreatorChecklist `json:"creatorChecklist,omitempty"`

	// Executable fields (only used when Type == RuleTypeExecutable)
	Type      RuleType           `json:"type,omitempty"`        // "declarative" (default) or "executable"
	Trigger   *TriggerConfig     `json:"trigger,omitempty"`     // Trigger configuration
	InstrCfg  *InstructionConfig `json:"instruction,omitempty"` // Instruction configuration for tool-trigger
	Frequency *FrequencyConfig   `json:"frequency,omitempty"`   // Frequency configuration
	Priority  int                `json:"priority,omitempty"`    // Higher = executed first
	Enabled   *bool              `json:"enabled,omitempty"`     // Default true
}

// UnmarshalJSON implements custom JSON unmarshaling to handle both old (object) and new (string) instruction formats,
// and to support the new tool-trigger fields.
func (r *Rule) UnmarshalJSON(data []byte) error {
	// Use a raw struct to avoid infinite recursion
	type RawRule struct {
		ID                      string            `json:"id"`
		SchemaVersion           string            `json:"schema_version"`
		CreatedAt               time.Time         `json:"created_at"`
		UpdatedAt               time.Time         `json:"updated_at"`
		Owner                   string            `json:"owner,omitempty"`
		Contact                 []string          `json:"contact,omitempty"`
		Summary                 string            `json:"summary"`
		Details                 string            `json:"details"`
		Topic                   string            `json:"topic"`
		RelatedTopics           []string          `json:"related_topics"`
		Event                   LifecycleEvent    `json:"event,omitempty"`
		Instructions            json.RawMessage   `json:"instructions"`
		Actions                 []string          `json:"actions"`
		ToolFilter              string            `json:"tool_filter,omitempty"`
		CommandFilter           string            `json:"command_filter,omitempty"`
		CommandFilterIgnoreCase bool              `json:"command_filter_ignore_case,omitempty"`
		PromptFilter            string            `json:"prompt_filter,omitempty"`
		PromptFilterIgnoreCase  bool              `json:"prompt_filter_ignore_case,omitempty"`
		GlobPatterns            []string          `json:"glob_patterns"`
		ExcludeGlobs            []string          `json:"exclude_globs"`
		AppliesTo               AppliesTo         `json:"applies_to"`
		Tags                    []string          `json:"tags"`
		Keywords                []string          `json:"keywords"`
		ParentKeywords          []string          `json:"parent_keywords"`
		Importance              string            `json:"importance"`
		AI                      AIProvenance      `json:"ai"`
		ConfirmedBy             string            `json:"confirmed_by,omitempty"`
		ConfirmedAt             time.Time         `json:"confirmed_at,omitempty"`
		Changelog               []ChangelogEntry  `json:"changelog,omitempty"`
		CreatorChecklist        *CreatorChecklist `json:"creatorChecklist,omitempty"`

		// Tool-trigger fields
		Type      RuleType           `json:"type,omitempty"`
		Trigger   *TriggerConfig     `json:"trigger,omitempty"`
		InstrCfg  *InstructionConfig `json:"instruction,omitempty"`
		Frequency *FrequencyConfig   `json:"frequency,omitempty"`
		Priority  int                `json:"priority,omitempty"`
		Enabled   *bool              `json:"enabled,omitempty"`
	}

	var raw RawRule
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Copy simple fields
	r.ID = raw.ID
	r.SchemaVersion = raw.SchemaVersion
	r.CreatedAt = raw.CreatedAt
	r.UpdatedAt = raw.UpdatedAt
	r.Owner = raw.Owner
	r.Contact = raw.Contact
	r.Summary = raw.Summary
	r.Details = raw.Details
	r.Topic = raw.Topic
	r.RelatedTopics = raw.RelatedTopics
	r.Event = raw.Event
	r.Actions = raw.Actions
	r.ToolFilter = raw.ToolFilter
	r.CommandFilter = raw.CommandFilter
	r.CommandFilterIgnoreCase = raw.CommandFilterIgnoreCase
	r.PromptFilter = raw.PromptFilter
	r.PromptFilterIgnoreCase = raw.PromptFilterIgnoreCase
	r.GlobPatterns = raw.GlobPatterns
	r.ExcludeGlobs = raw.ExcludeGlobs
	r.AppliesTo = raw.AppliesTo
	r.Tags = raw.Tags
	r.Keywords = raw.Keywords
	r.ParentKeywords = raw.ParentKeywords
	r.Importance = raw.Importance
	r.AI = raw.AI
	r.ConfirmedBy = raw.ConfirmedBy
	r.ConfirmedAt = raw.ConfirmedAt
	r.Changelog = raw.Changelog
	r.CreatorChecklist = raw.CreatorChecklist

	// Copy tool-trigger fields
	r.Type = raw.Type
	r.Trigger = raw.Trigger
	r.InstrCfg = raw.InstrCfg
	r.Frequency = raw.Frequency
	r.Priority = raw.Priority
	r.Enabled = raw.Enabled

	// Handle instructions - could be old format (objects) or new format (strings)
	if len(raw.Instructions) > 0 {
		// Try new format first (array of strings)
		var stringInstructions []string
		if err := json.Unmarshal(raw.Instructions, &stringInstructions); err == nil {
			r.Instructions = stringInstructions
		} else {
			// Try old format (array of objects)
			var objectInstructions []InstructionObject
			if err := json.Unmarshal(raw.Instructions, &objectInstructions); err == nil {
				// Convert old format to new format (extract text only)
				for _, obj := range objectInstructions {
					r.Instructions = append(r.Instructions, obj.Text)
				}
			}
		}
	}

	return nil
}

type ValidationResult struct {
	Rule   Rule
	Errors []string
}

func (r ValidationResult) Valid() bool {
	return len(r.Errors) == 0
}

// NormalizeTopics migrates topics from legacy AppliesTo.Topics to top-level Topic/RelatedTopics.
func (r *Rule) NormalizeTopics() {
	// If new field is empty, migrate from legacy
	if r.Topic == "" && len(r.AppliesTo.Topics) > 0 {
		r.Topic = r.AppliesTo.Topics[0]
		if len(r.AppliesTo.Topics) > 1 {
			r.RelatedTopics = r.AppliesTo.Topics[1:min(3, len(r.AppliesTo.Topics))]
		}
	}
	// Clear legacy field after migration
	r.AppliesTo.Topics = nil
}

// IsExecutable returns true if this rule is an executable type.
// Accepts both "executable" (canonical) and "tool-trigger" (legacy) types.
func (r *Rule) IsExecutable() bool {
	return r.Type == RuleTypeExecutable || r.Type == RuleTypeToolTrigger
}

// IsEnabled returns true if the rule is enabled (defaults to true if not set).
func (r *Rule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// EffectiveTimeoutMs returns the trigger timeout in milliseconds (default 5000).
func (r *Rule) EffectiveTimeoutMs() int {
	if r.Trigger == nil || r.Trigger.TimeoutMs == 0 {
		return 5000
	}
	return r.Trigger.TimeoutMs
}

// GetInstructionText returns the effective instruction text for a tool-trigger rule.
// It returns the trigger-returned message if provided, otherwise the stored instruction text.
func (r *Rule) GetInstructionText(triggerMessage string) string {
	if triggerMessage != "" {
		return triggerMessage
	}
	if r.InstrCfg != nil && r.InstrCfg.Text != "" {
		return r.InstrCfg.Text
	}
	return ""
}

// EffectiveEvent returns the canonical lifecycle event for this rule.
// Defaults to pre_tool_use if not set, preserving backwards compatibility.
func (r *Rule) EffectiveEvent() LifecycleEvent {
	if r.Event == "" {
		return DefaultLifecycleEvent
	}
	return r.Event
}

// canonicalRuleHashInput is the canonical form of a rule used for hashing.
// Only includes fields that affect delivery behavior.
type canonicalRuleHashInput struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Event        string   `json:"event"`
	Summary      string   `json:"summary"`
	Instructions []string `json:"instructions"` // Order matters - not sorted
	Actions      []string `json:"actions"`
	GlobPatterns []string `json:"glob_patterns"`
	ExcludeGlobs []string `json:"exclude_globs"`
	AppliesTo    struct {
		Files    []string `json:"files"`
		Commands []string `json:"commands"`
	} `json:"applies_to"`
	Topic      string `json:"topic"`
	Importance string `json:"importance"`
}

// ComputeRuleHash computes a canonical semantic SHA-256 hash of the rule.
// The hash changes when agent guidance should be delivered again, but stays
// stable across metadata churn (timestamps, keywords, AI provenance, etc.).
//
// Included fields: id, type, summary, instructions, actions, glob_patterns,
// exclude_globs, applies_to.files, applies_to.commands, topic, importance.
//
// Canonicalization rules:
//   - Object keys are sorted (via json.Marshal with sorted struct fields)
//   - Null arrays normalized to []
//   - Unordered arrays are sorted (actions, glob_patterns, exclude_globs, applies_to.*)
//   - Instruction order is preserved (order matters for guidance)
//   - No automatic trimming - hash what is stored after validation
func (r *Rule) ComputeRuleHash() string {
	input := canonicalRuleHashInput{
		ID:           r.ID,
		Type:         string(r.Type),
		Event:        string(r.EffectiveEvent()),
		Summary:      r.Summary,
		Instructions: r.Instructions, // Preserved order
		Topic:        r.Topic,
		Importance:   r.Importance,
	}

	// Sort unordered arrays
	input.Actions = sortedCopy(r.Actions)
	input.GlobPatterns = sortedCopy(r.GlobPatterns)
	input.ExcludeGlobs = sortedCopy(r.ExcludeGlobs)
	input.AppliesTo.Files = sortedCopy(r.AppliesTo.Files)
	input.AppliesTo.Commands = sortedCopy(r.AppliesTo.Commands)

	// Default type to "declarative" if empty
	if input.Type == "" {
		input.Type = "declarative"
	}

	// Marshal to canonical JSON (keys are sorted via struct field order)
	canonical, err := json.Marshal(input)
	if err != nil {
		// Should not happen with simple types
		return "sha256:error"
	}

	h := sha256.Sum256(canonical)
	return fmt.Sprintf("sha256:%x", h)
}

// sortedCopy returns a sorted copy of the slice, normalizing nil to empty.
func sortedCopy(s []string) []string {
	if s == nil {
		return []string{}
	}
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}

// ComputeTriggerHash computes a SHA-256 hash of the trigger file contents.
// Returns an error if the trigger file cannot be read.
func (r *Rule) ComputeTriggerHash() (string, error) {
	if r.Trigger == nil || r.Trigger.Entry == "" {
		return "", fmt.Errorf("no trigger entry configured")
	}

	// Resolve trigger path
	var triggerPath string
	if filepath.IsAbs(r.Trigger.Entry) {
		triggerPath = r.Trigger.Entry
	} else {
		var err error
		triggerPath, err = TriggerPath(r.Trigger.Entry)
		if err != nil {
			return "", fmt.Errorf("failed to resolve trigger path: %w", err)
		}
	}

	// Read file
	data, err := os.ReadFile(triggerPath)
	if err != nil {
		return "", fmt.Errorf("failed to read trigger file: %w", err)
	}

	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}
