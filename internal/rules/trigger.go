/**
 * Component: Rules Trigger Execution Engine
 * Block-UUID: 1a2b3c4d-5e6f-7890-abcd-ef1234567890
 * Parent-UUID: N/A
 * Version: 4.0.0
 * Description: Executes tool-trigger rules against a context, parses trigger output (V1/V2 schema), and aggregates results. Implements V1 executable trigger contract.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), MiMo-v2.5-pro (v4.0.0)
 */

package rules

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

// V1TriggerContext is the JSON sent to trigger stdin per V1 executable trigger contract.
type V1TriggerContext struct {
	Version      string                `json:"version"`
	Event        V1EventContext        `json:"event"`
	Session      V1SessionContext      `json:"session"`
	Conversation V1ConversationContext `json:"conversation"`
	Model        *V1ModelContext       `json:"model,omitempty"`
	Capabilities V1CapabilitiesContext `json:"capabilities"`
	Payload      V1PayloadContext      `json:"payload"`
	ToolCall     *V1ToolCallContext    `json:"toolCall,omitempty"`
	Repo         *V1RepoContext        `json:"repo,omitempty"`
	Rule         V1RuleContext         `json:"rule"`
}

// V1EventContext describes the lifecycle event.
type V1EventContext struct {
	Name    string `json:"name"`
	Runtime string `json:"runtime,omitempty"`
}

// V1CapabilitiesContext describes what the trigger can do.
type V1CapabilitiesContext struct {
	CanBlock bool `json:"canBlock"`
}

// V1PayloadContext holds event-specific payload data.
type V1PayloadContext struct {
	Prompt     *V1PromptPayload     `json:"prompt,omitempty"`
	ToolResult *V1ToolResultPayload `json:"toolResult,omitempty"`
	Stop       *V1StopPayload       `json:"stop,omitempty"`
	Session    *V1SessionPayload    `json:"session,omitempty"`
}

// V1PromptPayload is the payload for user_prompt_submit and before_agent_start events.
type V1PromptPayload struct {
	Text              string   `json:"text"`
	Images            []string `json:"images,omitempty"`
	Source            string   `json:"source,omitempty"`
	StreamingBehavior string   `json:"streamingBehavior,omitempty"`
	SystemPrompt      string   `json:"systemPrompt,omitempty"`
}

// V1ToolResultPayload is the payload for post_tool_use events.
type V1ToolResultPayload struct {
	ToolCallID string          `json:"toolCallId"`
	ToolName   string          `json:"toolName,omitempty"`
	Input      json.RawMessage `json:"input,omitempty"`
	Content    json.RawMessage `json:"content,omitempty"`
	Output     string          `json:"output,omitempty"`
	IsError    bool            `json:"isError,omitempty"`
	Duration   int64           `json:"duration,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// V1StopPayload is the payload for stop events.
type V1StopPayload struct {
	LastMessage  string   `json:"lastMessage,omitempty"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
}

// V1SessionPayload is the payload for session_start/session_end events.
type V1SessionPayload struct {
	Reason      string `json:"reason,omitempty"`
	SessionPath string `json:"sessionPath,omitempty"`
	CWD         string `json:"cwd,omitempty"`
}

// V1SessionContext describes the agent session.
type V1SessionContext struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	CWD  string `json:"cwd"`
}

// V1ConversationContext describes the conversation state.
type V1ConversationContext struct {
	LeafID     string   `json:"leafId"`
	MessageIDs []string `json:"messageIds"`
}

// V1ModelContext describes the model being used.
type V1ModelContext struct {
	Provider      string `json:"provider"`
	ID            string `json:"id"`
	ThinkingLevel string `json:"thinkingLevel"`
}

// V1ToolCallContext describes the tool call being evaluated.
type V1ToolCallContext struct {
	ID       string          `json:"id"`
	ToolName string          `json:"toolName"`
	Action   string          `json:"action"`
	File     *string         `json:"file,omitempty"`
	Command  *string         `json:"command,omitempty"`
	Input    json.RawMessage `json:"input"`
}

// V1RepoContext describes the repository.
type V1RepoContext struct {
	Root           string  `json:"root"`
	NormalizedFile *string `json:"normalizedFile,omitempty"`
}

// V1RuleContext identifies the rule being evaluated.
type V1RuleContext struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Type        string `json:"type"`
	RuleHash    string `json:"ruleHash"`
	TriggerHash string `json:"triggerHash"`
}

