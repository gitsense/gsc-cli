/**
 * Component: Rules Scoped Store
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Loads rule records from scoped storage (repo, personal, or both) with source provenance. Adds target-based write helpers.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	"github.com/gitsense/gsc-cli/internal/manifest"
)

// LoadRecordsFromSourcedDir loads rule records from a single sourced directory.
// Missing records file is allowed and returns no records.
func LoadRecordsFromSourcedDir(dir gitsensescope.SourcedDir) ([]SourcedRule, error) {
	recordsPath := gitsensescope.RecordsPath(dir, gitsensescope.KindRules)
	records, err := LoadRecordsFromPath(recordsPath, true)
	if err != nil {
		return nil, err
	}
	sourced := make([]SourcedRule, len(records))
	for i, r := range records {
		sourced[i] = SourcedRule{Source: dir.Source, Rule: r}
	}
	return sourced, nil
}

// LoadRecordsFromScope loads rule records from all directories in the given scope.
// Ordering: repo first, then personal.
func LoadRecordsFromScope(scope gitsensescope.Scope) ([]SourcedRule, error) {
	dirs, err := gitsensescope.GitSenseDirs(scope)
	if err != nil {
		return nil, err
	}
	return loadFromDirs(dirs)
}

// LoadRecordsFromScopeForRepo loads rule records with an explicit repo root override.
// If repoRoot is non-empty, it overrides the repo directory to <repoRoot>/.gitsense.
// Personal remains $GSC_HOME. For ScopeAll, includes discovered repo + personal.
// For ScopeRepo, uses discovered repo only.
func LoadRecordsFromScopeForRepo(scope gitsensescope.Scope, repoRoot string) ([]SourcedRule, error) {
	if repoRoot == "" {
		return LoadRecordsFromScope(scope)
	}

	switch scope {
	case gitsensescope.ScopeRepo:
		dir := gitsensescope.SourcedDir{
			Source: gitsensescope.SourceRepo,
			Path:   gitsensescope.RepoGitSenseDirForRoot(repoRoot),
		}
		return LoadRecordsFromSourcedDir(dir)

	case gitsensescope.ScopePersonal:
		personalPath, err := gitsensescope.PersonalGitSenseDir()
		if err != nil {
			return nil, err
		}
		dir := gitsensescope.SourcedDir{
			Source: gitsensescope.SourcePersonal,
			Path:   personalPath,
		}
		return LoadRecordsFromSourcedDir(dir)

	case gitsensescope.ScopeAll:
		var all []SourcedRule

		// Repo first
		repoDir := gitsensescope.SourcedDir{
			Source: gitsensescope.SourceRepo,
			Path:   gitsensescope.RepoGitSenseDirForRoot(repoRoot),
		}
		repoRecords, err := LoadRecordsFromSourcedDir(repoDir)
		if err != nil {
			return nil, err
		}
		all = append(all, repoRecords...)

		// Then personal
		personalPath, err := gitsensescope.PersonalGitSenseDir()
		if err != nil {
			return nil, err
		}
		personalDir := gitsensescope.SourcedDir{
			Source: gitsensescope.SourcePersonal,
			Path:   personalPath,
		}
		personalRecords, err := LoadRecordsFromSourcedDir(personalDir)
		if err != nil {
			return nil, err
		}
		all = append(all, personalRecords...)

		return all, nil

	default:
		return nil, nil
	}
}

// loadFromDirs loads records from multiple sourced directories, preserving order.
func loadFromDirs(dirs []gitsensescope.SourcedDir) ([]SourcedRule, error) {
	var all []SourcedRule
	for _, dir := range dirs {
		records, err := LoadRecordsFromSourcedDir(dir)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	return all, nil
}

// UnwrapSourcedRules extracts plain Rule slice from SourcedRule slice.
func UnwrapSourcedRules(sourced []SourcedRule) []Rule {
	rules := make([]Rule, len(sourced))
	for i, sr := range sourced {
		rules[i] = sr.Rule
	}
	return rules
}

// WrapMatchedRules wraps MatchedRules with a source.
func WrapMatchedRules(source gitsensescope.Source, matched []MatchedRule) []SourcedMatchedRule {
	sourced := make([]SourcedMatchedRule, len(matched))
	for i, mr := range matched {
		sourced[i] = SourcedMatchedRule{Source: source, MatchedRule: mr}
	}
	return sourced
}

// --- Target path helpers ---

// RecordsPathForTarget returns the records JSONL path for a write target.
func RecordsPathForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.RecordsPath(base, gitsensescope.KindRules), nil
}

// ManifestPathForTarget returns the manifest path for a write target.
func ManifestPathForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.ManifestPath(base, DatabaseName), nil
}

// RulesDirForTarget returns the rules directory for a write target.
func RulesDirForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(base.Path, string(gitsensescope.KindRules)), nil
}

// TriggersDirForTarget returns the triggers directory for a write target.
func TriggersDirForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.RulesTriggersDir(base), nil
}

// FixturesDirForTarget returns the fixtures directory for a write target.
func FixturesDirForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.RulesFixturesDir(base), nil
}

// SourceFromTarget converts a Target to a Source.
func SourceFromTarget(target gitsensescope.Target) gitsensescope.Source {
	switch target {
	case gitsensescope.TargetRepo:
		return gitsensescope.SourceRepo
	case gitsensescope.TargetPersonal:
		return gitsensescope.SourcePersonal
	default:
		return gitsensescope.SourceRepo
	}
}

// --- Targeted store mutators ---

// LoadRecordsFromTarget loads rule records from a write target.
func LoadRecordsFromTarget(target gitsensescope.Target) ([]Rule, error) {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return nil, err
	}
	return LoadRecordsFromPath(path, true)
}

// AppendRecordToTarget appends a rule record to the target store.
func AppendRecordToTarget(record Rule, target gitsensescope.Target) error {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return err
	}
	return appendRecordToPath(record, path)
}

// WriteRecordsToTarget writes all rule records to the target store.
func WriteRecordsToTarget(records []Rule, target gitsensescope.Target) error {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return err
	}
	return writeRecordsToPath(records, path)
}

// DeleteRecordFromTarget deletes a rule record from the target store.
func DeleteRecordFromTarget(id string, target gitsensescope.Target) (bool, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return false, err
	}
	var kept []Rule
	deleted := false
	for _, record := range records {
		if record.ID == id {
			deleted = true
			continue
		}
		kept = append(kept, record)
	}
	if !deleted {
		return false, nil
	}
	return true, WriteRecordsToTarget(kept, target)
}

// ResolveRecordFromTarget finds a record in a target store by exact ID or unique substring.
func ResolveRecordFromTarget(idOrPrefix string, target gitsensescope.Target) (*Rule, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return nil, err
	}
	return ResolveRecordFromRecords(idOrPrefix, records)
}

// ResolveSourcedRecordFromRecords finds a sourced record by exact ID or unique substring.
func ResolveSourcedRecordFromRecords(idOrPrefix string, records []SourcedRule) (*SourcedRule, error) {
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}

	var exact []SourcedRule
	for _, record := range records {
		if record.Rule.ID == q {
			exact = append(exact, record)
		}
	}
	switch len(exact) {
	case 1:
		return &exact[0], nil
	case 0:
	default:
		return nil, fmt.Errorf("ambiguous rule id %q matches %d rules across scopes; use --scope repo or --scope personal", idOrPrefix, len(exact))
	}

	var matches []SourcedRule
	for _, record := range records {
		if strings.Contains(record.Rule.ID, q) {
			matches = append(matches, record)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous rule id %q matches %d rules across scopes; use a longer prefix or specify --scope", idOrPrefix, len(matches))
	}
}

// ResolveRecordFromRecords finds a record by exact ID or unique substring.
func ResolveRecordFromRecords(idOrPrefix string, records []Rule) (*Rule, error) {
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}
	for i := range records {
		if records[i].ID == q {
			return &records[i], nil
		}
	}
	var matches []Rule
	for _, r := range records {
		if strings.Contains(r.ID, q) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous rule id %q matches %d rules; use a longer prefix", idOrPrefix, len(matches))
	}
}

// RebuildAndImportForTarget rebuilds the manifest and imports the Brain for a target.
func RebuildAndImportForTarget(target gitsensescope.Target) error {
	if target == gitsensescope.TargetPersonal {
		_, err := RebuildManifestFromRecordsForTarget(target)
		return err
	}
	manifestPath, err := RebuildManifestFromRecordsForTarget(target)
	if err != nil {
		return err
	}
	return manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false)
}

// RebuildManifestFromRecordsForTarget rebuilds the manifest for a target.
func RebuildManifestFromRecordsForTarget(target gitsensescope.Target) (string, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return "", err
	}
	path, err := ManifestPathForTarget(target)
	if err != nil {
		return "", err
	}
	return rebuildManifestAtPath(records, path)
}

// --- Internal helpers ---

func appendRecordToPath(record Rule, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func writeRecordsToPath(records []Rule, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func rebuildManifestAtPath(records []Rule, path string) (string, error) {
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

// --- Scoped matching helpers ---

// GetSourcedRulesForFile returns sourced rules matching a specific file path.
func GetSourcedRulesForFile(records []SourcedRule, file, action string, event LifecycleEvent) []SourcedMatchedRule {
	grouped := groupBySource(records)
	var result []SourcedMatchedRule
	for _, g := range grouped {
		matched := GetRulesForFile(g.rules, file, action, event)
		result = append(result, WrapMatchedRules(g.source, matched)...)
	}
	return result
}

// GetSourcedRulesForFileAllActions returns sourced rules matching a file path (all actions).
func GetSourcedRulesForFileAllActions(records []SourcedRule, file string, event LifecycleEvent) []SourcedMatchedRule {
	return GetSourcedRulesForFile(records, file, "", event)
}

// GetSourcedRulesForGlob returns sourced rules matching a glob pattern.
func GetSourcedRulesForGlob(records []SourcedRule, glob string, event LifecycleEvent) []SourcedMatchedRule {
	grouped := groupBySource(records)
	var result []SourcedMatchedRule
	for _, g := range grouped {
		matched := GetRulesForGlob(g.rules, glob, event)
		result = append(result, WrapMatchedRules(g.source, matched)...)
	}
	return result
}

// GetSourcedRulesForTag returns sourced rules with a specific tag.
func GetSourcedRulesForTag(records []SourcedRule, tag string, event LifecycleEvent) []SourcedMatchedRule {
	grouped := groupBySource(records)
	var result []SourcedMatchedRule
	for _, g := range grouped {
		matched := GetRulesForTag(g.rules, tag, event)
		result = append(result, WrapMatchedRules(g.source, matched)...)
	}
	return result
}

// GetSourcedRulesForAction returns sourced rules matching a specific action.
func GetSourcedRulesForAction(records []SourcedRule, action string, event LifecycleEvent) []SourcedMatchedRule {
	grouped := groupBySource(records)
	var result []SourcedMatchedRule
	for _, g := range grouped {
		matched := GetRulesForAction(g.rules, action, event)
		result = append(result, WrapMatchedRules(g.source, matched)...)
	}
	return result
}

// FilterSourcedRecords filters sourced records by ListFilter, preserving source.
func FilterSourcedRecords(records []SourcedRule, filter ListFilter) []SourcedRule {
	grouped := groupBySource(records)
	var result []SourcedRule
	for _, g := range grouped {
		filtered := FilterRecords(g.rules, filter)
		for _, r := range filtered {
			result = append(result, SourcedRule{Source: g.source, Rule: r})
		}
	}
	return result
}

// SearchSourcedRecords searches sourced rules by query string.
func SearchSourcedRecords(records []SourcedRule, query string) []SourcedRule {
	grouped := groupBySource(records)
	var result []SourcedRule
	for _, g := range grouped {
		searched := SearchRecords(g.rules, query, nil)
		for _, r := range searched {
			result = append(result, SourcedRule{Source: g.source, Rule: r})
		}
	}
	return result
}

// sourceGroup holds records from a single source for batch processing.
type sourceGroup struct {
	source gitsensescope.Source
	rules  []Rule
}

// groupBySource groups sourced rules by source, preserving order.
func groupBySource(records []SourcedRule) []sourceGroup {
	type entry struct {
		source gitsensescope.Source
		rules  []Rule
	}
	var order []gitsensescope.Source
	groups := make(map[gitsensescope.Source]*entry)

	for _, sr := range records {
		if _, ok := groups[sr.Source]; !ok {
			groups[sr.Source] = &entry{source: sr.Source}
			order = append(order, sr.Source)
		}
		groups[sr.Source].rules = append(groups[sr.Source].rules, sr.Rule)
	}

	result := make([]sourceGroup, 0, len(order))
	for _, src := range order {
		g := groups[src]
		result = append(result, sourceGroup{source: g.source, rules: g.rules})
	}
	return result
}
