/**
 * Component: Rules Get Command
 * Block-UUID: e1f2a3b4-c5d6-7890-ef01-901234567890
 * Parent-UUID: N/A
 * Version: 3.1.0
 * Description: Implements gsc rules get, the agent-facing command for querying rules by file, glob, tag, or event. Returns git_root in JSON output. Supports action filtering, absolute paths, and lifecycle event filtering.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Updated-at: 2026-06-24T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), terrchen (v3.1.0)
 * Changelog:
 *   v3.1.0 - Add --event flag for lifecycle event filtering
 */

package rules

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	gitpkg "github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func getCmd() *cobra.Command {
	var (
		file       string
		glob       string
		tags       []string // Repeatable --tag for OR semantics
		action     string
		event      string
		format     string
		scopeValue string
		toolName   string // Actual tool name to match against rule's tool_filter
		command    string // Actual command to match against rule's command_filter
		prompt     string // Actual prompt text to match against rule's prompt_filter
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Query rules for a file, glob pattern, tag, or event",
		Long: `Query rules that match a specific file, glob pattern, tag, or lifecycle event.

This is the primary command for coding agents to check for rules before modifying files.

Supports absolute paths: when --file is an absolute path, the command
discovers the owning repository and normalizes the path to repo-relative.

Exit codes:
  0 - Lookup succeeded (including "no rules found")
  1 - Lookup failed (bad args, etc.)`,
		Example: `  # Check rules for a file
  gsc rules get --file internal/cli/root.go

  # Check rules for a file when editing
  gsc rules get --file internal/cli/root.go --action edit

  # Check rules for a lifecycle event
  gsc rules get --event pre_tool_use --file src/foo.ts --action edit

  # Check rules for agent_end event
  gsc rules get --event agent_end

  # Check rules for an absolute path
  gsc rules get --file /abs/path/to/repo/foo.bar --action edit

  # Check rules for a directory
  gsc rules get --glob "internal/cli/**"

  # Check rules by tag
  gsc rules get --tag formatting

  # Check rules matching a specific tool name
  gsc rules get --event pre_tool_use --action mcp_tool --tool github.create_issue

  # Check rules matching a bash command
  gsc rules get --event pre_tool_use --action bash --command "rm -rf /tmp/foo"

  # JSON output for agents
  gsc rules get --file internal/cli/root.go --format json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate: at least one query parameter
			if file == "" && glob == "" && len(tags) == 0 && action == "" && event == "" && toolName == "" && command == "" && prompt == "" {
				return fmt.Errorf("at least one of --file, --glob, --tag, --action, --event, --tool, --command, or --prompt is required")
			}

			// Parse and validate lifecycle event
			var lifecycleEvent rulespkg.LifecycleEvent
			if event != "" {
				lifecycleEvent = rulespkg.LifecycleEvent(event)
				if !rulespkg.IsValidLifecycleEvent(lifecycleEvent) {
					return fmt.Errorf("invalid event %q; must be one of: session_start, user_prompt_submit, pre_tool_use, post_tool_use, post_tool_batch, agent_end, session_end", event)
				}
			}

			// Validate mutually exclusive flags
			if toolName != "" && command != "" {
				return fmt.Errorf("--tool and --command are mutually exclusive; use one or the other")
			}
			if prompt != "" && (toolName != "" || command != "") {
				return fmt.Errorf("--prompt is mutually exclusive with --tool and --command")
			}

			// Handle absolute paths
			normalizedFile := file
			var gitRoot string
			if file != "" && filepath.IsAbs(file) {
				// Discover owning repo from the absolute path
				discoveredRoot, err := discoverRepoFromPath(file)
				if err != nil {
					return fmt.Errorf("could not discover repository for path %s: %w", file, err)
				}
				gitRoot = discoveredRoot

				// Convert to repo-relative path
				relPath, err := filepath.Rel(gitRoot, file)
				if err != nil {
					return fmt.Errorf("failed to compute relative path: %w", err)
				}
				normalizedFile = filepath.ToSlash(relPath)
			}

			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			// Load records with scope
			var sourcedRecords []rulespkg.SourcedRule
			var loadErr error
			if gitRoot != "" {
				sourcedRecords, loadErr = rulespkg.LoadRecordsFromScopeForRepo(scope, gitRoot)
			} else {
				sourcedRecords, loadErr = rulespkg.LoadRecordsFromScope(scope)
			}
			if loadErr != nil {
				return fmt.Errorf("failed to load rules: %w", loadErr)
			}

			var sourcedMatched []rulespkg.SourcedMatchedRule
			queryType := ""
			queryValue := ""
			normalizedValue := ""

			if file != "" {
				queryType = "file"
				queryValue = file
				normalizedValue = normalizedFile
				if action != "" {
					sourcedMatched = rulespkg.GetSourcedRulesForFile(sourcedRecords, normalizedFile, action, lifecycleEvent)
				} else {
					sourcedMatched = rulespkg.GetSourcedRulesForFileAllActions(sourcedRecords, normalizedFile, lifecycleEvent)
				}
			} else if glob != "" {
				queryType = "glob"
				queryValue = glob
				sourcedMatched = rulespkg.GetSourcedRulesForGlob(sourcedRecords, glob, lifecycleEvent)
			} else if len(tags) > 0 {
				queryType = "tag"
				queryValue = strings.Join(tags, ",")
				// OR semantics: match rules that have ANY of the provided tags
				seen := make(map[string]bool)
				for _, tag := range tags {
					for _, smr := range rulespkg.GetSourcedRulesForTag(sourcedRecords, tag, lifecycleEvent) {
						seenKey := string(smr.Source) + ":" + smr.MatchedRule.Rule.ID
						if !seen[seenKey] {
							seen[seenKey] = true
							sourcedMatched = append(sourcedMatched, smr)
						}
					}
				}
			} else if action != "" {
				queryType = "action"
				queryValue = action
				sourcedMatched = rulespkg.GetSourcedRulesForAction(sourcedRecords, action, lifecycleEvent)
			} else if event != "" {
				queryType = "event"
				queryValue = event
				// For event-only queries, return all rules matching that event
				filtered := rulespkg.FilterSourcedRecords(sourcedRecords, rulespkg.ListFilter{Event: lifecycleEvent})
				for _, sr := range filtered {
					sourcedMatched = append(sourcedMatched, rulespkg.SourcedMatchedRule{
						Source: sr.Source,
						MatchedRule: rulespkg.MatchedRule{
							Rule:        sr.Rule,
							MatchReason: fmt.Sprintf("event: %s", event),
							RuleHash:    sr.Rule.ComputeRuleHash(),
						},
					})
				}
			} else if toolName != "" || command != "" || prompt != "" {
				queryType = "filter"
				queryValue = "tool=" + toolName + ",command=" + command + ",prompt=" + prompt
				// For filter-only queries, return all rules and let the filter logic handle it
				for _, sr := range sourcedRecords {
					sourcedMatched = append(sourcedMatched, rulespkg.SourcedMatchedRule{
						Source: sr.Source,
						MatchedRule: rulespkg.MatchedRule{
							Rule:        sr.Rule,
							MatchReason: "filter query",
							RuleHash:    sr.Rule.ComputeRuleHash(),
						},
					})
				}
			}

			// Apply tool and command filters (evaluate mode)
			var filtered []rulespkg.SourcedMatchedRule
			for _, smr := range sourcedMatched {
				mr := smr.MatchedRule
				// Filter by tool name
				if toolName != "" {
					// With --tool: only include rules where tool_filter matches
					if mr.Rule.ToolFilter == "" {
						// Skip rules with tool_filter: null (generic match-any)
						continue
					}
					matched, err := path.Match(mr.Rule.ToolFilter, toolName)
					if err != nil || !matched {
						continue
					}
				} else if toolName == "" && command == "" {
					// Without --tool and --command: only include rules with tool_filter: null (match any)
					if mr.Rule.ToolFilter != "" {
						continue
					}
				}
				// Filter by command
				if command != "" {
					// With --command: only include rules where command_filter matches
					if mr.Rule.CommandFilter == "" {
						// Skip rules with command_filter: null (generic match-any)
						continue
					}
					re, err := regexp.Compile(mr.Rule.CommandFilter)
					if err != nil {
						continue
					}
					if !re.MatchString(command) {
						continue
					}
				} else if toolName == "" && command == "" {
					// Without --tool and --command: only include rules with command_filter: null (match any)
					if mr.Rule.CommandFilter != "" {
						continue
					}
				}
				// Filter by prompt
				if prompt != "" {
					// With --prompt: only include rules where prompt_filter matches
					if mr.Rule.PromptFilter == "" {
						// Skip rules with prompt_filter: null (generic match-any)
						continue
					}
					re, err := regexp.Compile(mr.Rule.PromptFilter)
					if err != nil {
						continue
					}
					if !re.MatchString(prompt) {
						continue
					}
				} else if toolName == "" && command == "" && prompt == "" {
					// Without --tool, --command, and --prompt: only include rules with prompt_filter: null (match any)
					if mr.Rule.PromptFilter != "" {
						continue
					}
				}
				filtered = append(filtered, smr)
			}
			sourcedMatched = filtered

			switch format {
			case "json":
				return renderGetJSON(queryType, queryValue, normalizedValue, action, event, gitRoot, scope, sourcedMatched)
			case "rules-json":
				return renderGetRulesJSON(queryType, queryValue, normalizedValue, action, event, gitRoot, scope, sourcedMatched, command, prompt)
			default:
				return renderGetHuman(queryType, queryValue, scope, sourcedMatched)
			}
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "File path to query (supports absolute paths)")
	cmd.Flags().StringVar(&glob, "glob", "", "Glob pattern to query")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag to query (repeatable for OR semantics: --tag foo --tag bar)")
	cmd.Flags().StringVar(&action, "action", "", "Filter by action (read, write, edit, bash, tool, mcp_tool, prompt, agent_end)")
	cmd.Flags().StringVar(&event, "event", "", "Filter by lifecycle event (session_start, before_agent_start, user_prompt_submit, agent_start, pre_tool_use, post_tool_use, post_tool_batch, context, session_before_compact, session_compact, agent_end, session_end)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVar(&toolName, "tool", "", "Match actual tool name against rule's tool_filter pattern")
	cmd.Flags().StringVar(&command, "command", "", "Match actual command against rule's command_filter pattern")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Match actual prompt text against rule's prompt_filter pattern")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json, rules-json)")
	return cmd
}

// discoverRepoFromPath discovers the owning repository from an absolute file path.
func discoverRepoFromPath(absPath string) (string, error) {
	// Get the starting directory (parent if file, itself if directory)
	startPath := absPath
	info, err := os.Stat(absPath)
	if err != nil {
		// Path doesn't exist, try parent directory
		startPath = filepath.Dir(absPath)
	} else if !info.IsDir() {
		startPath = filepath.Dir(absPath)
	}

	// Walk upward to find the git root
	root, err := gitpkg.FindGitRootFrom(startPath)
	if err != nil {
		return "", fmt.Errorf("no git repository found for path %s", absPath)
	}

	return root, nil
}

// loadRecordsFromRepo loads rule records from a specific repository.
func loadRecordsFromRepo(repoRoot string) ([]rulespkg.Rule, error) {
	recordsPath := filepath.Join(repoRoot, ".gitsense", "rules", "records.jsonl")
	return rulespkg.LoadRecordsFromPath(recordsPath, true)
}

func renderGetHuman(queryType, queryValue string, scope gitsensescope.Scope, matched []rulespkg.SourcedMatchedRule) error {
	fmt.Printf("Query: %s=%s\n", queryType, queryValue)
	fmt.Printf("Scope: %s", scope)
	if scope == gitsensescope.ScopeAll {
		fmt.Print(" (repo + personal)")
	}
	fmt.Println()
	fmt.Printf("Rules matched: %d\n\n", len(matched))

	if len(matched) == 0 {
		emptyMessage := fmt.Sprintf("No rules found in %s scope.", scope)
		if scope == gitsensescope.ScopeAll {
			emptyMessage = "No rules found in repo or personal scope."
		}
		fmt.Println(emptyMessage)
		return nil
	}

	// Group by source for display
	repoMatched, personalMatched := splitBySource(matched)

	if len(repoMatched) > 0 {
		fmt.Println("Repo rules:")
		fmt.Print(rulespkg.RenderMatchedRulesTable(unwrapSourcedMatched(repoMatched)))
	}
	if len(personalMatched) > 0 {
		if len(repoMatched) > 0 {
			fmt.Println()
		}
		fmt.Println("Personal rules:")
		fmt.Print(rulespkg.RenderMatchedRulesTable(unwrapSourcedMatched(personalMatched)))
	}
	return nil
}

func splitBySource(matched []rulespkg.SourcedMatchedRule) (repo, personal []rulespkg.SourcedMatchedRule) {
	for _, smr := range matched {
		if smr.Source == gitsensescope.SourceRepo {
			repo = append(repo, smr)
		} else {
			personal = append(personal, smr)
		}
	}
	return
}

func unwrapSourcedMatched(matched []rulespkg.SourcedMatchedRule) []rulespkg.MatchedRule {
	result := make([]rulespkg.MatchedRule, len(matched))
	for i, smr := range matched {
		result[i] = smr.MatchedRule
	}
	return result
}

// sourcedJSONRule wraps MatchedRule with source for JSON output.
type sourcedJSONRule struct {
	Source      gitsensescope.Source      `json:"source"`
	Rule        rulespkg.Rule             `json:"rule"`
	MatchReason string                    `json:"match_reason"`
	Match       *rulespkg.MatchProvenance `json:"match,omitempty"`
	RuleHash    string                    `json:"ruleHash"`
	TriggerHash string                    `json:"triggerHash,omitempty"`
}

func renderGetJSON(queryType, queryValue, normalizedValue, action, event, gitRoot string, scope gitsensescope.Scope, matched []rulespkg.SourcedMatchedRule) error {
	high, medium, low := 0, 0, 0
	for _, smr := range matched {
		switch smr.MatchedRule.Rule.Importance {
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}

	// If gitRoot not provided, try to discover from current directory
	if gitRoot == "" {
		var err error
		gitRoot, err = gitpkg.FindGitRoot()
		if err != nil {
			gitRoot = ""
		}
	}

	// Determine active sources
	sourceSet := make(map[gitsensescope.Source]bool)
	for _, smr := range matched {
		sourceSet[smr.Source] = true
	}
	var sources []gitsensescope.Source
	if sourceSet[gitsensescope.SourceRepo] {
		sources = append(sources, gitsensescope.SourceRepo)
	}
	if sourceSet[gitsensescope.SourcePersonal] {
		sources = append(sources, gitsensescope.SourcePersonal)
	}

	// Build rules with source
	rules := make([]sourcedJSONRule, len(matched))
	for i, smr := range matched {
		rules[i] = sourcedJSONRule{
			Source:      smr.Source,
			Rule:        smr.MatchedRule.Rule,
			MatchReason: smr.MatchedRule.MatchReason,
			Match:       smr.MatchedRule.Match,
			RuleHash:    smr.MatchedRule.RuleHash,
			TriggerHash: smr.MatchedRule.TriggerHash,
		}
	}

	output := struct {
		Query struct {
			File           string `json:"file,omitempty"`
			NormalizedFile string `json:"normalized_file,omitempty"`
			Glob           string `json:"glob,omitempty"`
			Tag            string `json:"tag,omitempty"`
			Action         string `json:"action,omitempty"`
			Event          string `json:"event,omitempty"`
		} `json:"query"`
		Scope   string                 `json:"scope"`
		Sources []gitsensescope.Source `json:"sources"`
		GitRoot string                 `json:"git_root"`
		Rules   []sourcedJSONRule      `json:"rules"`
		Summary struct {
			RulesMatched int `json:"rules_matched"`
			High         int `json:"high"`
			Medium       int `json:"medium"`
			Low          int `json:"low"`
		} `json:"summary"`
	}{}

	switch queryType {
	case "file":
		output.Query.File = queryValue
		if normalizedValue != "" && normalizedValue != queryValue {
			output.Query.NormalizedFile = normalizedValue
		}
	case "glob":
		output.Query.Glob = queryValue
	case "tag":
		output.Query.Tag = queryValue
	}

	if action != "" {
		output.Query.Action = action
	}

	if event != "" {
		output.Query.Event = event
	}

	output.Scope = string(scope)
	output.Sources = sources
	output.GitRoot = gitRoot
	output.Rules = rules
	output.Summary.RulesMatched = len(matched)
	output.Summary.High = high
	output.Summary.Medium = medium
	output.Summary.Low = low

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// RulesJSONRule is the rule representation in rules-json output format.
type RulesJSONRule struct {
	Source       gitsensescope.Source      `json:"source"`
	ID           string                    `json:"id"`
	Type         string                    `json:"type"`
	Event        string                    `json:"event,omitempty"`
	Summary      string                    `json:"summary"`
	Instructions []string                  `json:"instructions,omitempty"`
	Trigger      *rulespkg.TriggerConfig   `json:"trigger,omitempty"`
	Frequency    *rulespkg.FrequencyConfig `json:"frequency,omitempty"`
	Match        *rulespkg.MatchProvenance `json:"match,omitempty"`
	RuleHash     string                    `json:"ruleHash"`
	TriggerHash  string                    `json:"triggerHash,omitempty"`
	Priority     int                       `json:"priority"`
	Importance   string                    `json:"importance"`
}

// RulesJSONOutput is the top-level output structure for rules-json format.
type RulesJSONOutput struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Scope         string                 `json:"scope"`
	Sources       []gitsensescope.Source `json:"sources"`
	Query         RulesJSONQuery         `json:"query"`
	GitRoot       string                 `json:"gitRoot"`
	Rules         []RulesJSONRule        `json:"rules"`
	Summary       RulesJSONSummary       `json:"summary"`
}

// RulesJSONQuery captures the query parameters used.
type RulesJSONQuery struct {
	File    string `json:"file,omitempty"`
	Glob    string `json:"glob,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Action  string `json:"action,omitempty"`
	Event   string `json:"event,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Command string `json:"command,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
}