// V1TriggerResult is the JSON returned on stdout by a V1 trigger.
type V1TriggerResult struct {
	Matched bool   `json:"matched"`
	Block   bool   `json:"block"`
	Message string `json:"message,omitempty"`
	Notice  string `json:"notice,omitempty"`
}

// TriggerResult is the JSON returned on stdout by a trigger (V2 schema).
type TriggerResult struct {
	SchemaVersion int            `json:"schemaVersion,omitempty"` // 2 for V2, omit for V1 compat
	Matched       *bool          `json:"matched,omitempty"`       // Does this rule apply to this event? nil = auto-detect
	Block         bool           `json:"block"`                   // Should this tool call stop?
	Message       string         `json:"message,omitempty"`       // Model-facing: block reason or advisory injection
	Notice        string         `json:"notice,omitempty"`        // User/operator-facing: never sent to LLM
	Level         string         `json:"level,omitempty"`         // Severity for notice: info, warning, error
	FrequencyKey  string         `json:"frequencyKey,omitempty"`  // Runtime/frequency metadata
	DeliveryMode  string         `json:"deliveryMode,omitempty"`  // Delivery mode: "steer", "followUp", "passiveSteer"
	Details       map[string]any `json:"details,omitempty"`       // Structured diagnostics for logs/debugging
}

// ValidTriggerLevels is the list of valid notice levels.
var ValidTriggerLevels = []string{"info", "warning", "error"}

// NormalizeTriggerResult normalizes a V1 result to V2 format and applies defaults.
func NormalizeTriggerResult(r *TriggerResult) {
	// Default level to "info"
	if r.Level == "" {
		r.Level = "info"
	}
	// Default schemaVersion to 2
	if r.SchemaVersion == 0 {
		r.SchemaVersion = 2
	}
	// Default matched based on other fields
	if r.Matched == nil {
		matched := r.computeDefaultMatched()
		r.Matched = &matched
	}
}

// computeDefaultMatched determines matched status from other fields when not explicitly set.
// Backward-compatible defaults:
//   - true when block === true
//   - true when message is present
//   - true when notice is present (operator said something, so it "matched" the situation)
//   - false otherwise (e.g., only details)
func (r *TriggerResult) computeDefaultMatched() bool {
	if r.Block {
		return true
	}
	if r.Message != "" {
		return true
	}
	if r.Notice != "" {
		return true
	}
	return false
}

// IsMatched returns true if this trigger result represents a match.
func (r *TriggerResult) IsMatched() bool {
	if r.Matched != nil {
		return *r.Matched
	}
	return r.computeDefaultMatched()
}

// HasMeaningfulEffect returns true if the trigger result has at least one meaningful effect.
func (r *TriggerResult) HasMeaningfulEffect() bool {
	return r.Block || r.Message != "" || r.Notice != "" || len(r.Details) > 0
}

// MatchedTrigger is a trigger that matched and should be delivered.
type MatchedTrigger struct {
	RuleID       string          `json:"ruleId"`
	Block        bool            `json:"block"`
	Message      string          `json:"message,omitempty"`
	Notice       string          `json:"notice,omitempty"`
	Level        string          `json:"level,omitempty"`
	DeliveryMode string          `json:"deliveryMode,omitempty"` // Delivery mode: "steer", "followUp", "passiveSteer"
	Details      map[string]any  `json:"details,omitempty"`
	Frequency    FrequencyConfig `json:"frequency"`
	Priority     int             `json:"priority"`
	RuleHash     string          `json:"ruleHash"`
	DurationMs   int64           `json:"durationMs,omitempty"` // Trigger execution time in milliseconds
}

// TriggerError captures a per-rule execution error.
type TriggerError struct {
	RuleID  string `json:"ruleId"`
	Error   string `json:"error"`
	Timeout bool   `json:"timeout,omitempty"`
}

// AggregateResult is the output of running all triggers.
type AggregateResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Evaluated     int              `json:"evaluated"` // Total triggers evaluated
	Matched       []MatchedTrigger `json:"matched"`   // Only triggers where matched=true
	Errors        []TriggerError   `json:"errors"`
}

