/**
 * Component: Rules Tree Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-ef1234567891
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules tree for visualizing rule scoping across the repository tree.
 * Language: Go
 * Created-at: 2026-06-23T20:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	gitpkg "github.com/gitsense/gsc-cli/internal/git"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func treeCmd() *cobra.Command {
	var (
		ruleIDs   []string
		topics    []string
		ruleType  string
		action    string
		maxDepth  int
		format    string
		scopeValue string
	)

	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Visualize rule scoping across the repository tree",
		Long: `Display a tree view showing where rules apply in the repository.

This is a validation tool for confirming:
- Rules are scoped correctly
- Broad ** rules are not too broad
- Group ownership boundaries
- What will run where for pi-brains

Filters are composable (AND-combined):
  --topic go-conventions --type tool-trigger --action edit

At least one rule must exist (or match filter criteria).

Broad ** rules are shown in a separate "repo-wide" section, not expanded
to every file.`,
		Example: `  # Show all rules in tree
  gsc rules tree

  # Show specific rule
  gsc rules tree --rule-id 019ef5f6

  # Filter by topic and type
  gsc rules tree --topic go-conventions --type tool-trigger

  # Filter by action
  gsc rules tree --action edit

  # Limit depth
  gsc rules tree --max-depth 2

  # JSON output
  gsc rules tree --format json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load records from scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}
			sourcedRecords, err := rulespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return fmt.Errorf("failed to load rules: %w", err)
			}

			// Convert to plain rules
			var records []rulespkg.Rule
			for _, sr := range sourcedRecords {
				records = append(records, sr.Rule)
			}

			// 2. Apply filters (composable)
			var filtered []rulespkg.Rule

			if len(ruleIDs) > 0 {
				// --rule-id: select exact rules
				for _, id := range ruleIDs {
					sourced, err := rulespkg.ResolveSourcedRecordFromRecords(id, sourcedRecords)
					if err != nil || sourced == nil {
						return fmt.Errorf("rule not found: %s", id)
					}
					filtered = append(filtered, sourced.Rule)
				}
			} else {
				// Start with all rules
				filtered = records
			}

			// Filter by topic
			if len(topics) > 0 {
				filtered = filterByTopic(filtered, topics)
			}

			// Filter by type
			if ruleType != "" {
				filtered = filterByType(filtered, ruleType)
			}

			// Filter by action
			if action != "" {
				filtered = filterByAction(filtered, action)
			}

			// 3. Require at least one rule
			if len(filtered) == 0 {
				return fmt.Errorf("no rules match the specified criteria")
			}

			// 4. Get repo context
			repoRoot, cwdOffset, err := gitpkg.GetRepoContext()
			if err != nil {
				return fmt.Errorf("failed to get repository context: %w", err)
			}

			// 5. Get tracked files
			files, err := gitpkg.GetTrackedFiles(context.Background(), repoRoot)
			if err != nil {
				return fmt.Errorf("failed to get tracked files: %w", err)
			}

			// 6. Build rules tree
			result := buildRulesTree(filtered, files, cwdOffset, action, maxDepth)

			// 7. Render output
			switch format {
			case "json":
				return renderRulesTreeJSON(result, repoRoot)
			default:
				return renderRulesTreeHuman(result, maxDepth)
			}
		},
	}

	cmd.Flags().StringSliceVar(&ruleIDs, "rule-id", nil, "Specific rule ID(s) to show")
	cmd.Flags().StringSliceVar(&topics, "topic", nil, "Filter by topic slug(s)")
	cmd.Flags().StringVar(&ruleType, "type", "", "Filter by rule type (instruction, tool-trigger)")
	cmd.Flags().StringVar(&action, "action", "", "Filter by action (read, edit, write)")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 0, "Maximum tree depth (0 = unlimited)")
	cmd.Flags().StringVar(&format, "format", "human", "Output format (human, json)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")

	return cmd
}

// RulesTreeResult represents the result of building a rules tree.
type RulesTreeResult struct {
	Nodes        []RulesTreeNode        `json:"nodes"`
	RepoWide     []RulesTreeRule        `json:"repo_wide"`
	UnmatchedRules []UnmatchedRule      `json:"unmatched_rules"`
	Stats        RulesTreeStats         `json:"stats"`
}

// RulesTreeNode represents a directory or file in the tree.
type RulesTreeNode struct {
	Path     string          `json:"path"`
	Kind     string          `json:"kind"` // "file" or "directory"
	Rules    []RulesTreeRule `json:"rules,omitempty"`
	Children []RulesTreeNode `json:"children,omitempty"`
}

// RulesTreeRule represents a rule matched at a node.
type RulesTreeRule struct {
	ID          string                  `json:"id"`
	Type        string                  `json:"type"`
	Summary     string                  `json:"summary"`
	Actions     []string                `json:"actions"`
	Match       *rulespkg.MatchProvenance `json:"match"`
	Trigger     *TriggerInfo            `json:"trigger,omitempty"`
	RuleHash    string                  `json:"ruleHash"`
	TriggerHash string                  `json:"triggerHash,omitempty"`
	Error       string                  `json:"error,omitempty"`
}

// TriggerInfo contains trigger metadata for tool-trigger rules.
type TriggerInfo struct {
	Runtime string `json:"runtime"`
	Entry   string `json:"entry"`
}

// UnmatchedRule represents a rule that didn't match any files.
type UnmatchedRule struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Reason  string `json:"reason"`
}

// RulesTreeStats contains statistics about the rules tree.
type RulesTreeStats struct {
	TotalFiles      int `json:"total_files"`
	SelectedRules   int `json:"selected_rules"`
	MatchedRules    int `json:"matched_rules"`
	UnmatchedRules  int `json:"unmatched_rules"`
	Nodes           int `json:"nodes"`
	RepoWideRules   int `json:"repo_wide_rules"`
}

// buildRulesTree builds the rules tree structure.
func buildRulesTree(rules []rulespkg.Rule, files []string, cwdOffset string, filterAction string, maxDepth int) RulesTreeResult {
	result := RulesTreeResult{
		Stats: RulesTreeStats{
			TotalFiles:    len(files),
			SelectedRules: len(rules),
		},
	}

	// Track which rules matched at least one file
	matchedRuleIDs := make(map[string]bool)

	// Build a map of directory -> rules
	dirRules := make(map[string][]RulesTreeRule)

	// Track repo-wide rules (broad ** patterns)
	var repoWideRules []RulesTreeRule

	// Process each rule
	for _, rule := range rules {
		ruleHash := rule.ComputeRuleHash()
		triggerHash := ""
		var triggerErr string

		if rule.IsExecutable() {
			th, err := rule.ComputeTriggerHash()
			if err != nil {
				triggerErr = err.Error()
				triggerHash = ""
			} else {
				triggerHash = th
			}
		}

		// Determine actions to display
		actions := rule.Actions
		if filterAction != "" {
			actions = []string{filterAction}
		}

		// Check if this is a repo-wide rule
		isRepoWide := false
		for _, glob := range rule.GlobPatterns {
			if glob == "**" || glob == ".*" {
				isRepoWide = true
				break
			}
		}

		if isRepoWide {
			ruleType := string(rule.Type)
			if ruleType == "" {
				ruleType = "instruction"
			}

			treeRule := RulesTreeRule{
				ID:          rule.ID,
				Type:        ruleType,
				Summary:     rule.Summary,
				Actions:     actions,
				RuleHash:    ruleHash,
				TriggerHash: triggerHash,
				Error:       triggerErr,
				Match: &rulespkg.MatchProvenance{
					Kind:  "glob",
					Value: "**",
				},
			}
			if rule.IsExecutable() && rule.Trigger != nil {
				treeRule.Trigger = &TriggerInfo{
					Runtime: rule.Trigger.Runtime,
					Entry:   rule.Trigger.Entry,
				}
			}
			repoWideRules = append(repoWideRules, treeRule)
			matchedRuleIDs[rule.ID] = true
			continue
		}

		// For each file, check if the rule matches
		ruleMatched := false
		for _, file := range files {
			provenance := rulespkg.GetFileMatchProvenance(rule, file)
			if provenance == nil {
				continue
			}

			// Filter by action if specified
			if filterAction != "" && !containsString(rule.Actions, filterAction) {
				continue
			}

			ruleMatched = true
			matchedRuleIDs[rule.ID] = true

			// Determine the display path
			displayPath := determineDisplayPath(file, rule)

			ruleType := string(rule.Type)
			if ruleType == "" {
				ruleType = "instruction"
			}

			treeRule := RulesTreeRule{
				ID:          rule.ID,
				Type:        ruleType,
				Summary:     rule.Summary,
				Actions:     actions,
				Match:       provenance,
				RuleHash:    ruleHash,
				TriggerHash: triggerHash,
				Error:       triggerErr,
			}
			if rule.IsExecutable() && rule.Trigger != nil {
				treeRule.Trigger = &TriggerInfo{
					Runtime: rule.Trigger.Runtime,
					Entry:   rule.Trigger.Entry,
				}
			}

			dirRules[displayPath] = append(dirRules[displayPath], treeRule)
		}

		if !ruleMatched {
			// Rule didn't match any files
			reason := "no tracked files matched"
			if len(rule.GlobPatterns) > 0 {
				reason = fmt.Sprintf("no tracked files matched glob: %s", strings.Join(rule.GlobPatterns, ", "))
			}
			result.UnmatchedRules = append(result.UnmatchedRules, UnmatchedRule{
				ID:      rule.ID,
				Summary: rule.Summary,
				Reason:  reason,
			})
		}
	}

	// Build tree structure from dirRules
	result.Nodes = buildTreeNodes(dirRules, cwdOffset, maxDepth, 0)
	result.RepoWide = repoWideRules

	// Update stats
	result.Stats.MatchedRules = len(matchedRuleIDs)
	result.Stats.UnmatchedRules = len(result.UnmatchedRules)
	result.Stats.Nodes = countNodes(result.Nodes)
	result.Stats.RepoWideRules = len(repoWideRules)

	return result
}

// determineDisplayPath determines the path to display a rule at.
// For exact file matches (applies_to.files), returns the file path.
// For glob patterns, returns the narrowest stable directory.
func determineDisplayPath(file string, rule rulespkg.Rule) string {
	// Check applies_to.files for exact file matches
	for _, exactFile := range rule.AppliesTo.Files {
		if exactFile == file {
			return file
		}
	}

	// For glob patterns, use the narrowest stable directory
	for _, glob := range rule.GlobPatterns {
		if !strings.Contains(glob, "*") && !strings.Contains(glob, "?") {
			// Exact file match in glob
			if glob == file {
				return file
			}
		}
		dir := narrowestDir(glob)
		if dir != "" {
			// Find the actual directory that matches
			fileDir := filepath.Dir(file)
			if strings.HasPrefix(fileDir, dir) {
				return dir
			}
		}
	}

	// Default to file's directory
	return filepath.Dir(file)
}

// narrowestDir extracts the narrowest stable directory from a glob pattern.
func narrowestDir(glob string) string {
	// Remove ** patterns to find static prefix
	parts := strings.Split(glob, "/")
	var staticParts []string
	for _, part := range parts {
		if strings.Contains(part, "*") || strings.Contains(part, "?") {
			break
		}
		staticParts = append(staticParts, part)
	}
	if len(staticParts) > 0 {
		return strings.Join(staticParts, "/")
	}
	return ""
}

// buildTreeNodes builds the tree node structure from a path map.
func buildTreeNodes(pathRules map[string][]RulesTreeRule, cwdOffset string, maxDepth, currentDepth int) []RulesTreeNode {
	if maxDepth > 0 && currentDepth >= maxDepth {
		return nil
	}

	// Get sorted paths
	paths := make([]string, 0, len(pathRules))
	for path := range pathRules {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var nodes []RulesTreeNode
	seen := make(map[string]bool)

	for _, path := range paths {
		if seen[path] {
			continue
		}

		// Check if this path is a child of another path
		isChild := false
		for otherPath := range pathRules {
			if otherPath != path && strings.HasPrefix(path, otherPath+"/") {
				isChild = true
				break
			}
		}

		if isChild {
			continue
		}

		// Determine if this is a file or directory
		// Check if path looks like a file (has extension)
		isFile := strings.Contains(filepath.Base(path), ".")

		kind := "directory"
		if isFile {
			kind = "file"
		}

		node := RulesTreeNode{
			Path:  path,
			Kind:  kind,
			Rules: pathRules[path],
		}

		// Find child paths
		for otherPath, otherRules := range pathRules {
			if strings.HasPrefix(otherPath, path+"/") {
				childPath := strings.TrimPrefix(otherPath, path+"/")
				
				// Determine if child is file or directory
				childIsFile := false
				for _, rule := range otherRules {
					if rule.Match != nil && rule.Match.Kind == "file" {
						childIsFile = true
						break
					}
				}
				childKind := "directory"
				if childIsFile {
					childKind = "file"
				}

				childNode := RulesTreeNode{
					Path:  childPath,
					Kind:  childKind,
					Rules: otherRules,
				}
				node.Children = append(node.Children, childNode)
				seen[otherPath] = true
			}
		}

		// Sort children
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Path < node.Children[j].Path
		})

		nodes = append(nodes, node)
		seen[path] = true
	}

	// Sort nodes
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Path < nodes[j].Path
	})

	return nodes
}

// countNodes counts the total number of nodes in the tree.
func countNodes(nodes []RulesTreeNode) int {
	count := len(nodes)
	for _, node := range nodes {
		count += countNodes(node.Children)
	}
	return count
}

// filterByTopic filters rules by topic(s).
func filterByTopic(rules []rulespkg.Rule, topics []string) []rulespkg.Rule {
	var filtered []rulespkg.Rule
	for _, r := range rules {
		for _, topic := range topics {
			if r.Topic == topic {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}

// filterByType filters rules by type.
func filterByType(rules []rulespkg.Rule, ruleType string) []rulespkg.Rule {
	var filtered []rulespkg.Rule
	for _, r := range rules {
		if string(r.Type) == ruleType {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// filterByAction filters rules by action.
func filterByAction(rules []rulespkg.Rule, action string) []rulespkg.Rule {
	var filtered []rulespkg.Rule
	for _, r := range rules {
		if containsString(r.Actions, action) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// containsString checks if a string is in a slice.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// renderRulesTreeHuman renders the tree in human-readable format.
func renderRulesTreeHuman(result RulesTreeResult, maxDepth int) error {
	var sb strings.Builder

	// Render tree
	sb.WriteString(".\n")
	for i, node := range result.Nodes {
		isLast := i == len(result.Nodes)-1
		renderNodeHuman(&sb, node, "", isLast, maxDepth, 1)
	}

	// Render repo-wide rules
	if len(result.RepoWide) > 0 {
		sb.WriteString("└── ** repo-wide\n")
		for i, rule := range result.RepoWide {
			isLast := i == len(result.RepoWide)-1
			prefix := "    "
			if isLast {
				sb.WriteString(fmt.Sprintf("%s└── %s %s %s  %s\n",
					prefix,
					rule.Type,
					strings.Join(rule.Actions, ","),
					rule.Summary,
					formatMatch(rule.Match)))
			} else {
				sb.WriteString(fmt.Sprintf("%s├── %s %s %s  %s\n",
					prefix,
					rule.Type,
					strings.Join(rule.Actions, ","),
					rule.Summary,
					formatMatch(rule.Match)))
			}
		}
	}

	// Render unmatched rules
	if len(result.UnmatchedRules) > 0 {
		sb.WriteString("\nUnmatched rules:\n")
		for _, ur := range result.UnmatchedRules {
			sb.WriteString(fmt.Sprintf("- %s [%s]\n", ur.Summary, ur.Reason))
		}
	}

	fmt.Print(sb.String())
	return nil
}

// renderNodeHuman renders a node in human-readable format.
func renderNodeHuman(sb *strings.Builder, node RulesTreeNode, prefix string, isLast bool, maxDepth, depth int) {
	// Calculate connector
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Render node name with kind indicator
	suffix := "/"
	if node.Kind == "file" {
		suffix = ""
	}
	sb.WriteString(fmt.Sprintf("%s%s%s%s\n", prefix, connector, node.Path, suffix))

	// Calculate child prefix
	childPrefix := prefix + "│   "
	if isLast {
		childPrefix = prefix + "    "
	}

	// Render rules at this node
	for i, rule := range node.Rules {
		ruleConnector := "├── "
		if i == len(node.Rules)-1 && len(node.Children) == 0 {
			ruleConnector = "└── "
		}

		ruleType := "instruction"
		if rule.Type == "tool-trigger" {
			ruleType = "trigger"
		}

		triggerInfo := ""
		if rule.Trigger != nil {
			triggerInfo = fmt.Sprintf("[%s: %s]", rule.Trigger.Runtime, rule.Trigger.Entry)
		}

		sb.WriteString(fmt.Sprintf("%s%s%s %s %s  %s %s\n",
			childPrefix,
			ruleConnector,
			ruleType,
			strings.Join(rule.Actions, ","),
			rule.Summary,
			formatMatch(rule.Match),
			triggerInfo))
	}

	// Check max depth
	if maxDepth > 0 && depth >= maxDepth {
		if len(node.Children) > 0 {
			sb.WriteString(fmt.Sprintf("%s└── ... (%d more rules in subdirectories)\n",
				childPrefix, countChildRules(node.Children)))
		}
		return
	}

	// Render children
	for i, child := range node.Children {
		isLastChild := i == len(node.Children)-1
		renderNodeHuman(sb, child, childPrefix, isLastChild, maxDepth, depth+1)
	}
}

// countChildRules counts rules in child nodes.
func countChildRules(nodes []RulesTreeNode) int {
	count := 0
	for _, node := range nodes {
		count += len(node.Rules)
		count += countChildRules(node.Children)
	}
	return count
}

// formatMatch formats a match provenance for display.
func formatMatch(match *rulespkg.MatchProvenance) string {
	if match == nil {
		return ""
	}
	return fmt.Sprintf("[%s: %s]", match.Kind, match.Value)
}

// renderRulesTreeJSON renders the tree in JSON format.
func renderRulesTreeJSON(result RulesTreeResult, repoRoot string) error {
	output := struct {
		GitRoot string            `json:"git_root"`
		RulesTreeResult
	}{
		GitRoot:         repoRoot,
		RulesTreeResult: result,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
