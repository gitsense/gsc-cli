/**
 * Component: Rules Trigger Validate Command
 * Block-UUID: 4d5e6f7a-8b9c-0123-defa-345678901234
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc rules trigger validate for validating tool-trigger rules with V1 executable trigger contract.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func triggerValidateCmd() *cobra.Command {
	var (
		all         bool
		contextFile string
		path        string
		scopeValue  string
	)

	cmd := &cobra.Command{
		Use:   "validate [rule-id]",
		Short: "Validate tool-trigger rules",
		Long: `Validate tool-trigger rules by checking schema, trigger file existence,
and optionally running the trigger against a fixture context.

Validation checks:
  - Rule schema is valid
  - Trigger file exists and has correct extension
  - Trigger runtime is supported
  - If context provided: trigger executes within timeout
  - If context provided: stdout is valid JSON matching V1 schema
  - block=true has either returned message or stored instruction
  - Frequency mode is known
  - Referenced query/instruction exists if required

V1 trigger output schema:
  - matched: boolean (required)
  - block: boolean (required)
  - message: string (required when block=true)
  - notice: string (optional, user-facing)`,
		Example: `  # Validate a specific rule
  gsc rules trigger validate rule_abc123

  # Validate with a fixture context
  gsc rules trigger validate rule_abc123 --context fixture.json

  # Validate all tool-trigger rules
  gsc rules trigger validate --all

  # Validate a personal rule
  gsc rules trigger validate rule_abc123 --scope personal

  # Validate a trigger file directly
  gsc rules trigger validate-path .gitsense/rules/triggers/my-trigger.mjs`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path != "" {
				return validateTriggerPath(path, contextFile)
			}

			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			if all {
				return validateAllTriggers(contextFile, scope)
			}

			if len(args) == 0 {
				return fmt.Errorf("rule ID is required (or use --all or validate-path)")
			}

			return validateSingleTrigger(args[0], contextFile, scope)
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Validate all tool-trigger rules")
	cmd.Flags().StringVar(&contextFile, "context", "", "Path to fixture context JSON file")
	cmd.Flags().StringVar(&path, "path", "", "Validate a trigger file directly (instead of rule ID)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")

	return cmd
}

func validateSingleTrigger(ruleID string, contextFile string, scope gitsensescope.Scope) error {
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

	// Run schema validation
	errs := rulespkg.ValidateRule(rule)
	if len(errs) > 0 {
		fmt.Printf("Rule %s has validation errors:\n", ruleID)
		for _, err := range errs {
			fmt.Printf("  ERROR %s\n", err)
		}
		return fmt.Errorf("validation failed")
	}

	// If context provided, run the trigger
	if contextFile != "" {
		return validateWithContext(rule, contextFile)
	}

	fmt.Printf("Rule %s: schema valid\n", ruleID)
	fmt.Printf("Trigger: %s\n", rule.Trigger.Entry)
	fmt.Printf("Instruction mode: %s\n", rule.InstrCfg.Mode)
	fmt.Printf("Frequency: %s\n", rule.Frequency.Mode)

	// Check trigger file exists
	triggerErrs := rulespkg.ValidateTriggerFile(rule.Trigger.Entry, rule.Trigger.Runtime)
	if len(triggerErrs) > 0 {
		fmt.Printf("\nTrigger file errors:\n")
		for _, err := range(triggerErrs) {
			fmt.Printf("  ERROR %s\n", err)
		}
		return fmt.Errorf("trigger file validation failed")
	}

	fmt.Printf("Trigger file: OK\n")
	return nil
}

func validateAllTriggers(contextFile string, scope gitsensescope.Scope) error {
	records, err := rulespkg.LoadRecordsFromScope(scope)
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	var triggerRules []rulespkg.Rule
	for _, r := range records {
		if r.Rule.IsExecutable() {
			triggerRules = append(triggerRules, r.Rule)
		}
	}

	if len(triggerRules) == 0 {
		fmt.Println("No tool-trigger rules found.")
		return nil
	}

	fmt.Printf("Validating %d tool-trigger rules...\n\n", len(triggerRules))

	allValid := true
	for _, rule := range triggerRules {
		fmt.Printf("--- Rule: %s ---\n", rule.ID)
		fmt.Printf("Summary: %s\n", rule.Summary)

		errs := rulespkg.ValidateRule(rule)
		if len(errs) > 0 {
			allValid = false
			fmt.Printf("Schema errors:\n")
			for _, err := range errs {
				fmt.Printf("  ERROR %s\n", err)
			}
		} else {
			fmt.Printf("Schema: OK\n")
		}

		triggerErrs := rulespkg.ValidateTriggerFile(rule.Trigger.Entry, rule.Trigger.Runtime)
		if len(triggerErrs) > 0 {
			allValid = false
			fmt.Printf("Trigger file errors:\n")
			for _, err := range(triggerErrs) {
				fmt.Printf("  ERROR %s\n", err)
			}
		} else {
			fmt.Printf("Trigger file: OK\n")
		}

		if contextFile != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			fixtureCtx, err := loadV1FixtureContext(contextFile)
			if err != nil {
				fmt.Printf("Context load error: %v\n", err)
			} else {
				validateErrs := rulespkg.ValidateTriggerWithContext(ctx, rule, *fixtureCtx)
				if len(validateErrs) > 0 {
					allValid = false
					fmt.Printf("Execution errors:\n")
					for _, err := range validateErrs {
						fmt.Printf("  ERROR %s\n", err)
					}
				} else {
					fmt.Printf("Execution: OK\n")
				}
			}
		}

		fmt.Println()
	}

	if allValid {
		fmt.Println("All tool-trigger rules are valid.")
	} else {
		return fmt.Errorf("some rules have validation errors")
	}

	return nil
}

func validateTriggerPath(path string, contextFile string) error {
	// This validates a trigger file directly, not tied to a rule
	fmt.Printf("Validating trigger file: %s\n", path)

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("trigger file does not exist: %s", absPath)
	}

	// Check extension
	ext := filepath.Ext(absPath)
	if ext != ".mjs" && ext != ".js" {
		return fmt.Errorf("trigger file must have .mjs or .js extension, got: %s", ext)
	}

	if contextFile != "" {
		// Create a temporary rule for validation
		rule := rulespkg.Rule{
			Type: rulespkg.RuleTypeExecutable,
			Trigger: &rulespkg.TriggerConfig{
				Runtime: "node",
				Entry:   absPath,
			},
			InstrCfg: &rulespkg.InstructionConfig{
				Mode: "inline",
				Text: "Test instruction",
			},
			Frequency: &rulespkg.FrequencyConfig{
				Mode: rulespkg.FrequencyAlways,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fixtureCtx, err := loadV1FixtureContext(contextFile)
		if err != nil {
			return fmt.Errorf("failed to load fixture context: %w", err)
		}

		errs := rulespkg.ValidateTriggerWithContext(ctx, rule, *fixtureCtx)
		if len(errs) > 0 {
			fmt.Printf("Validation errors:\n")
			for _, err := range errs {
				fmt.Printf("  ERROR %s\n", err)
			}
			return fmt.Errorf("validation failed")
		}

		fmt.Println("Trigger execution: OK")
	} else {
		fmt.Println("Trigger file exists and has correct extension.")
		fmt.Println("Use --context to run the trigger against a fixture.")
	}

	return nil
}

func validateWithContext(rule rulespkg.Rule, contextFile string) error {
	fixtureCtx, err := loadV1FixtureContext(contextFile)
	if err != nil {
		return fmt.Errorf("failed to read fixture context: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errs := rulespkg.ValidateTriggerWithContext(ctx, rule, *fixtureCtx)
	if len(errs) > 0 {
		fmt.Printf("Execution errors:\n")
		for _, err := range errs {
			fmt.Printf("  ERROR %s\n", err)
		}
		return fmt.Errorf("validation failed")
	}

	fmt.Println("Trigger execution: OK")
	return nil
}

func loadV1FixtureContext(path string) (*rulespkg.V1TriggerContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read fixture file: %w", err)
	}

	var ctx rulespkg.V1TriggerContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("invalid fixture JSON: %w", err)
	}

	// Ensure version
	if ctx.Version == "" {
		ctx.Version = "1"
	}

	return &ctx, nil
}
