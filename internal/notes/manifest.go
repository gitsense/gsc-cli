/**
 * Component: Notes Manifest Projection
 * Block-UUID: d0e1f2a3-b4c5-6789-defa-890123456789
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Projects notes into the gsc-notes Brain manifest for file-level querying.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

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
	fieldNoteIDs           = "F3000"
	fieldNoteSummaries     = "F3001"
	fieldNoteContent       = "F3002"
	fieldGlobPatterns      = "F3003"
	fieldTags              = "F3004"
	fieldKeywords          = "F3005"
	fieldParentKeywords    = "F3006"
	fieldImportance        = "F3007"
	fieldLinkedFiles       = "F3008"
	fieldLatestNoteAt      = "F3009"
	fieldLatestNoteSummary = "F3010"
	fieldNoteCount         = "F3011"
	fieldPurpose           = "F3012"
)

type projection struct {
	FilePath       string
	NoteIDs        []string
	Summaries      []string
	Content        []string
	GlobPatterns   []string
	Tags           []string
	Keywords       []string
	ParentKeywords []string
	Importance     []string
	LinkedFiles    []string
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

func RebuildManifestFromRecords(records []Note) (string, error) {
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

func BuildManifest(records []Note) manifest.ManifestFile {
	now := time.Now().UTC()
	repoName := repoName()
	projections := map[string]*projection{}

	for _, record := range records {
		// For notes, project onto glob patterns and linked files
		var targets []string
		targets = append(targets, record.GlobPatterns...)
		targets = append(targets, record.LinkedFiles...)
		if len(targets) == 0 {
			targets = syntheticTargets(record)
		}
		for _, target := range targets {
			p := projections[target]
			if p == nil {
				p = &projection{FilePath: target}
				projections[target] = p
			}
			p.NoteIDs = append(p.NoteIDs, record.ID)
			p.Summaries = append(p.Summaries, record.Summary)
			p.Content = append(p.Content, record.Content)
			p.GlobPatterns = append(p.GlobPatterns, record.GlobPatterns...)
			p.Tags = append(p.Tags, record.Tags...)
			p.Keywords = append(p.Keywords, record.Keywords...)
			p.ParentKeywords = append(p.ParentKeywords, record.ParentKeywords...)
			p.Importance = append(p.Importance, record.Importance)
			p.LinkedFiles = append(p.LinkedFiles, record.LinkedFiles...)
			p.Count++
			if record.CreatedAt.After(p.Latest) {
				p.Latest = record.CreatedAt
				p.LatestSummary = record.Summary
			}
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
			RepoRef:   "R3000",
			BranchRef: "B3000",
			FilePath:  p.FilePath,
			Language:  "",
			ChatID:    300000 + i + 1,
			Fields: map[string]interface{}{
				fieldNoteIDs:           normalizeStringList(p.NoteIDs),
				fieldNoteSummaries:     normalizeStringList(p.Summaries),
				fieldNoteContent:       normalizeStringList(p.Content),
				fieldGlobPatterns:      normalizeStringList(p.GlobPatterns),
				fieldTags:              normalizeStringList(p.Tags),
				fieldKeywords:          normalizeStringList(p.Keywords),
				fieldParentKeywords:    normalizeStringList(p.ParentKeywords),
				fieldImportance:        normalizeStringList(p.Importance),
				fieldLinkedFiles:       normalizeStringList(p.LinkedFiles),
				fieldLatestNoteAt:      p.Latest.Format(time.RFC3339),
				fieldLatestNoteSummary: p.LatestSummary,
				fieldNoteCount:         p.Count,
				fieldPurpose:           "",
			},
		})
	}

	return manifest.ManifestFile{
		SchemaVersion: "1.0.0",
		GeneratedAt:   now,
		Manifest: manifest.ManifestInfo{
			ManifestName: "GitSense Notes",
			DatabaseName: DatabaseName,
			Description:  "Searchable scratchpad notes for coding agents",
			Tags:         []string{"gsc-notes", "repository-notes", "agent-notes"},
		},
		Repositories: []manifest.Repository{{Ref: "R3000", Name: repoName}},
		Branches:     []manifest.Branch{{Ref: "B3000", Name: branchName()}},
		Analyzers: []manifest.Analyzer{{
			Ref:         "A3000",
			ID:          "gsc-notes",
			Name:        "GitSense Notes",
			Description: "Projects notes into file-level metadata for agent queries",
			Version:     "1.0.0",
		}},
		Fields: noteFields(),
		Data:   data,
	}
}

func noteFields() []manifest.Field {
	fields := []struct{ ref, name, display, typ, desc string }{
		{fieldNoteIDs, "note_ids", "Note IDs", "array", "Stable note IDs included in this projection"},
		{fieldNoteSummaries, "note_summaries", "Note Summaries", "array", "Concise note summaries"},
		{fieldNoteContent, "note_content", "Note Content", "array", "Note content"},
		{fieldGlobPatterns, "glob_patterns", "Glob Patterns", "array", "Glob patterns that matched this file"},
		{fieldTags, "tags", "Tags", "array", "Note tags"},
		{fieldKeywords, "keywords", "Keywords", "array", "Queryable note keywords"},
		{fieldParentKeywords, "parent_keywords", "Parent Keywords", "array", "Broader note domains"},
		{fieldImportance, "importance_levels", "Importance Levels", "array", "Importance values for projected notes"},
		{fieldLinkedFiles, "linked_files", "Linked Files", "array", "Files related to this note"},
		{fieldLatestNoteAt, "latest_note_at", "Latest Note At", "string", "Timestamp of the most recent note for this file"},
		{fieldLatestNoteSummary, "latest_note_summary", "Latest Note Summary", "string", "Most recent note summary as a single string — safe for gsc rg overlays"},
		{fieldNoteCount, "note_count", "Note Count", "integer", "Total number of notes projected to this file — use to decide whether to dig deeper"},
		{fieldPurpose, "purpose", "Purpose", "string", "Reserved for GitSense Chat — a synthesized description of this file's role based on all notes"},
	}
	out := make([]manifest.Field, 0, len(fields))
	for _, field := range fields {
		out = append(out, manifest.Field{
			Ref:         field.ref,
			AnalyzerRef: "A3000",
			Name:        field.name,
			DisplayName: field.display,
			Type:        field.typ,
			Description: field.desc,
		})
	}
	return out
}

func syntheticTargets(record Note) []string {
	if len(record.Tags) > 0 {
		return []string{".gitsense/notes/topics/" + record.Tags[0]}
	}
	return []string{".gitsense/notes/topics/general"}
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