// BuildV1TriggerContext builds a V1 trigger context from components.
func BuildV1TriggerContext(
	sessionID, sessionPath, sessionCWD string,
	leafID string, messageIDs []string,
	modelProvider, modelID, thinkingLevel string,
	toolCallID, toolName, action string,
	file *string, command *string, input json.RawMessage,
	repoRoot *string, normalizedFile *string,
	rule Rule, ruleHash, triggerHash string,
) V1TriggerContext {
	ctx := V1TriggerContext{
		Version: "1",
		Event: V1EventContext{
			Name: string(rule.EffectiveEvent()),
		},
		Session: V1SessionContext{
			ID:   sessionID,
			Path: sessionPath,
			CWD:  sessionCWD,
		},
		Conversation: V1ConversationContext{
			LeafID:     leafID,
			MessageIDs: messageIDs,
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		ToolCall: &V1ToolCallContext{
			ID:       toolCallID,
			ToolName: toolName,
			Action:   action,
			File:     file,
			Command:  command,
			Input:    input,
		},
		Rule: V1RuleContext{
			ID:          rule.ID,
			Summary:     rule.Summary,
			Type:        string(rule.Type),
			RuleHash:    ruleHash,
			TriggerHash: triggerHash,
		},
	}

	// Add model if provided
	if modelProvider != "" || modelID != "" {
		ctx.Model = &V1ModelContext{
			Provider:      modelProvider,
			ID:            modelID,
			ThinkingLevel: thinkingLevel,
		}
	}

	// Add repo if provided
	if repoRoot != nil {
		ctx.Repo = &V1RepoContext{
			Root:           *repoRoot,
			NormalizedFile: normalizedFile,
		}
	}

	return ctx
}

// BuildV1TriggerContextForEvent builds a V1 trigger context for a specific lifecycle event.
func BuildV1TriggerContextForEvent(
	sessionID, sessionPath, sessionCWD string,
	leafID string, messageIDs []string,
	modelProvider, modelID, thinkingLevel string,
	event LifecycleEvent,
	payload V1PayloadContext,
	toolCall *V1ToolCallContext,
	repoRoot *string, normalizedFile *string,
	rule Rule, ruleHash, triggerHash string,
) V1TriggerContext {
	ctx := V1TriggerContext{
		Version: "1",
		Event: V1EventContext{
			Name: string(event),
		},
		Session: V1SessionContext{
			ID:   sessionID,
			Path: sessionPath,
			CWD:  sessionCWD,
		},
		Conversation: V1ConversationContext{
			LeafID:     leafID,
			MessageIDs: messageIDs,
		},
		Capabilities: V1CapabilitiesContext{
			CanBlock: true,
		},
		Payload:  payload,
		ToolCall: toolCall,
		Rule: V1RuleContext{
			ID:          rule.ID,
			Summary:     rule.Summary,
			Type:        string(rule.Type),
			RuleHash:    ruleHash,
			TriggerHash: triggerHash,
		},
	}

	// Add model if provided
	if modelProvider != "" || modelID != "" {
		ctx.Model = &V1ModelContext{
			Provider:      modelProvider,
			ID:            modelID,
			ThinkingLevel: thinkingLevel,
		}
	}

	// Add repo if provided
	if repoRoot != nil {
		ctx.Repo = &V1RepoContext{
			Root:           *repoRoot,
			NormalizedFile: normalizedFile,
		}
	}

	return ctx
}

// ComputeTriggerHash computes a SHA-256 hash of the trigger file contents.
func ComputeTriggerHash(triggerPath string) (string, error) {
	data, err := os.ReadFile(triggerPath)
	if err != nil {
		return "", fmt.Errorf("failed to read trigger file: %w", err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}

// TriggerPathForSource resolves a trigger entry path for a given source.
// - source == repo: resolves under repo .gitsense/rules/triggers/
// - source == personal: resolves under $GSC_HOME/rules/triggers/
// - empty source: treats as repo for backward compatibility
func TriggerPathForSource(source gitsensescope.Source, entry string) (string, error) {
	return TriggerPathForSourceWithRepoRoot(source, entry, "")
}

// TriggerPathForSourceWithRepoRoot resolves a trigger entry path for a given
// source. repoRoot is optional and only applies to repo source; when empty,
// repo source falls back to the current git repository.
func TriggerPathForSourceWithRepoRoot(source gitsensescope.Source, entry string, repoRoot string) (string, error) {
	if entry == "" {
		return "", fmt.Errorf("trigger entry is required")
	}
	if filepath.IsAbs(entry) {
		return "", fmt.Errorf("trigger entry must be relative to %s trigger directory: %s", source, entry)
	}

	// Default to repo for backward compatibility
	if source == "" {
		source = gitsensescope.SourceRepo
	}

	var baseDir string
	var err error

	switch source {
	case gitsensescope.SourceRepo:
		var repoDir string
		if repoRoot != "" {
			repoDir = gitsensescope.RepoGitSenseDirForRoot(repoRoot)
		} else {
			repoPath, repoErr := gitsensescope.RepoGitSenseDir()
			if repoErr != nil {
				return "", fmt.Errorf("not in a git repository: %w", repoErr)
			}
			repoDir = repoPath
		}
		baseDir = gitsensescope.RulesTriggersDir(gitsensescope.SourcedDir{Source: source, Path: repoDir})

	case gitsensescope.SourcePersonal:
		personalDir, personalErr := gitsensescope.PersonalGitSenseDir()
		if personalErr != nil {
			return "", fmt.Errorf("failed to resolve personal GSC_HOME: %w", personalErr)
		}
		baseDir = gitsensescope.RulesTriggersDir(gitsensescope.SourcedDir{Source: source, Path: personalDir})

	default:
		return "", fmt.Errorf("invalid source: %s", source)
	}

	// Resolve the full path
	triggerPath := filepath.Join(baseDir, entry)

	// Security: verify resolved path stays under the triggers directory
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute triggers directory: %w", err)
	}
	absTriggerPath, err := filepath.Abs(triggerPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute trigger path: %w", err)
	}
	if !strings.HasPrefix(absTriggerPath, absBaseDir+string(filepath.Separator)) && absTriggerPath != absBaseDir {
		return "", fmt.Errorf("trigger entry escapes %s trigger directory: %s", source, entry)
	}

	return triggerPath, nil
}

// RunTrigger executes a single trigger with the given context (repo source assumed).
func RunTrigger(ctx context.Context, rule Rule, triggerCtx V1TriggerContext) (*TriggerResult, error) {
	return RunTriggerWithSource(ctx, rule, triggerCtx, gitsensescope.SourceRepo)
}

// RunTriggerWithSource executes a single trigger with the given context and source.
func RunTriggerWithSource(ctx context.Context, rule Rule, triggerCtx V1TriggerContext, source gitsensescope.Source) (*TriggerResult, error) {
	if !rule.IsExecutable() {
		return nil, fmt.Errorf("rule %s is not a tool-trigger", rule.ID)
	}
	if rule.Trigger == nil {
		return nil, fmt.Errorf("rule %s has no trigger configuration", rule.ID)
	}

	// Validate runtime
	if !IsValidRuntime(rule.Trigger.Runtime) {
		return nil, fmt.Errorf("unsupported runtime: %s", rule.Trigger.Runtime)
	}

	// Validate event matches rule's event
	expectedEvent := rule.EffectiveEvent()
	if triggerCtx.Event.Name != string(expectedEvent) {
		return nil, fmt.Errorf("context event %q does not match rule event %q", triggerCtx.Event.Name, expectedEvent)
	}

	repoRoot := ""
	if triggerCtx.Repo != nil {
		repoRoot = triggerCtx.Repo.Root
	}
	triggerPath, err := TriggerPathForSourceWithRepoRoot(source, rule.Trigger.Entry, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve trigger path for %s rule %s: %w", source, rule.ID, err)
	}

	// Get runtime command
	runtimeCmd, err := RuntimeCommand(rule.Trigger.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime command: %w", err)
	}

	// Marshal context to JSON
	contextJSON, err := json.Marshal(triggerCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trigger context: %w", err)
	}

	// Create command with timeout
	timeout := rule.EffectiveTimeoutMs()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtimeCmd, triggerPath)
	cmd.Stdin = strings.NewReader(string(contextJSON))

	// Set working directory: repo.root when repo is known, otherwise session.cwd
	cwd := ""
	if triggerCtx.Repo != nil && triggerCtx.Repo.Root != "" {
		cwd = triggerCtx.Repo.Root
	} else if triggerCtx.Session.CWD != "" {
		cwd = triggerCtx.Session.CWD
	}
	if cwd != "" {
		cmd.Dir = cwd
	}

	// Capture stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the trigger
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("trigger timed out after %dms", timeout)
		}
		return nil, fmt.Errorf("trigger failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse output
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return nil, fmt.Errorf("trigger produced no output")
	}

	// Try V2 result first (has schemaVersion or V2-specific fields like deliveryMode, level, details)
	var result TriggerResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("trigger output is not valid JSON: %w", err)
	}

	// If schemaVersion is set or any V2-specific field is present, treat as V2
	isV2 := result.SchemaVersion > 0 || result.DeliveryMode != "" || result.Level != "" || result.Details != nil || result.FrequencyKey != ""

	if !isV2 {
		// Try V1 result (legacy format without V2 fields)
		var v1Result V1TriggerResult
		if err := json.Unmarshal([]byte(output), &v1Result); err == nil {
			// Convert V1 to V2
			result = TriggerResult{
				SchemaVersion: 1,
				Matched:       &v1Result.Matched,
				Block:         v1Result.Block,
				Message:       v1Result.Message,
				Notice:        v1Result.Notice,
			}
		}
	}

	// Normalize
	NormalizeTriggerResult(&result)

	// Validate level
	if result.Level != "" && !isValidTriggerLevel(result.Level) {
		return nil, fmt.Errorf("invalid level %q; must be one of: info, warning, error", result.Level)
	}

	// For matched results, check meaningful effect
	if result.IsMatched() && !result.HasMeaningfulEffect() {
		return nil, fmt.Errorf("matched trigger produced no meaningful effect (need block, message, notice, or details)")
	}

	// Validate block requires message or stored instruction
	if result.Block && result.Message == "" {
		if rule.InstrCfg == nil || rule.InstrCfg.Text == "" {
			return nil, fmt.Errorf("trigger returned block=true but no message and no stored instruction")
		}
	}

	return &result, nil
}