// RulesJSONSummary captures summary statistics.
type RulesJSONSummary struct {
	Total       int `json:"total"`
	Declarative int `json:"declarative"`
	Executable  int `json:"executable"`
}

// renderGetRulesJSON renders matched rules in rules-json format for gsc rules execute.
func renderGetRulesJSON(queryType, queryValue, normalizedValue, action, event, gitRoot string, scope gitsensescope.Scope, matched []rulespkg.SourcedMatchedRule, command, prompt string) error {
	// If gitRoot not provided, try to discover from current directory
	if gitRoot == "" {
		var err error
		gitRoot, err = gitpkg.FindGitRoot()
		if err != nil {
			gitRoot = ""
		}
	}

	// Build query
	query := RulesJSONQuery{}
	switch queryType {
	case "file":
		query.File = queryValue
	case "glob":
		query.Glob = queryValue
	case "tag":
		query.Tag = queryValue
	case "action":
		query.Action = queryValue
	case "event":
		query.Event = queryValue
	}
	if action != "" {
		query.Action = action
	}
	if event != "" {
		query.Event = event
	}
	if command != "" {
		query.Command = command
	}
	if prompt != "" {
		query.Prompt = prompt
	}

	// Determine active sources
	sourceSet := make(map[gitsensescope.Source]bool)
	for _, smr := range matched {
		sourceSet[smr.Source] = true
	}
	var sources []gitsensescope.Source
	if sourceSet[gitsensescope.SourceRepo] {
		sources = append(sources, gitsensescope.SourceRepo)
	}
	if sourceSet[gitsensescope.SourcePersonal] {
		sources = append(sources, gitsensescope.SourcePersonal)
	}

	// Build rules array
	rules := make([]RulesJSONRule, 0, len(matched))
	declarativeCount := 0
	executableCount := 0

	for _, smr := range matched {
		mr := smr.MatchedRule
		ruleType := string(mr.Rule.Type)
		// Normalize tool-trigger to executable for rules-json output
		if ruleType == "tool-trigger" {
			ruleType = "executable"
		} else if ruleType == "" || ruleType == "instruction" {
			ruleType = "declarative"
		}

		if ruleType == "executable" {
			executableCount++
		} else {
			declarativeCount++
		}

		eventStr := string(mr.Rule.Event)
		if eventStr == "" {
			eventStr = "pre_tool_use"
		}

		rulesJSONRule := RulesJSONRule{
			Source:       smr.Source,
			ID:           mr.Rule.ID,
			Type:         ruleType,
			Event:        eventStr,
			Summary:      mr.Rule.Summary,
			Instructions: mr.Rule.Instructions,
			Trigger:      mr.Rule.Trigger,
			Frequency:    mr.Rule.Frequency,
			Match:        mr.Match,
			RuleHash:     mr.RuleHash,
			TriggerHash:  mr.TriggerHash,
			Priority:     mr.Rule.Priority,
			Importance:   mr.Rule.Importance,
		}

		rules = append(rules, rulesJSONRule)
	}

	// Build output
	output := RulesJSONOutput{
		SchemaVersion: 1,
		Scope:         string(scope),
		Sources:       sources,
		Query:         query,
		GitRoot:       gitRoot,
		Rules:         rules,
		Summary: RulesJSONSummary{
			Total:       len(matched),
			Declarative: declarativeCount,
			Executable:  executableCount,
		},
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
