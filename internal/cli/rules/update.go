/**
 * Component: Rules Update Command
 * Block-UUID: b0c1d2e3-f4a5-6789-bcde-789012345678
 * Parent-UUID: N/A
 * Version: 2.1.0
 * Description: Implements gsc rules update to replace an existing rule with new content. Requires --changelog flag. Instructions simplified to strings, actions required. Supports lifecycle event binding.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Updated-at: 2026-06-24T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0), MiMo-v2.5-pro (v2.0.0), terrchen (v2.1.0)
 * Changelog:
 *   v2.1.0 - Add --event flag for lifecycle event binding
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

func updateCmd() *cobra.Command {
	var (
		id           string
		fromFile     string
		useStdin     bool
		summary      string
		details      string
		event        string
		importance   string
		owner        string
		contact      []string
		globs        []string
		excludeGlobs []string
		files        []string
		linkedFiles  []string
		commands     []string
		topics       []string
		tags         []string
		actions      []string
		changelog    string
		targetValue  string
		creator      string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing rule",
		Long: `Update an existing rule with new content.

Provide the rule ID and the new content one of three ways:
  --from-file <path>   Rule-shaped JSON
  --stdin              Rule-shaped JSON on stdin
  individual flags     --summary/--instruction/--glob/--tag/...

A --changelog message is required to describe what changed and why.
The rule is validated, then the existing rule is replaced.`,

		Example: `  # Update a rule's summary
  gsc rules update --id <id> --summary "New summary" --changelog "Updated summary for clarity"

  # Update a rule from a JSON file
  gsc rules update --id <id> --from-file /tmp/rule.json --changelog "Added new instruction"

  # Update a rule's instructions
  gsc rules update --id <id> \
    --instruction "New instruction 1" \
    --instruction "New instruction 2" \
    --action edit --action write \
    --changelog "Updated formatting instructions"`,

		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			if changelog == "" {
				return fmt.Errorf("--changelog is required")
			}
			if err := validateCreatorFlag(creator); err != nil {
				return err
			}
			if isAgentCreator(creator) && fromFile == "" && !useStdin {
				return fmt.Errorf("--creator agent requires --from-file or --stdin with structured rule JSON")
			}

			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			// Load and resolve within the target store only.
			records, err := rulespkg.LoadRecordsFromTarget(target)
			if err != nil {
				return fmt.Errorf("failed to load rules: %w", err)
			}
			existing, err := rulespkg.ResolveRecordFromRecords(id, records)
			if err != nil {
				return err
			}
			if existing == nil {
				return fmt.Errorf("rule not found in %s store: %s", target, id)
			}

			// Parse instructions from flags (now just strings)
			instructions, _ := cmd.Flags().GetStringArray("instruction")

			var rule rulespkg.Rule
			if fromFile != "" || useStdin {
				// JSON mode
				var data []byte
				var err error
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
				// Flag mode - merge with existing
				rule = *existing
				if cmd.Flags().Changed("summary") {
					rule.Summary = summary
				}
				if cmd.Flags().Changed("details") {
					rule.Details = details
				}
				if cmd.Flags().Changed("event") {
					rule.Event = rulespkg.LifecycleEvent(event)
				}
				if cmd.Flags().Changed("importance") {
					rule.Importance = importance
				}
				if cmd.Flags().Changed("owner") {
					rule.Owner = owner
				}
				if cmd.Flags().Changed("contact") {
					rule.Contact = contact
				}
				if cmd.Flags().Changed("glob") {
					rule.GlobPatterns = globs
				}
				if cmd.Flags().Changed("exclude-glob") {
					rule.ExcludeGlobs = excludeGlobs
				}
				if cmd.Flags().Changed("file") {
					rule.AppliesTo.Files = files
				}
				if cmd.Flags().Changed("linked-file") {
					rule.AppliesTo.LinkedFiles = linkedFiles
				}
				if cmd.Flags().Changed("command") {
					rule.AppliesTo.Commands = commands
				}
				if cmd.Flags().Changed("topic") {
					rule.AppliesTo.Topics = topics
				}
				if cmd.Flags().Changed("tag") {
					rule.Tags = tags
				}
				if len(instructions) > 0 {
					rule.Instructions = instructions
				}
				if cmd.Flags().Changed("action") {
					rule.Actions = actions
				}
			}

			// Preserve identity
			rule.ID = existing.ID
			rule.SchemaVersion = existing.SchemaVersion
			rule.CreatedAt = existing.CreatedAt
			rule.ConfirmedBy = existing.ConfirmedBy
			rule.ConfirmedAt = existing.ConfirmedAt

			// Update timestamp
			rule.UpdatedAt = time.Now().UTC()

			// Validate and normalize
			result := rulespkg.ValidateAndNormalize(rule)
			if !result.Valid() {
				fmt.Println("Rule content is invalid; nothing updated:")
				for _, err := range result.Errors {
					fmt.Printf("  ERROR %s\n", err)
				}
				return fmt.Errorf("rule content is invalid")
			}
			if isAgentCreator(creator) {
				if err := validateAndStripAgentChecklist(&result.Rule, target); err != nil {
					fmt.Println("Agent creator checklist is invalid; nothing updated:")
					printAgentChecklistErrors(err)
					return err
				}
			}

			rule = result.Rule
			rule.Keywords = rulespkg.KeywordsFor(rule)
			rule.ParentKeywords = rulespkg.ParentKeywordsFor(rule)

			// Add changelog entry
			changelogEntry := rulespkg.ChangelogEntry{
				Timestamp: rule.UpdatedAt,
				Message:   changelog,
			}
			rule.Changelog = append(existing.Changelog, changelogEntry)

			found := false
			for i, r := range records {
				if r.ID == rule.ID {
					records[i] = rule
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("rule not found in %s store: %s", target, rule.ID)
			}

			// Write records
			if err := rulespkg.WriteRecordsToTarget(records, target); err != nil {
				return fmt.Errorf("failed to write rules: %w", err)
			}

			// Rebuild the Brain
			if err := rulespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}

			fmt.Printf("Rule updated in %s scope: %s\n", target, rule.ID)
			fmt.Printf("Summary: %s\n", rule.Summary)
			fmt.Printf("Instructions: %d\n", len(rule.Instructions))
			fmt.Printf("Actions: %s\n", rule.Actions)
			if len(rule.Tags) > 0 {
				fmt.Printf("Tags: %s\n", rule.Tags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Rule ID to update (required)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read rule-shaped JSON content from a file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read rule-shaped JSON content from stdin")
	cmd.Flags().StringVar(&summary, "summary", "", "Rule summary")
	cmd.Flags().StringVar(&details, "details", "", "Rule details")
	cmd.Flags().StringVar(&event, "event", "", "Lifecycle event (session_start, user_prompt_submit, pre_tool_use, post_tool_use, post_tool_batch, agent_end, session_end)")
	cmd.Flags().StringVar(&importance, "importance", "", "Importance: low, medium, or high")
	cmd.Flags().StringVar(&owner, "owner", "", "Rule owner")
	cmd.Flags().StringArrayVar(&contact, "contact", nil, "Contact (repeatable)")
	cmd.Flags().StringArrayVar(&globs, "glob", nil, "Glob pattern (repeatable)")
	cmd.Flags().StringArrayVar(&excludeGlobs, "exclude-glob", nil, "Exclude glob pattern (repeatable)")
	cmd.Flags().StringArrayVar(&files, "file", nil, "Repo-relative file this rule applies to (repeatable)")
	cmd.Flags().StringArrayVar(&linkedFiles, "linked-file", nil, "Repo-relative related file (repeatable)")
	cmd.Flags().StringArrayVar(&commands, "command", nil, "Command this rule applies to (repeatable)")
	cmd.Flags().StringArrayVar(&topics, "topic", nil, "Topic slug (repeatable)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag slug (repeatable)")
	cmd.Flags().StringArray("instruction", nil, "Instruction text (repeatable)")
	cmd.Flags().StringArrayVar(&actions, "action", nil, "Action this rule applies to (repeatable: read, write, edit, bash, tool, mcp_tool, prompt, agent_end)")
	cmd.Flags().StringVar(&changelog, "changelog", "", "Changelog message describing what changed (required)")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	cmd.Flags().StringVar(&creator, "creator", "", "Creator type: agent or human. Agent-created updates require structured JSON with creatorChecklist.")
	return cmd
}