// isValidTriggerLevel checks if a level is valid.
func isValidTriggerLevel(level string) bool {
	for _, valid := range ValidTriggerLevels {
		if level == valid {
			return true
		}
	}
	return false
}

// RunAllTriggers executes all enabled tool-trigger rules against the given context.
func RunAllTriggers(ctx context.Context, triggerCtx V1TriggerContext) (*AggregateResult, error) {
	// Load all records
	records, err := LoadRecords()
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}

	// Filter to enabled tool-trigger rules
	var triggers []Rule
	for _, r := range records {
		if r.IsExecutable() && r.IsEnabled() {
			triggers = append(triggers, r)
		}
	}

	// Sort by priority (higher first), then by ID for stability
	sort.Slice(triggers, func(i, j int) bool {
		if triggers[i].Priority != triggers[j].Priority {
			return triggers[i].Priority > triggers[j].Priority
		}
		return triggers[i].ID < triggers[j].ID
	})

	result := &AggregateResult{
		SchemaVersion: 2,
		Evaluated:     0,
		Matched:       make([]MatchedTrigger, 0),
		Errors:        make([]TriggerError, 0),
	}

	// Run each trigger
	for _, rule := range triggers {
		startTime := time.Now()
		triggerResult, err := RunTrigger(ctx, rule, triggerCtx)
		durationMs := time.Since(startTime).Milliseconds()

		if err != nil {
			isTimeout := strings.Contains(err.Error(), "timed out")
			result.Errors = append(result.Errors, TriggerError{
				RuleID:  rule.ID,
				Error:   err.Error(),
				Timeout: isTimeout,
			})
			result.Evaluated++
			continue
		}

		result.Evaluated++

		// Only include matched triggers in the aggregate matched array
		if !triggerResult.IsMatched() {
			continue
		}

		// Resolve instruction text
		message := rule.GetInstructionText(triggerResult.Message)

		// If mode is query, we could execute the query here (future enhancement)
		if rule.InstrCfg != nil && rule.InstrCfg.Mode == "query" && message == "" {
			// For now, use the query as the message
			message = fmt.Sprintf("Run: %s", rule.InstrCfg.Query)
		}

		// Determine frequency key
		freqKey := triggerResult.FrequencyKey
		if freqKey == "" && rule.Frequency != nil {
			freqKey = rule.Frequency.Key
		}

		matched := MatchedTrigger{
			RuleID:       rule.ID,
			Block:        triggerResult.Block,
			Message:      message,
			Notice:       triggerResult.Notice,
			Level:        triggerResult.Level,
			DeliveryMode: triggerResult.DeliveryMode,
			Details:      triggerResult.Details,
			Frequency: FrequencyConfig{
				Mode: rule.Frequency.Mode,
				Key:  freqKey,
			},
			Priority:   rule.Priority,
			RuleHash:   rule.ComputeRuleHash(),
			DurationMs: durationMs,
		}
		result.Matched = append(result.Matched, matched)
	}

	return result, nil
}

