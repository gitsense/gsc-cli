/**
 * Component: Rules Trigger Run Command
 * Block-UUID: 5e6f7a8b-9c0d-1234-efab-456789012345
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc rules trigger run for executing tool-trigger rules with V1 executable trigger contract.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package rules

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func triggerRunCmd() *cobra.Command {
	var (
		all         bool
		contextFile string
		timeout     int
		scopeValue  string
	)

	cmd := &cobra.Command{
		Use:   "run [rule-id]",
		Short: "Execute tool-trigger rules",
		Long: `Execute tool-trigger rules against a context and return matched results.

This command is designed for agent integrations like pi-brains to evaluate
tool-trigger rules at runtime. It runs each trigger independently and returns
an aggregate result with matched rules and any errors.

The context JSON must follow the V1 executable trigger contract:
  - version: "1"
  - session: { id, path, cwd }
  - conversation: { leafId, messageIds }
  - toolCall: { id, toolName, action, file?, command?, input }
  - repo?: { root, normalizedFile? }
  - rule: { id, summary, type, ruleHash, triggerHash }

The trigger receives context on stdin and returns JSON on stdout:
  - matched: boolean (required)
  - block: boolean (required)
  - message: string (required when block=true)
  - notice: string (optional, user-facing)`,
		Example: `  # Run a specific trigger
  gsc rules trigger run rule_abc123 --context context.json

  # Run all enabled triggers
  gsc rules trigger run --all --context context.json

  # Run with custom timeout
  gsc rules trigger run --all --context context.json --timeout 10000

  # Run a personal trigger
  gsc rules trigger run rule_abc123 --context context.json --scope personal`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if contextFile == "" {
				return fmt.Errorf("--context is required")
			}

			// Load the context
			contextData, err := os.ReadFile(contextFile)
			if err != nil {
				return fmt.Errorf("failed to read context file: %w", err)
			}

			var triggerCtx rulespkg.V1TriggerContext
			if err := json.Unmarshal(contextData, &triggerCtx); err != nil {
				return fmt.Errorf("invalid context JSON: %w", err)
			}

			// Ensure version
			if triggerCtx.Version == "" {
				triggerCtx.Version = "1"
			}

			ctx := context.Background()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
				defer cancel()
			}

			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			if all {
				return runAllTriggers(ctx, triggerCtx, scope)
			}

			if len(args) == 0 {
				return fmt.Errorf("rule ID is required (or use --all)")
			}

			return runSingleTrigger(ctx, args[0], triggerCtx, scope)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Run all enabled tool-trigger rules")
	cmd.Flags().StringVar(&contextFile, "context", "", "Path to trigger context JSON file (required)")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Override timeout in milliseconds")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")

	return cmd
}

func runSingleTrigger(ctx context.Context, ruleID string, triggerCtx rulespkg.V1TriggerContext, scope gitsensescope.Scope) error {
	// Resolve the rule from scope
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

	if !rule.IsExecutable() {
		return fmt.Errorf("rule %s is not a tool-trigger rule (type: %s)", ruleID, rule.Type)
	}

	if !rule.IsEnabled() {
		return fmt.Errorf("rule %s is disabled", ruleID)
	}

	// Use provided context or create one with timeout
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(rule.EffectiveTimeoutMs())*time.Millisecond)
		defer cancel()
	}

	// Compute trigger hash
	triggerPath, err := rulespkg.TriggerPath(rule.Trigger.Entry)
	if err != nil {
		return fmt.Errorf("failed to resolve trigger path: %w", err)
	}
	triggerHash, err := computeTriggerHash(triggerPath)
	if err != nil {
		return fmt.Errorf("failed to compute trigger hash: %w", err)
	}

	// Update rule context with hash
	triggerCtx.Rule.TriggerHash = triggerHash
	triggerCtx.Rule.RuleHash = rule.ComputeRuleHash()

	startTime := time.Now()
	result, err := rulespkg.RunTrigger(ctx, rule, triggerCtx)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		return fmt.Errorf("trigger failed: %w", err)
	}

	// Output the result
	output := map[string]interface{}{
		"ruleId":  rule.ID,
		"matched": result.IsMatched(),
		"block":   result.Block,
		"message": rule.GetInstructionText(result.Message),
		"notice":  result.Notice,
		"frequency": map[string]interface{}{
			"mode": rule.Frequency.Mode,
			"key":  result.FrequencyKey,
		},
		"priority":   rule.Priority,
		"ruleHash":   rule.ComputeRuleHash(),
		"durationMs": durationMs,
	}

	// Include deliveryMode if set
	if result.DeliveryMode != "" {
		output["deliveryMode"] = result.DeliveryMode
	}

	jsonOutput, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(jsonOutput))
	return nil
}

func runAllTriggers(ctx context.Context, triggerCtx rulespkg.V1TriggerContext, scope gitsensescope.Scope) error {
	result, err := rulespkg.RunAllTriggers(ctx, triggerCtx)
	if err != nil {
		return fmt.Errorf("failed to run triggers: %w", err)
	}

	jsonOutput, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(jsonOutput))
	return nil
}

func computeTriggerHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h), nil
}
