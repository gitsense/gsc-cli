/**
 * Component: Lessons Scoped Store
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Loads lesson records from scoped storage (repo, personal, or both) with source provenance. Adds target-based write helpers.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package lessons

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

// LoadRecordsFromSourcedDir loads lesson records from a single sourced directory.
// Missing records file is allowed and returns no records.
func LoadRecordsFromSourcedDir(dir gitsensescope.SourcedDir) ([]SourcedLesson, error) {
	recordsPath := gitsensescope.RecordsPath(dir, gitsensescope.KindLessons)
	records, err := LoadRecordsFromPath(recordsPath, true)
	if err != nil {
		return nil, err
	}
	sourced := make([]SourcedLesson, len(records))
	for i, r := range records {
		sourced[i] = SourcedLesson{Source: dir.Source, Lesson: r}
	}
	return sourced, nil
}

// LoadRecordsFromScope loads lesson records from all directories in the given scope.
// Ordering: repo first, then personal.
func LoadRecordsFromScope(scope gitsensescope.Scope) ([]SourcedLesson, error) {
	dirs, err := gitsensescope.GitSenseDirs(scope)
	if err != nil {
		return nil, err
	}
	return loadFromDirs(dirs)
}

// LoadRecordsFromScopeForRepo loads lesson records with an explicit repo root override.
func LoadRecordsFromScopeForRepo(scope gitsensescope.Scope, repoRoot string) ([]SourcedLesson, error) {
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
		var all []SourcedLesson

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
func loadFromDirs(dirs []gitsensescope.SourcedDir) ([]SourcedLesson, error) {
	var all []SourcedLesson
	for _, dir := range dirs {
		records, err := LoadRecordsFromSourcedDir(dir)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	return all, nil
}

// UnwrapSourcedLessons extracts plain Record slice from SourcedLesson slice.
func UnwrapSourcedLessons(sourced []SourcedLesson) []Record {
	records := make([]Record, len(sourced))
	for i, sl := range sourced {
		records[i] = sl.Lesson
	}
	return records
}

// --- Target path helpers ---

// RecordsPathForTarget returns the records JSONL path for a write target.
func RecordsPathForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.RecordsPath(base, gitsensescope.KindLessons), nil
}

// ManifestPathForTarget returns the manifest path for a write target.
func ManifestPathForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.ManifestPath(base, DatabaseName), nil
}

// LessonsDirForTarget returns the lessons directory for a write target.
func LessonsDirForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(base.Path, string(gitsensescope.KindLessons)), nil
}

// ArchiveDirForTarget returns the committed draft archive directory for a write target.
func ArchiveDirForTarget(target gitsensescope.Target) (string, error) {
	dir, err := LessonsDirForTarget(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "archive"), nil
}

// --- Targeted store mutators ---

// LoadRecordsFromTarget loads lesson records from a write target.
func LoadRecordsFromTarget(target gitsensescope.Target) ([]Record, error) {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return nil, err
	}
	return LoadRecordsFromPath(path, true)
}

// AppendRecordToTarget appends a lesson record to the target store.
func AppendRecordToTarget(record Record, target gitsensescope.Target) error {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return err
	}
	return appendRecordToPath(record, path)
}

// WriteRecordsToTarget writes all lesson records to the target store.
func WriteRecordsToTarget(records []Record, target gitsensescope.Target) error {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return err
	}
	return writeRecordsToPath(records, path)
}

// DeleteRecordFromTarget deletes a lesson record from the target store.
func DeleteRecordFromTarget(id string, target gitsensescope.Target) (bool, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return false, err
	}
	var kept []Record
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

// ResolveRecordFromTarget finds a record by ID or prefix within a target store.
func ResolveRecordFromTarget(idOrPrefix string, target gitsensescope.Target) (*Record, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return nil, err
	}
	return resolveRecordFromRecords(idOrPrefix, records)
}

// ResolveSourcedRecordFromRecords finds a sourced record by ID or prefix.
// Returns the SourcedLesson to preserve source provenance.
// Errors on ambiguity across sources.
func ResolveSourcedRecordFromRecords(idOrPrefix string, records []SourcedLesson) (*SourcedLesson, error) {
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}

	var exact []SourcedLesson
	for _, record := range records {
		if record.Lesson.ID == q {
			exact = append(exact, record)
		}
	}
	switch len(exact) {
	case 1:
		return &exact[0], nil
	case 0:
	default:
		return nil, fmt.Errorf("ambiguous lesson id %q matches %d lessons across repo and personal scopes; use --scope repo or --scope personal", idOrPrefix, len(exact))
	}

	// Substring/prefix match
	var matches []SourcedLesson
	for _, sl := range records {
		if strings.Contains(sl.Lesson.ID, q) {
			matches = append(matches, sl)
		}
	}

	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		// Check if all matches are from the same source
		sources := make(map[gitsensescope.Source]bool)
		for _, m := range matches {
			sources[m.Source] = true
		}
		if len(sources) > 1 {
			return nil, fmt.Errorf("ambiguous lesson id %q matches %d lessons across repo and personal scopes; use --scope repo or --scope personal", idOrPrefix, len(matches))
		}
		return nil, fmt.Errorf("ambiguous lesson id %q matches %d lessons; use a longer prefix", idOrPrefix, len(matches))
	}
}

// resolveRecordFromRecords finds a record by ID or prefix within a record slice.
func resolveRecordFromRecords(idOrPrefix string, records []Record) (*Record, error) {
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}
	for i := range records {
		if records[i].ID == q {
			return &records[i], nil
		}
	}
	var matches []Record
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
		return nil, fmt.Errorf("ambiguous lesson id %q matches %d lessons; use a longer prefix or specify --scope", idOrPrefix, len(matches))
	}
}

// RebuildAndImportForTarget rebuilds the manifest and imports the Brain for a target.
func RebuildAndImportForTarget(target gitsensescope.Target) error {
	manifestPath, err := RebuildManifestFromRecordsForTarget(target)
	if err != nil {
		return err
	}
	// Only import Brain for repo target
	if target == gitsensescope.TargetRepo {
		return manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false)
	}
	// Personal target: manifest generated but Brain import not supported yet
	return nil
}

// RebuildAndImportRecordsForTarget rebuilds the target manifest from supplied records and imports repo targets.
func RebuildAndImportRecordsForTarget(records []Record, target gitsensescope.Target) error {
	manifestPath, err := RebuildManifestRecordsForTarget(records, target)
	if err != nil {
		return err
	}
	if target == gitsensescope.TargetRepo {
		return manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false)
	}
	return nil
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

// RebuildManifestRecordsForTarget rebuilds the target manifest from supplied records.
func RebuildManifestRecordsForTarget(records []Record, target gitsensescope.Target) (string, error) {
	path, err := ManifestPathForTarget(target)
	if err != nil {
		return "", err
	}
	return rebuildManifestAtPath(records, path)
}

// --- Internal helpers ---

func appendRecordToPath(record Record, path string) error {
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

func writeRecordsToPath(records []Record, path string) error {
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

func rebuildManifestAtPath(records []Record, path string) (string, error) {
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

// --- Sourced query helpers ---

// SearchSourcedRecords searches sourced lessons by query string.
func SearchSourcedRecords(records []SourcedLesson, query string, fields []string) []SourcedLesson {
	grouped := groupBySource(records)
	var result []SourcedLesson
	for _, g := range grouped {
		searched := SearchRecords(g.records, query, fields)
		for _, r := range searched {
			result = append(result, SourcedLesson{Source: g.source, Lesson: r})
		}
	}
	return result
}

// FilterSourcedRecords filters sourced lessons by ListFilter, preserving source.
func FilterSourcedRecords(records []SourcedLesson, filter ListFilter) []SourcedLesson {
	grouped := groupBySource(records)
	var result []SourcedLesson
	for _, g := range grouped {
		filtered := FilterRecords(g.records, filter)
		for _, r := range filtered {
			result = append(result, SourcedLesson{Source: g.source, Lesson: r})
		}
	}
	return result
}

// GetSourcedLessonsForFile returns sourced lessons matching a specific file path.
func GetSourcedLessonsForFile(records []SourcedLesson, file string) []SourcedLesson {
	grouped := groupBySource(records)
	var result []SourcedLesson
	for _, g := range grouped {
		filtered := FilterRecords(g.records, ListFilter{File: file})
		for _, r := range filtered {
			result = append(result, SourcedLesson{Source: g.source, Lesson: r})
		}
	}
	return result
}

// GetSourcedLessonsForTag returns sourced lessons with a specific tag.
func GetSourcedLessonsForTag(records []SourcedLesson, tag string) []SourcedLesson {
	grouped := groupBySource(records)
	var result []SourcedLesson
	for _, g := range grouped {
		filtered := FilterRecords(g.records, ListFilter{Tag: tag})
		for _, r := range filtered {
			result = append(result, SourcedLesson{Source: g.source, Lesson: r})
		}
	}
	return result
}

// sourceGroup holds records from a single source for batch processing.
type sourceGroup struct {
	source  gitsensescope.Source
	records []Record
}

// groupBySource groups sourced lessons by source, preserving order.
func groupBySource(records []SourcedLesson) []sourceGroup {
	type entry struct {
		source  gitsensescope.Source
		records []Record
	}
	var order []gitsensescope.Source
	groups := make(map[gitsensescope.Source]*entry)

	for _, sl := range records {
		if _, ok := groups[sl.Source]; !ok {
			groups[sl.Source] = &entry{source: sl.Source}
			order = append(order, sl.Source)
		}
		groups[sl.Source].records = append(groups[sl.Source].records, sl.Lesson)
	}

	result := make([]sourceGroup, 0, len(order))
	for _, src := range order {
		g := groups[src]
		result = append(result, sourceGroup{source: g.source, records: g.records})
	}
	return result
}