// ValidateTriggerFile validates a trigger file without running it against real context.
// It checks that the file exists and the entry path stays under the appropriate triggers directory.
// Source defaults to repo for backward compatibility.
func ValidateTriggerFile(entry string, runtime string) []string {
	return ValidateTriggerFileWithSource(entry, runtime, gitsensescope.SourceRepo)
}

// ValidateTriggerFileWithSource validates a trigger file with explicit source.
func ValidateTriggerFileWithSource(entry string, runtime string, source gitsensescope.Source) []string {
	var errs []string

	// Validate runtime
	if runtime == "" {
		errs = append(errs, "trigger.runtime is required")
	} else if !IsValidRuntime(runtime) {
		errs = append(errs, fmt.Sprintf("unsupported runtime: %s (must be one of: node, python, bash)", runtime))
	}

	// Validate entry is not empty
	if entry == "" {
		errs = append(errs, "trigger.entry is required")
		return errs
	}

	triggerPath, err := TriggerPathForSource(source, entry)
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to resolve trigger path for %s rule: %v", source, err))
		return errs
	}

	// Security: verify resolved path stays under the triggers directory
	var triggersDir string
	switch source {
	case gitsensescope.SourcePersonal:
		personalDir, personalErr := gitsensescope.PersonalGitSenseDir()
		if personalErr != nil {
			errs = append(errs, fmt.Sprintf("failed to resolve personal GSC_HOME: %v", personalErr))
			return errs
		}
		triggersDir = gitsensescope.RulesTriggersDir(gitsensescope.SourcedDir{Source: source, Path: personalDir})
	default:
		triggersDir, err = TriggersDir()
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to resolve triggers directory: %v", err))
			return errs
		}
	}

	absTriggersDir, err := filepath.Abs(triggersDir)
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to resolve absolute triggers directory: %v", err))
		return errs
	}
	absTriggerPath, err := filepath.Abs(triggerPath)
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to resolve absolute trigger path: %v", err))
		return errs
	}
	if !strings.HasPrefix(absTriggerPath, absTriggersDir+string(filepath.Separator)) && absTriggerPath != absTriggersDir {
		errs = append(errs, fmt.Sprintf("trigger entry escapes %s trigger directory: %s", source, entry))
		return errs
	}

	// Check file exists
	info, err := os.Stat(triggerPath)
	if os.IsNotExist(err) {
		errs = append(errs, fmt.Sprintf("trigger file not found for %s rule: %s", source, triggerPath))
		return errs
	}
	if err != nil {
		errs = append(errs, fmt.Sprintf("failed to stat trigger file: %v", err))
		return errs
	}

	// Check it's a file, not a directory
	if info.IsDir() {
		errs = append(errs, fmt.Sprintf("trigger path is a directory, not a file: %s", triggerPath))
		return errs
	}

	return errs
}

// ValidateTriggerWithContext validates a trigger by running it with a fixture context.
func ValidateTriggerWithContext(ctx context.Context, rule Rule, fixtureCtx V1TriggerContext) []string {
	var errs []string

	// First do static validation
	runtime := ""
	if rule.Trigger != nil {
		runtime = rule.Trigger.Runtime
	}
	errs = append(errs, ValidateTriggerFile(rule.Trigger.Entry, runtime)...)
	if len(errs) > 0 {
		return errs
	}

	// Run the trigger
	result, err := RunTrigger(ctx, rule, fixtureCtx)
	if err != nil {
		errs = append(errs, fmt.Sprintf("trigger execution failed: %v", err))
		return errs
	}

	// Validate output schema
	// block=true requires message or stored instruction
	if result.Block && result.Message == "" {
		if rule.InstrCfg == nil || rule.InstrCfg.Text == "" {
			errs = append(errs, "trigger returned block=true but no message and no stored instruction")
		}
	}

	// matched=false with only notice/details is valid (non-match diagnostic)
	if !result.IsMatched() && result.HasMeaningfulEffect() {
		// Valid non-match with diagnostic output
	}

	return errs
}
