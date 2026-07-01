/**
 * Component: Rules Execution Engine
 * Block-UUID: b2c3d4e5-f6a7-8901-bcde-f01234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Executes matched rules against a context, runs triggers in parallel, and builds execution results. Implements the gsc rules execute command logic.
 * Language: Go
 * Created-at: 2026-06-26T16:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

// V1ExecutionContext is the input context for rule execution.
// This matches the pi-brains lifecycle envelope shape.
type V1ExecutionContext struct {
	Version      string                    `json:"version"`
	Event        V1EventContext            `json:"event"`
	Capabilities V1CapabilitiesContext     `json:"capabilities"`
	Session      V1SessionContext          `json:"session"`
	Conversation V1ConversationContext     `json:"conversation"`
	Model        *V1ModelContext           `json:"model,omitempty"`
	Payload      V1ExecutionContextPayload `json:"payload"`
	Repo         *V1RepoContext            `json:"repo,omitempty"`
	Debug        bool                      `json:"debug,omitempty"`
	DebugPath    string                    `json:"debugPath,omitempty"`
}

// V1ExecutionContextPayload holds event-specific payload data for the execution context.
type V1ExecutionContextPayload struct {
	ToolCall   *V1ToolCallContext   `json:"toolCall,omitempty"`
	Prompt     *V1PromptPayload     `json:"prompt,omitempty"`
	ToolResult *V1ToolResultPayload `json:"toolResult,omitempty"`
	Stop       *V1StopPayload       `json:"stop,omitempty"`
	Session    *V1SessionPayload    `json:"session,omitempty"`
}

// RulesInput is the input rules file format from gsc rules get --format rules-json.
type RulesInput struct {
	SchemaVersion int                `json:"schemaVersion"`
	Query         RulesInputQuery    `json:"query"`
	GitRoot       string             `json:"gitRoot"`
	Rules         []MatchedRuleInput `json:"rules"`
	Summary       RulesInputSummary  `json:"summary"`
}

// RulesInputQuery captures the query that produced the rules.
type RulesInputQuery struct {
	File           string `json:"file,omitempty"`
	NormalizedFile string `json:"normalized_file,omitempty"`
	Glob           string `json:"glob,omitempty"`
	Tag            string `json:"tag,omitempty"`
	Action         string `json:"action,omitempty"`
	Event          string `json:"event,omitempty"`
}

// RulesInputSummary captures summary statistics.
type RulesInputSummary struct {
	Total       int `json:"total"`
	Declarative int `json:"declarative"`
	Executable  int `json:"executable"`
}

// MatchedRuleInput represents a matched rule in the input.
type MatchedRuleInput struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Source       gitsensescope.Source `json:"source,omitempty"`
	Event        string               `json:"event,omitempty"`
	Summary      string               `json:"summary,omitempty"`
	Instructions []string             `json:"instructions,omitempty"`
	Trigger      *TriggerConfig       `json:"trigger,omitempty"`
	Match        *MatchInfo           `json:"match,omitempty"`
	RuleHash     string               `json:"ruleHash,omitempty"`
	TriggerHash  string               `json:"triggerHash,omitempty"`
	Priority     int                  `json:"priority,omitempty"`
	Importance   string               `json:"importance,omitempty"`
	InstrCfg     *InstructionConfig   `json:"instruction,omitempty"`
	Frequency    *FrequencyConfig     `json:"frequency,omitempty"`
}

// MatchInfo captures match provenance.
type MatchInfo struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	File   string `json:"file,omitempty"`
	Action string `json:"action,omitempty"`
}

// ExecutionResult is the output of gsc rules execute.
type ExecutionResult struct {
	SchemaVersion  int                 `json:"schemaVersion"`
	Block          bool                `json:"block"`
	Reason         string              `json:"reason,omitempty"`
	DurationMs     int64               `json:"durationMs,omitempty"` // Total execution time in milliseconds
	Notices        []string            `json:"notices"`
	MatchedRules   []MatchedRuleInfo   `json:"matchedRules"`
	TriggerResults []TriggerResultInfo `json:"triggerResults"`
	Errors         []ErrorInfo         `json:"errors"`
	SubagentTasks  []SubagentTaskInfo  `json:"subagentTasks"`
}

// MatchedRuleInfo represents a matched rule in the output.
type MatchedRuleInfo struct {
	RuleID       string     `json:"ruleId"`
	RuleHash     string     `json:"ruleHash,omitempty"`
	TriggerHash  string     `json:"triggerHash,omitempty"`
	Type         string     `json:"type"`
	Summary      string     `json:"summary,omitempty"`
	Priority     int        `json:"priority,omitempty"`
	Instructions []string   `json:"instructions,omitempty"`
	Match        *MatchInfo `json:"match,omitempty"`
}

// TriggerResultInfo represents a trigger execution result.
type TriggerResultInfo struct {
	RuleID       string `json:"ruleId"`
	Matched      bool   `json:"matched"`
	Block        bool   `json:"block"`
	Message      string `json:"message,omitempty"`
	Notice       string `json:"notice,omitempty"`
	Level        string `json:"level,omitempty"`
	DeliveryMode string `json:"deliveryMode,omitempty"` // Delivery mode: "steer", "followUp", "passiveSteer"
	DurationMs   int64  `json:"durationMs,omitempty"`   // Trigger execution time in milliseconds
	Error        string `json:"error,omitempty"`
	Timeout      bool   `json:"timeout,omitempty"`
}

// ErrorInfo represents an error during execution.
type ErrorInfo struct {
	RuleID  string `json:"ruleId,omitempty"`
	Error   string `json:"error"`
	Timeout bool   `json:"timeout,omitempty"`
}

// SubagentTaskInfo is reserved for future use.
type SubagentTaskInfo struct {
	// Reserved for future subagent support
}

// ExecuteOptions configures rule execution.
type ExecuteOptions struct {
	Concurrency int
	Timeout     time.Duration
}

// ExecuteRules executes matched rules against a context and returns the result.
func ExecuteRules(ctx context.Context, input *RulesInput, execCtx *V1ExecutionContext, opts ExecuteOptions) (*ExecutionResult, error) {
	startTime := time.Now()

	if input == nil {
		return nil, fmt.Errorf("rules input is required")
	}
	if execCtx == nil {
		return nil, fmt.Errorf("execution context is required")
	}

	// Apply defaults
	if opts.Concurrency <= 0 {
		opts.Concurrency = 8
	}

	// Create timeout context if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Partition rules into declarative and executable
	var declarativeRules []MatchedRuleInput
	var executableRules []MatchedRuleInput

	for _, rule := range input.Rules {
		if rule.Type == "executable" && rule.Trigger != nil {
			executableRules = append(executableRules, rule)
		} else {
			declarativeRules = append(declarativeRules, rule)
		}
	}

	// Sort executable rules by priority (desc), then ID (asc) for deterministic ordering
	sort.Slice(executableRules, func(i, j int) bool {
		if executableRules[i].Priority != executableRules[j].Priority {
			return executableRules[i].Priority > executableRules[j].Priority
		}
		return executableRules[i].ID < executableRules[j].ID
	})

	// Initialize result
	result := &ExecutionResult{
		SchemaVersion:  1,
		Notices:        make([]string, 0),
		MatchedRules:   make([]MatchedRuleInfo, 0),
		TriggerResults: make([]TriggerResultInfo, 0),
		Errors:         make([]ErrorInfo, 0),
		SubagentTasks:  make([]SubagentTaskInfo, 0),
	}

	// Build matched rules info (declarative first, then executable)
	for _, rule := range declarativeRules {
		info := buildMatchedRuleInfo(rule)
		result.MatchedRules = append(result.MatchedRules, info)
	}
	for _, rule := range executableRules {
		info := buildMatchedRuleInfo(rule)
		result.MatchedRules = append(result.MatchedRules, info)
	}

	// Execute triggers in parallel
	if len(executableRules) > 0 {
		triggerResults, errors := executeTriggers(ctx, executableRules, execCtx, opts.Concurrency)
		result.TriggerResults = triggerResults
		result.Errors = errors
	}

	// Determine block/allow based on event and capabilities
	block, reason := determineBlock(execCtx, declarativeRules, executableRules, result)

	result.Block = block
	result.Reason = reason

	// Set total duration
	result.DurationMs = time.Since(startTime).Milliseconds()

	// Build notices from trigger results
	for _, tr := range result.TriggerResults {
		if tr.Notice != "" {
			result.Notices = append(result.Notices, tr.Notice)
		}
	}

	return result, nil
}

// buildMatchedRuleInfo builds MatchedRuleInfo from a MatchedRuleInput.
func buildMatchedRuleInfo(rule MatchedRuleInput) MatchedRuleInfo {
	info := MatchedRuleInfo{
		RuleID:       rule.ID,
		RuleHash:     rule.RuleHash,
		TriggerHash:  rule.TriggerHash,
		Type:         rule.Type,
		Summary:      rule.Summary,
		Priority:     rule.Priority,
		Instructions: rule.Instructions,
		Match:        rule.Match,
	}
	return info
}

// executeTriggers executes triggers in parallel with bounded concurrency.
func executeTriggers(ctx context.Context, rules []MatchedRuleInput, execCtx *V1ExecutionContext, concurrency int) ([]TriggerResultInfo, []ErrorInfo) {
	var (
		mu      sync.Mutex
		results = make([]TriggerResultInfo, len(rules))
		errors  []ErrorInfo
		wg      sync.WaitGroup
		sem     = make(chan struct{}, concurrency)
	)

	for i, rule := range rules {
		wg.Add(1)
		go func(idx int, r MatchedRuleInput) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				errors = append(errors, ErrorInfo{
					RuleID:  r.ID,
					Error:   ctx.Err().Error(),
					Timeout: ctx.Err() == context.DeadlineExceeded,
				})
				mu.Unlock()
				return
			}

			// Execute trigger and track duration
			startTime := time.Now()
			result, err := executeSingleTrigger(ctx, r, execCtx)
			durationMs := time.Since(startTime).Milliseconds()

			if err != nil {
				mu.Lock()
				isTimeout := ctx.Err() == context.DeadlineExceeded
				errors = append(errors, ErrorInfo{
					RuleID:  r.ID,
					Error:   err.Error(),
					Timeout: isTimeout,
				})
				mu.Unlock()
				return
			}

			// Set duration on the result
			result.DurationMs = durationMs

			mu.Lock()
			results[idx] = *result
			mu.Unlock()
		}(i, rule)
	}

	wg.Wait()

	// Filter out zero-value results (failed triggers)
	var validResults []TriggerResultInfo
	for _, r := range results {
		if r.RuleID != "" {
			validResults = append(validResults, r)
		}
	}

	return validResults, errors
}

// executeSingleTrigger executes a single trigger and returns the result.
func executeSingleTrigger(ctx context.Context, rule MatchedRuleInput, execCtx *V1ExecutionContext) (*TriggerResultInfo, error) {
	// Build trigger context
	triggerCtx := buildTriggerContext(rule, execCtx)

	// Execute the trigger using existing RunTrigger
	// Copy Event field from input rule to ensure lifecycle event is preserved
	ruleObj := Rule{
		ID:      rule.ID,
		Summary: rule.Summary,
		Event:   LifecycleEvent(rule.Event),
		Type:    RuleTypeExecutable,
		Trigger: rule.Trigger,
	}

	// Determine source - default to repo for backward compatibility
	source := rule.Source
	if source == "" {
		source = gitsensescope.SourceRepo
	}

	triggerResult, err := RunTriggerWithSource(ctx, ruleObj, triggerCtx, source)
	if err != nil {
		return nil, err
	}

	// Convert to TriggerResultInfo
	info := &TriggerResultInfo{
		RuleID:       rule.ID,
		Matched:      triggerResult.IsMatched(),
		Block:        triggerResult.Block && execCtx.Capabilities.CanBlock,
		Message:      triggerResult.Message,
		Notice:       triggerResult.Notice,
		Level:        triggerResult.Level,
		DeliveryMode: triggerResult.DeliveryMode,
	}

	return info, nil
}

// buildTriggerContext builds a V1TriggerContext from a matched rule and execution context.
func buildTriggerContext(rule MatchedRuleInput, execCtx *V1ExecutionContext) V1TriggerContext {
	// Determine lifecycle event
	event := LifecycleEvent(rule.Event)
	if event == "" {
		event = EventPreToolUse
	}

	// Build rule context
	ruleCtx := V1RuleContext{
		ID:          rule.ID,
		Summary:     rule.Summary,
		Type:        rule.Type,
		RuleHash:    rule.RuleHash,
		TriggerHash: rule.TriggerHash,
	}

	// Build trigger context
	triggerCtx := V1TriggerContext{
		Version: "1",
		Event: V1EventContext{
			Name:    string(event),
			Runtime: execCtx.Event.Runtime,
		},
		Session:      execCtx.Session,
		Conversation: execCtx.Conversation,
		Model:        execCtx.Model,
		Capabilities: execCtx.Capabilities,
		Payload: V1PayloadContext{
			Prompt:     execCtx.Payload.Prompt,
			ToolResult: execCtx.Payload.ToolResult,
			Stop:       execCtx.Payload.Stop,
			Session:    execCtx.Payload.Session,
		},
		ToolCall: execCtx.Payload.ToolCall,
		Repo:     execCtx.Repo,
		Rule:     ruleCtx,
	}

	return triggerCtx
}

// determineBlock determines if the execution should block based on event and capabilities.
func determineBlock(execCtx *V1ExecutionContext, declarativeRules []MatchedRuleInput, executableRules []MatchedRuleInput, result *ExecutionResult) (bool, string) {
	event := LifecycleEvent(execCtx.Event.Name)

	// Check capabilities - if canBlock is false, never block
	if !execCtx.Capabilities.CanBlock {
		// Add notice that block was ignored
		if len(declarativeRules) > 0 || hasBlockingTrigger(result.TriggerResults) {
			result.Notices = append(result.Notices, "Block ignored: canBlock=false in context")
		}
		return false, ""
	}

	// Event-specific behavior
	switch event {
	case EventPreToolUse:
		// Declarative rules block with matched-rule packet
		// Executable triggers block based on trigger logic
		hasBlockingTrigger := hasBlockingTrigger(result.TriggerResults)
		hasDeclarative := len(declarativeRules) > 0

		if hasBlockingTrigger || hasDeclarative {
			reason := buildBlockReason(execCtx, declarativeRules, executableRules, result)
			return true, reason
		}

	default:
		// For other events, declarative rules are advisory
		// Executable triggers block based on trigger logic
		if hasBlockingTrigger(result.TriggerResults) {
			reason := buildBlockReason(execCtx, declarativeRules, executableRules, result)
			return true, reason
		}
	}

	return false, ""
}

// hasBlockingTrigger checks if any trigger result has block=true.
func hasBlockingTrigger(results []TriggerResultInfo) bool {
	for _, r := range results {
		if r.Block {
			return true
		}
	}
	return false
}

// buildBlockReason builds the human-readable block reason message.
func buildBlockReason(execCtx *V1ExecutionContext, declarativeRules []MatchedRuleInput, executableRules []MatchedRuleInput, result *ExecutionResult) string {
	var lines []string

	lines = append(lines, "GitSense matched repository rules before this lifecycle event.")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Event: %s", execCtx.Event.Name))
	lines = append(lines, fmt.Sprintf("Runtime: %s", execCtx.Event.Runtime))

	// Original event info
	lines = append(lines, "")
	lines = append(lines, "Original event:")
	if execCtx.Payload.ToolCall != nil {
		lines = append(lines, fmt.Sprintf("- Tool: %s", execCtx.Payload.ToolCall.ToolName))
		lines = append(lines, fmt.Sprintf("- Action: %s", execCtx.Payload.ToolCall.Action))
		if execCtx.Payload.ToolCall.File != nil {
			lines = append(lines, fmt.Sprintf("- File: %s", *execCtx.Payload.ToolCall.File))
		}
		if execCtx.Payload.ToolCall.Command != nil {
			lines = append(lines, fmt.Sprintf("- Command: %s", *execCtx.Payload.ToolCall.Command))
		}
	}

	// Matched rules section
	lines = append(lines, "")
	lines = append(lines, "Matched rules:")
	lines = append(lines, "")

	ruleIndex := 1

	// Declarative rules
	for _, rule := range declarativeRules {
		title := rule.Summary
		if title == "" {
			title = rule.ID
		}
		lines = append(lines, fmt.Sprintf("%d. %s [instruction]", ruleIndex, title))
		lines = append(lines, fmt.Sprintf("   Rule: %s", rule.ID))
		if rule.Match != nil {
			lines = append(lines, fmt.Sprintf("   Match: %s: %s", rule.Match.Kind, rule.Match.Value))
		}
		if len(rule.Instructions) > 0 {
			lines = append(lines, "   Instructions:")
			for _, instruction := range rule.Instructions {
				rendered := renderInstructionTemplate(instruction, execCtx, rule)
				lines = append(lines, fmt.Sprintf("   - %s", rendered))
			}
		}
		lines = append(lines, "")
		ruleIndex++
	}

	// Executable rules
	for _, rule := range executableRules {
		title := rule.Summary
		if title == "" {
			title = rule.ID
		}
		lines = append(lines, fmt.Sprintf("%d. %s [tool-trigger]", ruleIndex, title))
		lines = append(lines, fmt.Sprintf("   Rule: %s", rule.ID))
		if rule.Match != nil {
			lines = append(lines, fmt.Sprintf("   Match: %s: %s", rule.Match.Kind, rule.Match.Value))
		}

		// Find trigger result
		for _, tr := range result.TriggerResults {
			if tr.RuleID == rule.ID {
				lines = append(lines, "   Trigger result:")
				if tr.Error != "" {
					lines = append(lines, fmt.Sprintf("   - Error: %s", tr.Error))
				} else if tr.Block {
					msg := tr.Message
					if msg == "" {
						msg = "No message provided"
					}
					lines = append(lines, fmt.Sprintf("   - BLOCKED: %s", msg))
				} else {
					lines = append(lines, "   - Allowed")
				}
				break
			}
		}

		lines = append(lines, "")
		ruleIndex++
	}

	// Required next steps
	lines = append(lines, "Required next steps:")
	if len(declarativeRules) > 0 {
		lines = append(lines, "- Apply all deterministic instructions above.")
	}
	if hasBlockingTrigger(result.TriggerResults) {
		lines = append(lines, "- Address all blocking trigger results above.")
	}
	lines = append(lines, "- Retry the original tool call only after satisfying the rule packet.")

	return strings.Join(lines, "\n")
}

// renderInstructionTemplate renders an instruction template with context variables.
func renderInstructionTemplate(instruction string, execCtx *V1ExecutionContext, rule MatchedRuleInput) string {
	// Build variable map
	vars := map[string]string{
		"rule_id": rule.ID,
	}

	// Add match info
	if rule.Match != nil {
		vars["match_kind"] = rule.Match.Kind
		vars["match_value"] = rule.Match.Value
		vars["file"] = rule.Match.File
		vars["action"] = rule.Match.Action
	}

	// Add repo info
	if execCtx.Repo != nil {
		vars["repo_root"] = execCtx.Repo.Root
		if execCtx.Repo.NormalizedFile != nil {
			vars["normalized_file"] = *execCtx.Repo.NormalizedFile
		}
	}

	// Add tool call info
	if execCtx.Payload.ToolCall != nil {
		if execCtx.Payload.ToolCall.File != nil {
			vars["file"] = *execCtx.Payload.ToolCall.File
		}
		vars["action"] = execCtx.Payload.ToolCall.Action
	}

	// Replace template variables
	re := regexp.MustCompile(`\{\{([a-z_]+)\}\}`)
	return re.ReplaceAllStringFunc(instruction, func(match string) string {
		name := match[2 : len(match)-2] // Remove {{ and }}
		if value, ok := vars[name]; ok {
			return value
		}
		return match // Keep original if no match
	})
}
