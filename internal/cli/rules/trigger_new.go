/**
 * Component: Rules Trigger New Command
 * Block-UUID: 2b3c4d5e-6f7a-8901-bcde-f12345678901
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc rules trigger new for creating tool-trigger rules with V1 executable trigger contract.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func triggerNewCmd() *cobra.Command {
	var (
		title       string
		event       string
		runtime     string
		entry       string
		timeoutMs   int
		instruction string
		query       string
		frequency   string
		priority    int
		topic       string
		tags        []string
		fromFile    string
		useStdin    bool
		targetValue string
		creator     string
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new tool-trigger rule",
		Long: `Create a new tool-trigger rule with an executable trigger.

The trigger is an executable file that receives context on stdin and returns
a JSON result on stdout indicating whether to block/inject knowledge.

V1 Executable Trigger Contract:
  - Input: JSON on stdin with version, session, conversation, toolCall, repo, rule
  - Output: JSON on stdout with matched, block, message?, notice?
  - cwd = repo.root when repo is known, otherwise session.cwd

Supported runtimes: node, python, bash`,
		Example: `  # Create a node trigger for editing a specific file
  gsc rules trigger new \
    --title "Foo edit policy" \
    --runtime node \
    --entry foo-edit.mjs \
    --instruction "Run gsc rules get --file foo.bar before editing" \
    --topic file-knowledge

  # Create a trigger from a JSON definition
  gsc rules trigger new --from-file trigger-def.json

  # Create a python trigger with a knowledge query
  gsc rules trigger new \
    --title "API conventions" \
    --runtime python \
    --entry api-check.py \
    --query "gsc knowledge search api conventions" \
    --topic api

  # Create a bash trigger
  gsc rules trigger new \
    --title "Unsafe command blocker" \
    --runtime bash \
    --entry unsafe-cmd.sh \
    --instruction "Do not run rm -rf without confirmation" \
    --topic safety

  # Create a user_prompt_submit trigger
  gsc rules trigger new \
    --title "Block prompts containing hello" \
    --event user_prompt_submit \
    --runtime node \
    --entry block-hello.mjs \
    --instruction "Blocks prompts containing hello" \
    --topic agent-lifecycle-rules`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCreatorFlag(creator); err != nil {
				return err
			}
			if isAgentCreator(creator) && fromFile == "" && !useStdin {
				return fmt.Errorf("--creator agent requires --from-file or --stdin with structured trigger JSON")
			}
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			var rule rulespkg.Rule

			if fromFile != "" || useStdin {
				var data []byte
				var err error
				if useStdin {
					data, err = io.ReadAll(cmd.InOrStdin())
				} else {
					data, err = os.ReadFile(fromFile)
				}
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				if err := json.Unmarshal(data, &rule); err != nil {
					return fmt.Errorf("invalid JSON: %w", err)
				}
			} else {
				// Build from flags
				if title == "" {
					return fmt.Errorf("--title is required")
				}
				if runtime == "" {
					return fmt.Errorf("--runtime is required (node, python, bash)")
				}
				if entry == "" {
					return fmt.Errorf("--entry is required")
				}

				// Validate runtime
				if !rulespkg.IsValidRuntime(runtime) {
					return fmt.Errorf("unsupported runtime: %s (must be one of: node, python, bash)", runtime)
				}

				// Validate entry doesn't contain ..
				if filepath.Clean(entry) != entry || len(entry) > 0 && entry[0] == '/' {
					return fmt.Errorf("--entry must be a relative path without ..")
				}

				// Build instruction config
				instrMode := "inline"
				instrText := instruction
				instrQuery := ""
				if query != "" {
					instrMode = "query"
					instrText = ""
					instrQuery = query
				}

				// Determine frequency mode
				freqMode := rulespkg.FrequencyOncePerContext
				if frequency != "" {
					freqMode = rulespkg.FrequencyMode(frequency)
				}

				// Build trigger config
				triggerConfig := &rulespkg.TriggerConfig{
					Runtime:   runtime,
					Entry:     entry,
					TimeoutMs: timeoutMs,
				}

				rule = rulespkg.Rule{
					Summary: title,
					Event:   rulespkg.LifecycleEvent(event),
					Topic:   topic,
					Tags:    tags,
					Trigger: triggerConfig,
					Frequency: &rulespkg.FrequencyConfig{
						Mode: freqMode,
					},
					Priority: priority,
				}

				// Add instruction config if provided
				if instrText != "" || instrQuery != "" {
					rule.InstrCfg = &rulespkg.InstructionConfig{
						Mode:  instrMode,
						Text:  instrText,
						Query: instrQuery,
					}
				}
			}

			// Set type and defaults
			rule.Type = rulespkg.RuleTypeExecutable
			rule.SchemaVersion = "3.0.0"
			rule.Enabled = boolPtr(true)

			// Generate ID and timestamps
			now := time.Now().UTC()
			id, err := rulespkg.NewRuleID(now)
			if err != nil {
				return fmt.Errorf("failed to generate rule ID: %w", err)
			}
			rule.ID = id
			rule.CreatedAt = now
			rule.UpdatedAt = now

			// Normalize
			normalized := rulespkg.ValidateAndNormalize(rule)
			if !normalized.Valid() {
				fmt.Println("Rule content is invalid; nothing committed:")
				for _, err := range normalized.Errors {
					fmt.Printf("  ERROR %s\n", err)
				}
				return fmt.Errorf("rule content is invalid")
			}
			if isAgentCreator(creator) {
				if err := validateAndStripAgentChecklist(&normalized.Rule, target); err != nil {
					fmt.Println("Agent creator checklist is invalid; nothing committed:")
					printAgentChecklistErrors(err)
					return err
				}
			}
			rule = normalized.Rule

			// Compute keywords
			rule.Keywords = rulespkg.KeywordsFor(rule)
			rule.ParentKeywords = rulespkg.ParentKeywordsFor(rule)

			// Validate trigger entry against target source
			source := rulespkg.SourceFromTarget(target)
			if rule.Trigger != nil {
				if errs := rulespkg.ValidateTriggerFileWithSource(rule.Trigger.Entry, rule.Trigger.Runtime, source); len(errs) > 0 {
					fmt.Println("Trigger validation failed:")
					for _, e := range errs {
						fmt.Printf("  ERROR %s\n", e)
					}
					return fmt.Errorf("trigger validation failed")
				}
			}

			// Commit
			if err := rulespkg.AppendRecordToTarget(rule, target); err != nil {
				return fmt.Errorf("failed to commit rule: %w", err)
			}

			// Rebuild Brain
			if err := rulespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}

			// Show destination
			recordsPath, _ := rulespkg.RecordsPathForTarget(target)
			fmt.Printf("Rule written to %s scope: %s\n", target, recordsPath)
			fmt.Printf("Tool-trigger rule committed: %s\n", rule.ID)
			fmt.Printf("Summary: %s\n", rule.Summary)
			if rule.Trigger != nil {
				fmt.Printf("Runtime: %s\n", rule.Trigger.Runtime)
				fmt.Printf("Entry: %s\n", rule.Trigger.Entry)
			}
			if rule.InstrCfg != nil {
				fmt.Printf("Instruction mode: %s\n", rule.InstrCfg.Mode)
			}
			fmt.Printf("Frequency: %s\n", rule.Frequency.Mode)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Rule title/summary (required)")
	cmd.Flags().StringVar(&event, "event", "", "Lifecycle event: session_start, before_agent_start, user_prompt_submit, pre_tool_use, post_tool_use, post_tool_batch, agent_end, session_end")
	cmd.Flags().StringVar(&runtime, "runtime", "", "Trigger runtime: node, python, bash (required)")
	cmd.Flags().StringVar(&entry, "entry", "", "Trigger file entry relative to .gitsense/rules/triggers/ (required)")
	cmd.Flags().IntVar(&timeoutMs, "timeout", 5000, "Trigger timeout in milliseconds (max 60000)")
	cmd.Flags().StringVar(&instruction, "instruction", "", "Inline instruction text (optional fallback message)")
	cmd.Flags().StringVar(&query, "query", "", "Knowledge query to run")
	cmd.Flags().StringVar(&frequency, "frequency", "once-per-context", "Frequency mode: always, once-per-turn, once-per-context, once-per-session, once-per-branch, once-per-file, once-per-rule-hash")
	cmd.Flags().IntVar(&priority, "priority", 0, "Priority (higher = executed first)")
	cmd.Flags().StringVar(&topic, "topic", "", "Primary topic slug (required)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag slug (repeatable)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read trigger definition from JSON file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read trigger definition from JSON on stdin")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	cmd.Flags().StringVar(&creator, "creator", "", "Creator type: agent or human. Agent-created triggers require structured JSON with creatorChecklist.")

	return cmd
}

func boolPtr(b bool) *bool {
	return &b
}
