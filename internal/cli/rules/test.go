/**
 * Component: Rules Test Command
 * Block-UUID: d1e2f3a4-b5c6-7890-def0-123456789012
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc rules test for replay-testing instruction and tool-trigger rules against Pi session JSONL tool calls.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	sessionspkg "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

func testCmd() *cobra.Command {
	var (
		sessionPath string
		leafID      string
		format      string
		timeout     int
		scopeValue  string
	)

	cmd := &cobra.Command{
		Use:   "test <rule-id>",
		Short: "Replay-test a rule against Pi session tool calls",
		Long: `Test whether a rule is surgical enough by replaying
tool calls from a Pi session JSONL file.

For instruction rules: evaluates each file operation against the rule.

For tool-trigger rules: builds V1 trigger context from each tool call
and runs the trigger, reporting matched/notMatched/errors.

V1 trigger context is built from:
  - session id/path/cwd from JSONL header
  - leaf id and message ids from active branch
  - tool call id/name/action/file/command/input
  - repo root and normalized file path
  - rule metadata and hashes`,
		Example: `  # Test an instruction rule against a session
  gsc rules test rule_abc --session /path/to/session.jsonl --format json

  # Test a tool-trigger rule against a session
  gsc rules test rule_def --session /path/to/session.jsonl --format json

  # Test with explicit leaf
  gsc rules test rule_abc --session /path/to/session.jsonl --leaf entry-123 --format json

  # Test with custom timeout for triggers
  gsc rules test rule_abc --session /path/to/session.jsonl --timeout 10000 --format json

  # Test a personal rule
  gsc rules test rule_abc --session /path/to/session.jsonl --scope personal --format json`,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ruleID := args[0]

			if sessionPath == "" {
				return fmt.Errorf("--session is required")
			}

			// Resolve the rule from scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}
			records, err := rulespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return fmt.Errorf("failed to load rules: %w", err)
			}
			sourced, err := rulespkg.ResolveSourcedRecordFromRecords(ruleID, records)
			if err != nil {
				return fmt.Errorf("failed to resolve rule: %w", err)
			}
			if sourced == nil {
				return fmt.Errorf("rule not found in %s scope: %s", scope, ruleID)
			}
			rule := sourced.Rule

			// Resolve leaf
			latestLeafUsed := false
			if leafID == "" {
				leafID, err = sessionspkg.GetLatestLeaf(sessionPath)
				if err != nil {
					return fmt.Errorf("failed to get latest leaf: %w", err)
				}
				latestLeafUsed = true
			}

			// Get file references from the session
			filesResult, err := sessionspkg.ExtractFiles(sessionPath, leafID)
			if err != nil {
				return fmt.Errorf("failed to extract files: %w", err)
			}

			// Branch based on rule type
			if rule.IsExecutable() {
				return runToolTriggerTest(rule, filesResult, leafID, latestLeafUsed, timeout, format)
			}
			return runInstructionTest(rule, filesResult, leafID, latestLeafUsed, format)
		},
	}

	cmd.Flags().StringVar(&sessionPath, "session", "", "Path to session JSONL file (required)")
	cmd.Flags().StringVar(&leafID, "leaf", "", "Leaf entry ID (default: latest)")
	cmd.Flags().StringVarP(&format, "format", "o", "json", "Output format (json)")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Override trigger timeout in milliseconds")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")

	return cmd
}

// TestResult represents the result of testing a rule against a session.
type TestResult struct {
	Rule      TestRuleInfo      `json:"rule"`
	Session   TestSessionInfo   `json:"session"`
	Evaluated int               `json:"evaluated"`
	Matched   []TestMatchResult `json:"matched"`
	NotMatched []TestNotMatchResult `json:"notMatched"`
	Errors    []TestErrorResult `json:"errors,omitempty"`
}

// TestRuleInfo contains rule metadata for the test result.
type TestRuleInfo struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Type        string `json:"type"`
	RuleHash    string `json:"ruleHash"`
	TriggerHash string `json:"triggerHash,omitempty"`
}

// TestSessionInfo contains session metadata for the test result.
type TestSessionInfo struct {
	Path           string `json:"path"`
	Leaf           string `json:"leaf"`
	LatestLeafUsed bool   `json:"latestLeafUsed"`
}

// TestMatchResult represents a tool call that matched the rule.
type TestMatchResult struct {
	ToolCallID        string                    `json:"toolCallId"`
	ToolName          string                    `json:"toolName"`
	Action            string                    `json:"action"`
	Path              *string                   `json:"path"`
	Command           *string                   `json:"command,omitempty"`
	Result            *rulespkg.TriggerResult   `json:"result,omitempty"`
	Instructions      []string                  `json:"instructions,omitempty"`
	WouldBlock        bool                      `json:"wouldBlock"`
	DeliveryKeyPreview string                   `json:"deliveryKeyPreview,omitempty"`
}

// TestNotMatchResult represents a tool call that did not match the rule.
type TestNotMatchResult struct {
	ToolCallID string  `json:"toolCallId"`
	ToolName   string  `json:"toolName"`
	Action     string  `json:"action"`
	Path       *string `json:"path"`
	Command    *string `json:"command,omitempty"`
	Reason     string  `json:"reason"`
}

// TestErrorResult represents an error during trigger execution.
type TestErrorResult struct {
	ToolCallID string `json:"toolCallId"`
	Error      string `json:"error"`
	Timeout    bool   `json:"timeout,omitempty"`
}

// runInstructionTest runs the test for instruction rules.
func runInstructionTest(rule rulespkg.Rule, filesResult *sessionspkg.FilesResult, leafID string, latestLeafUsed bool, format string) error {
	var matched []TestMatchResult
	var notMatched []TestNotMatchResult

	for _, fileRef := range filesResult.Files {
		// Check if action is in rule actions
		if !containsAction(rule.Actions, fileRef.Op) {
			path := fileRef.Path
			notMatched = append(notMatched, TestNotMatchResult{
				ToolCallID: fileRef.ToolCallID,
				ToolName:   opToToolName(fileRef.Op),
				Action:     fileRef.Op,
				Path:       &path,
				Reason:     "action not in rule actions",
			})
			continue
		}

		// Evaluate the rule against the file
		provenance := rulespkg.GetFileMatchProvenance(rule, fileRef.Path)
		if provenance != nil {
			provenance.File = fileRef.Path
			provenance.Action = fileRef.Op

			// Build delivery key preview
			deliveryKey := fmt.Sprintf("static-rule:%s:%s:%s:%s:%s:<context>",
				rule.ID, rule.ComputeRuleHash(), fileRef.Op,
				provenance.Kind, provenance.Value)

			path := fileRef.Path
			matched = append(matched, TestMatchResult{
				ToolCallID:  fileRef.ToolCallID,
				ToolName:    opToToolName(fileRef.Op),
				Action:      fileRef.Op,
				Path:        &path,
				Instructions: rule.Instructions,
				WouldBlock:  true,
				DeliveryKeyPreview: deliveryKey,
			})
		} else {
			path := fileRef.Path
			notMatched = append(notMatched, TestNotMatchResult{
				ToolCallID: fileRef.ToolCallID,
				ToolName:   opToToolName(fileRef.Op),
				Action:     fileRef.Op,
				Path:       &path,
				Reason:     "file/action did not match rule",
			})
		}
	}

	absPath := filesResult.Session.Path
	result := TestResult{
		Rule: TestRuleInfo{
			ID:       rule.ID,
			Summary:  rule.Summary,
			Type:     string(rule.Type),
			RuleHash: rule.ComputeRuleHash(),
		},
		Session: TestSessionInfo{
			Path:           absPath,
			Leaf:           leafID,
			LatestLeafUsed: latestLeafUsed,
		},
		Evaluated:  len(filesResult.Files),
		Matched:    matched,
		NotMatched: notMatched,
	}

	return outputTestResult(result, format)
}

// runToolTriggerTest runs the test for tool-trigger rules.
func runToolTriggerTest(rule rulespkg.Rule, filesResult *sessionspkg.FilesResult, leafID string, latestLeafUsed bool, timeoutMs int, format string) error {
	// Compute trigger hash
	triggerHash := ""
	if th, err := rule.ComputeTriggerHash(); err == nil {
		triggerHash = th
	}

	// Build session context
	sessionID := filesResult.Session.ID
	sessionPath := filesResult.Session.Path
	sessionCWD := filesResult.Session.CWD

	// Build message IDs from the branch
	// For now, we'll use a simplified version - in production this would come from the branch
	messageIDs := []string{leafID}

	// Get model info from session (simplified for V1)
	// In production, this would be extracted from the session data
	modelProvider := ""
	modelID := ""
	thinkingLevel := ""

	// Build rule hash
	ruleHash := rule.ComputeRuleHash()

	var matched []TestMatchResult
	var notMatched []TestNotMatchResult
	var errors []TestErrorResult

	ctx := context.Background()
	if timeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
	}

	for _, fileRef := range filesResult.Files {
		// Build tool call context
		toolCallID := fileRef.ToolCallID
		toolName := opToToolName(fileRef.Op)
		action := fileRef.Op
		file := fileRef.Path
		input := json.RawMessage(`{}`)

		// Build repo context
		repoRoot := filesResult.Session.CWD
		normalizedFile := fileRef.Path

		// Build V1 trigger context
		triggerCtx := rulespkg.BuildV1TriggerContext(
			sessionID, sessionPath, sessionCWD,
			leafID, messageIDs,
			modelProvider, modelID, thinkingLevel,
			toolCallID, toolName, action,
			&file, nil, input,
			&repoRoot, &normalizedFile,
			rule, ruleHash, triggerHash,
		)

		// Run the trigger
		result, err := rulespkg.RunTrigger(ctx, rule, triggerCtx)
		if err != nil {
			isTimeout := false
			if ctx.Err() == context.DeadlineExceeded {
				isTimeout = true
			}
			errors = append(errors, TestErrorResult{
				ToolCallID: toolCallID,
				Error:      err.Error(),
				Timeout:    isTimeout,
			})
			continue
		}

		path := fileRef.Path
		if result.IsMatched() {
			matched = append(matched, TestMatchResult{
				ToolCallID:  toolCallID,
				ToolName:    toolName,
				Action:      action,
				Path:        &path,
				Result:      result,
				WouldBlock:  result.Block,
			})
		} else {
			notMatched = append(notMatched, TestNotMatchResult{
				ToolCallID: toolCallID,
				ToolName:   toolName,
				Action:     action,
				Path:       &path,
				Reason:     "trigger returned matched=false",
			})
		}
	}

	absPath := filesResult.Session.Path
	result := TestResult{
		Rule: TestRuleInfo{
			ID:          rule.ID,
			Summary:     rule.Summary,
			Type:        string(rule.Type),
			RuleHash:    ruleHash,
			TriggerHash: triggerHash,
		},
		Session: TestSessionInfo{
			Path:           absPath,
			Leaf:           leafID,
			LatestLeafUsed: latestLeafUsed,
		},
		Evaluated:  len(filesResult.Files),
		Matched:    matched,
		NotMatched: notMatched,
		Errors:     errors,
	}

	return outputTestResult(result, format)
}

func outputTestResult(result TestResult, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	default:
		printTestHuman(result)
	}
	return nil
}

// opToToolName maps file operations to tool names.
func opToToolName(op string) string {
	switch op {
	case "read":
		return "read"
	case "edit":
		return "edit"
	case "write":
		return "write"
	case "command":
		return "bash"
	default:
		return op
	}
}

// containsAction checks if an action is in the actions list.
func containsAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

func printTestHuman(result TestResult) {
	fmt.Printf("Rule: %s (%s)\n", result.Rule.ID, result.Rule.Summary)
	fmt.Printf("Type: %s\n", result.Rule.Type)
	fmt.Printf("Session: %s\n", result.Session.Path)
	fmt.Printf("Leaf: %s", result.Session.Leaf)
	if result.Session.LatestLeafUsed {
		fmt.Printf(" (latest)")
	}
	fmt.Println()
	fmt.Printf("Evaluated: %d\n\n", result.Evaluated)

	if len(result.Matched) > 0 {
		fmt.Printf("Matched (%d):\n", len(result.Matched))
		for _, m := range result.Matched {
			pathStr := "<nil>"
			if m.Path != nil {
				pathStr = *m.Path
			}
			fmt.Printf("  %s %s %s\n", m.Action, m.ToolName, pathStr)
			if m.Result != nil {
				fmt.Printf("    Block: %v\n", m.Result.Block)
				if m.Result.Message != "" {
					fmt.Printf("    Message: %s\n", m.Result.Message)
				}
				if m.Result.Notice != "" {
					fmt.Printf("    Notice: %s\n", m.Result.Notice)
				}
			}
			if len(m.Instructions) > 0 {
				for _, inst := range m.Instructions {
					fmt.Printf("    - %s\n", inst)
				}
			}
		}
		fmt.Println()
	}

	if len(result.NotMatched) > 0 {
		fmt.Printf("Not Matched (%d):\n", len(result.NotMatched))
		for _, nm := range result.NotMatched {
			pathStr := "<nil>"
			if nm.Path != nil {
				pathStr = *nm.Path
			}
			fmt.Printf("  %s %s %s -> %s\n", nm.Action, nm.ToolName, pathStr, nm.Reason)
		}
		fmt.Println()
	}

	if len(result.Errors) > 0 {
		fmt.Printf("Errors (%d):\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  %s: %s\n", e.ToolCallID, e.Error)
		}
	}
}
