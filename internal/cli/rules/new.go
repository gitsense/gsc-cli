/**
 * Component: Rules New Command
 * Block-UUID: 3c4d5e6f-7a8b-9012-cdef-234567890abc
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules new, the unified command for creating both declarative and executable rules.
 * Language: Go
 * Created-at: 2026-06-25T20:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

const ruleTemplateDeclarative = `{
  "glob_patterns": ["path/to/files/**"],
  "exclude_globs": ["path/to/files/legacy/**"],
  "summary": "Short description of the rule",
  "details": "Optional longer description",
  "event": "pre_tool_use",
  "instructions": [
    "First instruction",
    "Second instruction"
  ],
  "actions": ["edit", "write"],
  "importance": "medium",
  "owner": "team-or-person",
  "contact": ["email@example.com"],
  "tags": ["category"],
  "applies_to": {
    "files": ["specific/file.go"],
    "linked_files": ["related/file.go"],
    "commands": ["gsc rules new"],
    "topics": ["topic-slug"]
  }
}
`

const ruleTemplateExecutable = `{
  "type": "executable",
  "summary": "Short description of the rule",
  "details": "Optional longer description",
  "event": "pre_tool_use",
  "trigger": {
    "runtime": "node",
    "entry": "trigger-file.mjs",
    "timeoutMs": 5000
  },
  "instruction": {
    "mode": "inline",
    "text": "Instruction text for the trigger"
  },
  "frequency": {
    "mode": "always"
  },
  "priority": 10,
  "enabled": true,
  "topic": "topic-slug",
  "tags": ["category"]
}
`

func newCmd() *cobra.Command {
	var (
		// Common flags
		ruleType      string
		summary       string
		details       string
		event         string
		importance    string
		topic         string
		relatedTopics []string
		owner         string
		contact       []string
		tags          []string
		showTemplate  bool
		fromFile      string
		useStdin      bool
		targetValue   string
		creator       string

		// Declarative-specific flags
		instructions    []string
		actions         []string
		globs           []string
		excludeGlobs    []string
		files           []string
		linkedFiles     []string
		commands        []string
		toolFilter      string
		commandFilter   string
		caseInsensitive bool

		// Executable-specific flags
		runtime   string
		entry     string
		timeoutMs int
		mode      string
		text      string
		query     string
		frequency string
		priority  int
		enabled   bool
	)

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new rule (declarative or executable)",
		Long: `Create a new rule without hand-editing a draft file.

Two rule types are supported:

1. Declarative rules (--type declarative, default):
   - Match by criteria (file, glob, action, event)
   - No executable code
   - Advisory instructions for agents

2. Executable rules (--type executable):
   - Has code (node/python/bash) that evaluates runtime context
   - Can block actions or inject knowledge
   - Receives context on stdin, returns JSON on stdout

Provide the content one of three ways:
  --template           Print a rule template and exit
  --from-file <path>   Rule-shaped JSON
  --stdin              Rule-shaped JSON on stdin
  individual flags     --summary/--instruction/--action/--matches/--tool/...`,
		Example: `  # Print a declarative rule template
  gsc rules new --template

  # Print an executable rule template
  gsc rules new --type executable --template

  # Create a declarative rule (default type)
  gsc rules new --event pre_tool_use --action bash --matches "rm -rf" \
    --summary "Block rm -rf commands" \
    --instruction "Block destructive shell commands" \
    --topic safety

  # Create a declarative rule matching all commands
  gsc rules new --event pre_tool_use --action bash --matches ".*" \
    --summary "Log all commands" \
    --instruction "Log command for audit" \
    --topic audit

  # Create an executable rule
  gsc rules new --type executable \
    --event before_agent_start \
    --summary "Block dangerous bash" \
    --runtime node \
    --entry block-dangerous-bash.mjs \
    --instruction "Block destructive shell commands" \
    --topic safety

  # Create from JSON file
  gsc rules new --from-file /tmp/rule.json

  # Create from stdin
  cat rule.json | gsc rules new --stdin`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle template
			if showTemplate {
				if err := printRuleTemplate(cmd.OutOrStdout(), ruleType); err != nil {
					return err
				}
				return nil
			}

			// Validate rule type
			if ruleType != "" && ruleType != "declarative" && ruleType != "executable" {
				return fmt.Errorf("invalid rule type %q; must be one of: declarative, executable", ruleType)
			}
			if err := validateCreatorFlag(creator); err != nil {
				return err
			}
			if isAgentCreator(creator) && fromFile == "" && !useStdin {
				return fmt.Errorf("--creator agent requires --from-file or --stdin with structured rule JSON")
			}

			// Default to declarative
			if ruleType == "" {
				ruleType = "declarative"
			}

			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			var rule rulespkg.Rule
			if fromFile != "" || useStdin {
				// JSON mode
				var data []byte
				if useStdin {
					data, err = io.ReadAll(os.Stdin)
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
				// Flag mode
				if importance == "" {
					importance = "medium"
				}

				switch ruleType {
				case "declarative":
					// Validate required flags for declarative
					if summary == "" {
						return fmt.Errorf("--summary is required for declarative rules")
					}
					if topic == "" {
						return fmt.Errorf("--topic is required")
					}
					if len(instructions) == 0 {
						return fmt.Errorf("--instruction is required for declarative rules")
					}
					if len(actions) == 0 {
						return fmt.Errorf("--action is required for declarative rules")
					}

					// Prepend (?i) to command filter if case-insensitive
					if caseInsensitive && commandFilter != "" {
						commandFilter = "(?i)" + commandFilter
					}

					// Determine which filter to use based on action
					var promptFilter string
					var promptFilterIgnoreCase bool
					hasPromptAction := false
					for _, a := range actions {
						if a == "prompt" {
							hasPromptAction = true
							break
						}
					}
					if hasPromptAction && commandFilter != "" {
						// For prompt actions, use prompt_filter instead of command_filter
						promptFilter = commandFilter
						promptFilterIgnoreCase = caseInsensitive
						commandFilter = "" // Clear command_filter
					}

					rule = rulespkg.Rule{
						Type:                   rulespkg.RuleTypeDeclarative,
						Summary:                summary,
						Details:                details,
						Event:                  rulespkg.LifecycleEvent(event),
						Topic:                  topic,
						RelatedTopics:          relatedTopics,
						Importance:             importance,
						Owner:                  owner,
						Contact:                contact,
						GlobPatterns:           globs,
						ExcludeGlobs:           excludeGlobs,
						Instructions:           instructions,
						Actions:                actions,
						ToolFilter:             toolFilter,
						CommandFilter:          commandFilter,
						PromptFilter:           promptFilter,
						PromptFilterIgnoreCase: promptFilterIgnoreCase,
						Tags:                   tags,
						AppliesTo: rulespkg.AppliesTo{
							Files:       files,
							LinkedFiles: linkedFiles,
							Commands:    commands,
						},
					}

				case "executable":
					// Validate required flags for executable
					if summary == "" {
						return fmt.Errorf("--summary is required for executable rules")
					}
					if topic == "" {
						return fmt.Errorf("--topic is required")
					}
					if runtime == "" {
						return fmt.Errorf("--runtime is required for executable rules")
					}
					if entry == "" {
						return fmt.Errorf("--entry is required for executable rules")
					}
					if text == "" && query == "" {
						return fmt.Errorf("--text or --query is required for executable rules")
					}
					if frequency == "" {
						frequency = "always"
					}

					// Build instruction config
					instrCfg := &rulespkg.InstructionConfig{}
					if text != "" {
						instrCfg.Mode = "inline"
						instrCfg.Text = text
					} else if query != "" {
						instrCfg.Mode = "query"
						instrCfg.Query = query
					}

					rule = rulespkg.Rule{
						Type:          rulespkg.RuleTypeExecutable,
						Summary:       summary,
						Details:       details,
						Event:         rulespkg.LifecycleEvent(event),
						Topic:         topic,
						RelatedTopics: relatedTopics,
						Importance:    importance,
						Owner:         owner,
						Contact:       contact,
						Tags:          tags,
						Trigger: &rulespkg.TriggerConfig{
							Runtime:   runtime,
							Entry:     entry,
							TimeoutMs: timeoutMs,
						},
						InstrCfg: instrCfg,
						Frequency: &rulespkg.FrequencyConfig{
							Mode: rulespkg.FrequencyMode(frequency),
						},
						Priority: priority,
						Enabled:  &enabled,
					}
				}
			}

			// Validate and normalize
			result := rulespkg.ValidateAndNormalize(rule)
			if !result.Valid() {
				fmt.Println("Rule content is invalid; nothing committed:")
				for _, err := range result.Errors {
					fmt.Printf("  ERROR %s\n", err)
				}
				return fmt.Errorf("rule content is invalid")
			}
			if isAgentCreator(creator) {
				if err := validateAndStripAgentChecklist(&result.Rule, target); err != nil {
					fmt.Println("Agent creator checklist is invalid; nothing committed:")
					printAgentChecklistErrors(err)
					return err
				}
			}

			// Generate ID and timestamps
			now := time.Now().UTC()
			id, err := rulespkg.NewRuleID(now)
			if err != nil {
				return fmt.Errorf("failed to generate rule ID: %w", err)
			}

			rule = result.Rule
			rule.ID = id
			rule.SchemaVersion = "2.0.0"
			rule.CreatedAt = now
			rule.UpdatedAt = now
			rule.Keywords = rulespkg.KeywordsFor(rule)
			rule.ParentKeywords = rulespkg.ParentKeywordsFor(rule)

			// Commit
			if err := rulespkg.AppendRecordToTarget(rule, target); err != nil {
				return fmt.Errorf("failed to commit rule: %w", err)
			}

			// Rebuild the Brain
			if err := rulespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}

			// Show destination
			recordsPath, _ := rulespkg.RecordsPathForTarget(target)
			fmt.Printf("Rule written to %s scope: %s\n", target, recordsPath)
			fmt.Printf("Rule committed: %s\n", rule.ID)
			fmt.Printf("Summary: %s\n", rule.Summary)
			fmt.Printf("Instructions: %d\n", len(rule.Instructions))
			if len(rule.Tags) > 0 {
				fmt.Printf("Tags: %s\n", rule.Tags)
			}
			return nil
		},
	}

	// Common flags
	cmd.Flags().StringVar(&ruleType, "type", "", "Rule type: declarative (default), executable")
	cmd.Flags().StringVar(&summary, "summary", "", "Rule summary (required)")
	cmd.Flags().StringVar(&details, "details", "", "Rule details")
	cmd.Flags().StringVar(&event, "event", "", "Lifecycle event (session_start, before_agent_start, user_prompt_submit, agent_start, pre_tool_use, post_tool_use, post_tool_batch, context, session_before_compact, session_compact, agent_end, session_end)")
	cmd.Flags().StringVar(&importance, "importance", "", "Importance: low, medium, or high (default medium)")
	cmd.Flags().StringVar(&topic, "topic", "", "Primary topic slug (required)")
	cmd.Flags().StringArrayVar(&relatedTopics, "related-topic", nil, "Related topic slug (max 2, repeatable)")
	cmd.Flags().StringVar(&owner, "owner", "", "Rule owner")
	cmd.Flags().StringArrayVar(&contact, "contact", nil, "Contact (repeatable)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag slug (repeatable)")
	cmd.Flags().BoolVar(&showTemplate, "template", false, "Print a rule template and exit")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read rule-shaped JSON content from a file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read rule-shaped JSON content from stdin")
	cmd.Flags().StringVar(&creator, "creator", "", "Creator type: agent or human. Agent-created rules require structured JSON with creatorChecklist.")

	// Declarative-specific flags
	cmd.Flags().StringArrayVar(&instructions, "instruction", nil, "Instruction text (repeatable)")
	cmd.Flags().StringArrayVar(&actions, "action", nil, "Action (repeatable: read, write, edit, bash, tool, mcp_tool, prompt, agent_end)")
	cmd.Flags().StringArrayVar(&globs, "glob", nil, "Glob pattern (repeatable)")
	cmd.Flags().StringArrayVar(&excludeGlobs, "exclude-glob", nil, "Exclude glob pattern (repeatable)")
	cmd.Flags().StringArrayVar(&files, "file", nil, "Repo-relative file this rule applies to (repeatable)")
	cmd.Flags().StringArrayVar(&linkedFiles, "linked-file", nil, "Repo-relative related file (repeatable)")
	cmd.Flags().StringArrayVar(&commands, "command", nil, "Command this rule applies to (repeatable)")
	cmd.Flags().StringVar(&toolFilter, "tool", "", "Tool name filter (glob pattern, e.g., github.*)")
	cmd.Flags().StringVar(&commandFilter, "matches", "", "Bash command filter (regex pattern, e.g., rm -rf|chmod -R)")
	cmd.Flags().BoolVar(&caseInsensitive, "case-insensitive", false, "Make --matches pattern case-insensitive")

	// Executable-specific flags
	cmd.Flags().StringVar(&runtime, "runtime", "", "Trigger runtime: node, python, bash")
	cmd.Flags().StringVar(&entry, "entry", "", "Trigger file entry relative to .gitsense/rules/triggers/")
	cmd.Flags().IntVar(&timeoutMs, "timeout", 5000, "Trigger timeout in milliseconds (max 60000)")
	cmd.Flags().StringVar(&mode, "mode", "inline", "Instruction mode: inline, query")
	cmd.Flags().StringVar(&text, "text", "", "Inline instruction text")
	cmd.Flags().StringVar(&query, "query", "", "Knowledge query to run")
	cmd.Flags().StringVar(&frequency, "frequency", "always", "Frequency mode: always, once-per-turn, once-per-context, once-per-session, once-per-branch, once-per-file, once-per-rule-hash")
	cmd.Flags().IntVar(&priority, "priority", 0, "Priority (higher = executed first)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Enable/disable rule")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")

	return cmd
}

func printRuleTemplate(w io.Writer, ruleType string) error {
	switch ruleType {
	case "", "declarative":
		_, _ = fmt.Fprint(w, ruleTemplateDeclarative)
	case "executable":
		_, _ = fmt.Fprint(w, ruleTemplateExecutable)
	default:
		return fmt.Errorf("invalid rule type %q; must be one of: declarative, executable", ruleType)
	}
	return nil
}
