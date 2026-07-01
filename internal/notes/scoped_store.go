/**
 * Component: Notes Scoped Store
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Loads note records from scoped storage (repo, personal, or both) with source provenance. Adds target-based write helpers.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package notes

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

// LoadRecordsFromSourcedDir loads note records from a single sourced directory.
// Missing records file is allowed and returns no records.
func LoadRecordsFromSourcedDir(dir gitsensescope.SourcedDir) ([]SourcedNote, error) {
	recordsPath := gitsensescope.RecordsPath(dir, gitsensescope.KindNotes)
	records, err := LoadRecordsFromPath(recordsPath, true)
	if err != nil {
		return nil, err
	}
	sourced := make([]SourcedNote, len(records))
	for i, r := range records {
		sourced[i] = SourcedNote{Source: dir.Source, Note: r}
	}
	return sourced, nil
}

// LoadRecordsFromScope loads note records from all directories in the given scope.
// Ordering: repo first, then personal.
func LoadRecordsFromScope(scope gitsensescope.Scope) ([]SourcedNote, error) {
	dirs, err := gitsensescope.GitSenseDirs(scope)
	if err != nil {
		return nil, err
	}
	return loadFromDirs(dirs)
}

// LoadRecordsFromScopeForRepo loads note records with an explicit repo root override.
func LoadRecordsFromScopeForRepo(scope gitsensescope.Scope, repoRoot string) ([]SourcedNote, error) {
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
		var all []SourcedNote

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
func loadFromDirs(dirs []gitsensescope.SourcedDir) ([]SourcedNote, error) {
	var all []SourcedNote
	for _, dir := range dirs {
		records, err := LoadRecordsFromSourcedDir(dir)
		if err != nil {
			return nil, err
		}
		all = append(all, records...)
	}
	return all, nil
}

// UnwrapSourcedNotes extracts plain Note slice from SourcedNote slice.
func UnwrapSourcedNotes(sourced []SourcedNote) []Note {
	notes := make([]Note, len(sourced))
	for i, sn := range sourced {
		notes[i] = sn.Note
	}
	return notes
}

// WrapMatchedNotes wraps MatchedNotes with a source.
func WrapMatchedNotes(source gitsensescope.Source, matched []MatchedNote) []SourcedMatchedNote {
	sourced := make([]SourcedMatchedNote, len(matched))
	for i, mn := range matched {
		sourced[i] = SourcedMatchedNote{Source: source, MatchedNote: mn}
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
	return gitsensescope.RecordsPath(base, gitsensescope.KindNotes), nil
}

// ManifestPathForTarget returns the manifest path for a write target.
func ManifestPathForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return gitsensescope.ManifestPath(base, DatabaseName), nil
}

// NotesDirForTarget returns the notes directory for a write target.
func NotesDirForTarget(target gitsensescope.Target) (string, error) {
	base, err := gitsensescope.GitSenseDirForTarget(target)
	if err != nil {
		return "", err
	}
	return filepath.Join(base.Path, string(gitsensescope.KindNotes)), nil
}

// --- Targeted store mutators ---

// LoadRecordsFromTarget loads note records from a write target.
func LoadRecordsFromTarget(target gitsensescope.Target) ([]Note, error) {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return nil, err
	}
	return LoadRecordsFromPath(path, true)
}

// AppendRecordToTarget appends a note record to the target store.
func AppendRecordToTarget(record Note, target gitsensescope.Target) error {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return err
	}
	return appendRecordToPath(record, path)
}

// WriteRecordsToTarget writes all note records to the target store.
func WriteRecordsToTarget(records []Note, target gitsensescope.Target) error {
	path, err := RecordsPathForTarget(target)
	if err != nil {
		return err
	}
	return writeRecordsToPath(records, path)
}

// DeleteRecordFromTarget deletes a note record from the target store.
func DeleteRecordFromTarget(id string, target gitsensescope.Target) (bool, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return false, err
	}
	var kept []Note
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
func ResolveRecordFromTarget(idOrPrefix string, target gitsensescope.Target) (*Note, error) {
	records, err := LoadRecordsFromTarget(target)
	if err != nil {
		return nil, err
	}
	return ResolveRecordFromRecords(idOrPrefix, records)
}

// ResolveSourcedRecordFromRecords finds a sourced record by exact ID or unique substring/prefix.
func ResolveSourcedRecordFromRecords(idOrPrefix string, records []SourcedNote) (*SourcedNote, error) {
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}

	var exact []SourcedNote
	for _, record := range records {
		if record.Note.ID == q {
			exact = append(exact, record)
		}
	}
	switch len(exact) {
	case 1:
		return &exact[0], nil
	case 0:
	default:
		return nil, fmt.Errorf("ambiguous note id %q matches %d notes across scopes; use --scope repo or --scope personal", idOrPrefix, len(exact))
	}

	var matches []SourcedNote
	for _, record := range records {
		if strings.Contains(record.Note.ID, q) {
			matches = append(matches, record)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous note id %q matches %d notes across scopes; use a longer prefix or specify --scope", idOrPrefix, len(matches))
	}
}

// ResolveRecordFromRecords finds a record by ID or prefix within a record slice.
func ResolveRecordFromRecords(idOrPrefix string, records []Note) (*Note, error) {
	q := strings.TrimSpace(idOrPrefix)
	if q == "" {
		return nil, nil
	}
	for i := range records {
		if records[i].ID == q {
			return &records[i], nil
		}
	}
	var matches []Note
	for _, n := range records {
		if strings.Contains(n.ID, q) {
			matches = append(matches, n)
		}
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous note id %q matches %d notes; use a longer prefix or specify --scope", idOrPrefix, len(matches))
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

func appendRecordToPath(record Note, path string) error {
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

func writeRecordsToPath(records []Note, path string) error {
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

func rebuildManifestAtPath(records []Note, path string) (string, error) {
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

// --- Scoped query helpers ---

// GetSourcedNotesForFile returns sourced notes matching a specific file path.
func GetSourcedNotesForFile(records []SourcedNote, file string) []SourcedMatchedNote {
	grouped := groupBySource(records)
	var result []SourcedMatchedNote
	for _, g := range grouped {
		matched := GetNotesForFile(g.notes, file)
		result = append(result, WrapMatchedNotes(g.source, matched)...)
	}
	return result
}

// GetSourcedNotesForGlob returns sourced notes matching a glob pattern.
func GetSourcedNotesForGlob(records []SourcedNote, glob string) []SourcedMatchedNote {
	grouped := groupBySource(records)
	var result []SourcedMatchedNote
	for _, g := range grouped {
		matched := GetNotesForGlob(g.notes, glob)
		result = append(result, WrapMatchedNotes(g.source, matched)...)
	}
	return result
}

// GetSourcedNotesForTag returns sourced notes with a specific tag.
func GetSourcedNotesForTag(records []SourcedNote, tag string) []SourcedMatchedNote {
	grouped := groupBySource(records)
	var result []SourcedMatchedNote
	for _, g := range grouped {
		matched := GetNotesForTag(g.notes, tag)
		result = append(result, WrapMatchedNotes(g.source, matched)...)
	}
	return result
}

// SearchSourcedRecords searches sourced notes by query string.
func SearchSourcedRecords(records []SourcedNote, query string) []SourcedNote {
	grouped := groupBySource(records)
	var result []SourcedNote
	for _, g := range grouped {
		searched := SearchRecords(g.notes, query)
		for _, n := range searched {
			result = append(result, SourcedNote{Source: g.source, Note: n})
		}
	}
	return result
}

// FilterSourcedRecords filters sourced notes by ListFilter, preserving source.
func FilterSourcedRecords(records []SourcedNote, filter ListFilter) []SourcedNote {
	grouped := groupBySource(records)
	var result []SourcedNote
	for _, g := range grouped {
		filtered := FilterRecords(g.notes, filter)
		for _, n := range filtered {
			result = append(result, SourcedNote{Source: g.source, Note: n})
		}
	}
	return result
}

// sourceGroup holds notes from a single source for batch processing.
type sourceGroup struct {
	source gitsensescope.Source
	notes  []Note
}

// groupBySource groups sourced notes by source, preserving order.
func groupBySource(records []SourcedNote) []sourceGroup {
	type entry struct {
		source gitsensescope.Source
		notes  []Note
	}
	var order []gitsensescope.Source
	groups := make(map[gitsensescope.Source]*entry)

	for _, sn := range records {
		if _, ok := groups[sn.Source]; !ok {
			groups[sn.Source] = &entry{source: sn.Source}
			order = append(order, sn.Source)
		}
		groups[sn.Source].notes = append(groups[sn.Source].notes, sn.Note)
	}

	result := make([]sourceGroup, 0, len(order))
	for _, src := range order {
		g := groups[src]
		result = append(result, sourceGroup{source: g.source, notes: g.notes})
	}
	return result
}
