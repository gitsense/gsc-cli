/**
 * Component: Rules Manifest Projection
 * Block-UUID: f8a9b0c1-d2e3-4567-abcd-567890123456
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Projects rules into the gsc-rules Brain manifest for file-level querying.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/manifest"
)

const (
	fieldRuleIDs           = "F2000"
	fieldRuleSummaries     = "F2001"
	fieldRuleDetails       = "F2002"
	fieldInstructions      = "F2003"
	fieldInstructionTags   = "F2004"
	fieldGlobPatterns      = "F2005"
	fieldExcludeGlobs      = "F2006"
	fieldImportance        = "F2007"
	fieldTags              = "F2008"
	fieldKeywords          = "F2009"
	fieldParentKeywords    = "F2010"
	fieldOwners            = "F2011"
	fieldContacts          = "F2012"
	fieldLatestRuleAt      = "F2013"
	fieldLatestRuleSummary = "F2014"
	fieldRuleCount         = "F2015"
	fieldPurpose           = "F2016"
)

type projection struct {
	FilePath       string
	RuleIDs        []string
	Summaries      []string
	Details        []string
	Instructions   []string
	InstructionTags []string
	GlobPatterns   []string
	ExcludeGlobs   []string
	Importance     []string
	Tags           []string
	Keywords       []string
	ParentKeywords []string
	Owners         []string
	Contacts       []string
	Latest         time.Time
	LatestSummary  string
	Count          int
}

func RebuildManifest() (string, error) {
	records, err := LoadRecords()
	if err != nil {
		return "", err
	}
	return RebuildManifestFromRecords(records)
}

// RebuildAndImport rebuilds the manifest and imports the Brain.
func RebuildAndImport() error {
	manifestPath, err := RebuildManifest()
	if err != nil {
		return err
	}
	return manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false)
}

func RebuildManifestFromRecords(records []Rule) (string, error) {
	path, err := ManifestPath()
	if err != nil {
		return "", err
	}
	mf := BuildManifest(records)
	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func BuildManifest(records []Rule) manifest.ManifestFile {
	now := time.Now().UTC()
	repoName := repoName()
	projections := map[string]*projection{}

	for _, record := range records {
		// For rules, project onto glob patterns and file paths
		var targets []string
		targets = append(targets, record.GlobPatterns...)
		targets = append(targets, record.AppliesTo.Files...)
		if len(targets) == 0 {
			targets = syntheticTargets(record)
		}
		for _, target := range targets {
			p := projections[target]
			if p == nil {
				p = &projection{FilePath: target}
				projections[target] = p
			}
			p.RuleIDs = append(p.RuleIDs, record.ID)
			p.Summaries = append(p.Summaries, record.Summary)
			p.Details = append(p.Details, record.Details)
			p.GlobPatterns = append(p.GlobPatterns, record.GlobPatterns...)
			p.ExcludeGlobs = append(p.ExcludeGlobs, record.ExcludeGlobs...)
			p.Importance = append(p.Importance, record.Importance)
			p.Tags = append(p.Tags, record.Tags...)
			p.Keywords = append(p.Keywords, record.Keywords...)
			p.ParentKeywords = append(p.ParentKeywords, record.ParentKeywords...)
			p.Owners = append(p.Owners, record.Owner)
			p.Contacts = append(p.Contacts, record.Contact...)
			p.Count++
			if record.CreatedAt.After(p.Latest) {
				p.Latest = record.CreatedAt
				p.LatestSummary = record.Summary
			}
			// Flatten instructions (now just strings)
			p.Instructions = append(p.Instructions, record.Instructions...)
		}
	}

	var paths []string
	for path := range projections {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var data []manifest.DataEntry
	for i, path := range paths {
		p := projections[path]
		data = append(data, manifest.DataEntry{
			RepoRef:   "R2000",
			BranchRef: "B2000",
			FilePath:  p.FilePath,
			Language:  "",
			ChatID:    200000 + i + 1,
			Fields: map[string]interface{}{
				fieldRuleIDs:           normalizeStringList(p.RuleIDs),
				fieldRuleSummaries:     normalizeStringList(p.Summaries),
				fieldRuleDetails:       normalizeStringList(p.Details),
				fieldInstructions:      normalizeStringList(p.Instructions),
				fieldInstructionTags:   normalizeStringList(p.InstructionTags),
				fieldGlobPatterns:      normalizeStringList(p.GlobPatterns),
				fieldExcludeGlobs:      normalizeStringList(p.ExcludeGlobs),
				fieldImportance:        normalizeStringList(p.Importance),
				fieldTags:              normalizeStringList(p.Tags),
				fieldKeywords:          normalizeStringList(p.Keywords),
				fieldParentKeywords:    normalizeStringList(p.ParentKeywords),
				fieldOwners:            normalizeStringList(p.Owners),
				fieldContacts:          normalizeStringList(p.Contacts),
				fieldLatestRuleAt:      p.Latest.Format(time.RFC3339),
				fieldLatestRuleSummary: p.LatestSummary,
				fieldRuleCount:         p.Count,
				fieldPurpose:           "",
			},
		})
	}

	return manifest.ManifestFile{
		SchemaVersion: "1.0.0",
		GeneratedAt:   now,
		Manifest: manifest.ManifestInfo{
			ManifestName: "GitSense Rules",
			DatabaseName: DatabaseName,
			Description:  "Queryable guardrails and conventions for coding agents",
			Tags:         []string{"gsc-rules", "repository-rules", "agent-guardrails"},
		},
		Repositories: []manifest.Repository{{Ref: "R2000", Name: repoName}},
		Branches:     []manifest.Branch{{Ref: "B2000", Name: branchName()}},
		Analyzers: []manifest.Analyzer{{
			Ref:         "A2000",
			ID:          "gsc-rules",
			Name:        "GitSense Rules",
			Description: "Projects rules into file-level metadata for agent queries",
			Version:     "1.0.0",
		}},
		Fields: ruleFields(),
		Data:   data,
	}
}

func ruleFields() []manifest.Field {
	fields := []struct{ ref, name, display, typ, desc string }{
		{fieldRuleIDs, "rule_ids", "Rule IDs", "array", "Stable rule IDs included in this projection"},
		{fieldRuleSummaries, "rule_summaries", "Rule Summaries", "array", "Concise rule summaries"},
		{fieldRuleDetails, "rule_details", "Rule Details", "array", "Detailed rule context"},
		{fieldInstructions, "instructions", "Instructions", "array", "Actionable instructions to follow"},
		{fieldInstructionTags, "instruction_tags", "Instruction Tags", "array", "Categories for instructions (formatting, safety, naming, etc.)"},
		{fieldGlobPatterns, "glob_patterns", "Glob Patterns", "array", "Glob patterns that matched this file"},
		{fieldExcludeGlobs, "exclude_globs", "Exclude Globs", "array", "Exclusion patterns"},
		{fieldImportance, "importance_levels", "Importance Levels", "array", "Importance values for projected rules"},
		{fieldTags, "tags", "Tags", "array", "Aggregated instruction tags"},
		{fieldKeywords, "keywords", "Keywords", "array", "Queryable rule keywords"},
		{fieldParentKeywords, "parent_keywords", "Parent Keywords", "array", "Broader rule domains"},
		{fieldOwners, "owners", "Owners", "array", "Rule owners"},
		{fieldContacts, "contacts", "Contacts", "array", "Who to contact when rules match"},
		{fieldLatestRuleAt, "latest_rule_at", "Latest Rule At", "string", "Timestamp of the most recent rule for this file"},
		{fieldLatestRuleSummary, "latest_rule_summary", "Latest Rule Summary", "string", "Most recent rule summary as a single string — safe for gsc rg overlays"},
		{fieldRuleCount, "rule_count", "Rule Count", "integer", "Total number of rules projected to this file — use to decide whether to dig deeper"},
		{fieldPurpose, "purpose", "Purpose", "string", "Reserved for GitSense Chat — a synthesized description of this file's role based on all rules"},
	}
	out := make([]manifest.Field, 0, len(fields))
	for _, field := range fields {
		out = append(out, manifest.Field{
			Ref:         field.ref,
			AnalyzerRef: "A2000",
			Name:        field.name,
			DisplayName: field.display,
			Type:        field.typ,
			Description: field.desc,
		})
	}
	return out
}

func syntheticTargets(record Rule) []string {
	if len(record.AppliesTo.Topics) > 0 {
		return []string{".gitsense/rules/topics/" + record.AppliesTo.Topics[0]}
	}
	if len(record.AppliesTo.Commands) > 0 {
		return []string{".gitsense/rules/commands/" + slugify(record.AppliesTo.Commands[0])}
	}
	if len(record.Tags) > 0 {
		return []string{".gitsense/rules/topics/" + record.Tags[0]}
	}
	return []string{".gitsense/rules/topics/general"}
}

func repoName() string {
	root, err := rootDir()
	if err != nil {
		return "repository"
	}
	return filepath.Base(root)
}

func branchName() string {
	root, err := rootDir()
	if err != nil {
		return "main"
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "main"
	}
	return branch
}
