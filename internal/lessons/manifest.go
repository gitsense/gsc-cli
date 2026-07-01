/**
 * Component: Lessons Manifest Projection
 * Block-UUID: b3e2f1a0-9c4d-4e7b-8f5a-2d1e6c3b0a9f
 * Parent-UUID: 1cf52586-45b5-48e9-b735-b82724e15d93
 * Version: 1.1.0
 * Description: Added three scalar fields for gsc rg overlays: latest_lesson_summary (most recent summary as a single string), lesson_count (total lessons projected to the file), and purpose (reserved for GitSense Chat synthesis).
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), claude-sonnet-4-6 (v1.1.0)
 */

package lessons

import (
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
	fieldLessonIDs           = "F1000"
	fieldSummaries           = "F1001"
	fieldDetails             = "F1002"
	fieldImportance          = "F1003"
	fieldLinkedFiles         = "F1004"
	fieldCommands            = "F1005"
	fieldTopics              = "F1006"
	fieldTags                = "F1007"
	fieldKeywords            = "F1008"
	fieldParentKeywords      = "F1009"
	fieldReviewChecks        = "F1010"
	fieldAIModelIDs          = "F1011"
	fieldLatestLessonAt      = "F1012"
	fieldLatestLessonSummary = "F1013"
	fieldLessonCount         = "F1014"
	fieldPurpose             = "F1015"
)

type projection struct {
	FilePath       string
	LessonIDs      []string
	Summaries      []string
	Details        []string
	Importance     []string
	LinkedFiles    []string
	Commands       []string
	Topics         []string
	Tags           []string
	Keywords       []string
	ParentKeywords []string
	ReviewChecks   []string
	AIModelIDs     []string
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

func RebuildManifestFromRecords(records []Record) (string, error) {
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

func BuildManifest(records []Record) manifest.ManifestFile {
	now := time.Now().UTC()
	repoName := repoName()
	projections := map[string]*projection{}

	for _, record := range records {
		targets := append([]string{}, record.AppliesTo.Files...)
		if len(targets) == 0 {
			targets = syntheticTargets(record)
		}
		for _, target := range targets {
			p := projections[target]
			if p == nil {
				p = &projection{FilePath: target}
				projections[target] = p
			}
			p.LessonIDs = append(p.LessonIDs, record.ID)
			p.Summaries = append(p.Summaries, record.Summary)
			p.Details = append(p.Details, record.Details)
			p.Importance = append(p.Importance, record.Importance)
			p.LinkedFiles = append(p.LinkedFiles, record.AppliesTo.LinkedFiles...)
			p.Commands = append(p.Commands, record.AppliesTo.Commands...)
			// Use new Topic field, fallback to legacy if empty
			if record.Topic != "" {
				p.Topics = append(p.Topics, record.Topic)
				p.Topics = append(p.Topics, record.RelatedTopics...)
			} else {
				p.Topics = append(p.Topics, record.AppliesTo.Topics...)
			}
			p.Tags = append(p.Tags, record.Tags...)
			p.Keywords = append(p.Keywords, record.Keywords...)
			p.ParentKeywords = append(p.ParentKeywords, record.ParentKeywords...)
			p.ReviewChecks = append(p.ReviewChecks, record.ReviewChecks...)
			p.AIModelIDs = append(p.AIModelIDs, record.AI.ModelID)
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
			RepoRef:   "R1000",
			BranchRef: "B1000",
			FilePath:  p.FilePath,
			Language:  "",
			ChatID:    100000 + i + 1,
			Fields: map[string]interface{}{
				fieldLessonIDs:           normalizeStringList(p.LessonIDs),
				fieldSummaries:           normalizeStringList(p.Summaries),
				fieldDetails:             normalizeStringList(p.Details),
				fieldImportance:          normalizeStringList(p.Importance),
				fieldLinkedFiles:         normalizeStringList(p.LinkedFiles),
				fieldCommands:            normalizeStringList(p.Commands),
				fieldTopics:              normalizeStringList(p.Topics),
				fieldTags:                normalizeStringList(p.Tags),
				fieldKeywords:            normalizeStringList(p.Keywords),
				fieldParentKeywords:      normalizeStringList(p.ParentKeywords),
				fieldReviewChecks:        normalizeStringList(p.ReviewChecks),
				fieldAIModelIDs:          normalizeStringList(p.AIModelIDs),
				fieldLatestLessonAt:      p.Latest.Format(time.RFC3339),
				fieldLatestLessonSummary: p.LatestSummary,
				fieldLessonCount:         p.Count,
				fieldPurpose:             "",
			},
		})
	}

	return manifest.ManifestFile{
		SchemaVersion: "1.0.0",
		GeneratedAt:   now,
		Manifest: manifest.ManifestInfo{
			ManifestName: "GitSense Lessons",
			DatabaseName: DatabaseName,
			Description:  "GitSense-managed lessons worth preserving for future humans and agents",
			Tags:         []string{"gsc-lessons", "repository-knowledge", "agent-memory"},
		},
		Repositories: []manifest.Repository{{Ref: "R1000", Name: repoName}},
		Branches:     []manifest.Branch{{Ref: "B1000", Name: branchName()}},
		Analyzers: []manifest.Analyzer{{
			Ref:         "A1000",
			ID:          "gsc-lessons",
			Name:        "GitSense Lessons",
			Description: "Projects confirmed repository lessons into file-level metadata",
			Version:     "1.0.0",
		}},
		Fields: lessonFields(),
		Data:   data,
	}
}

func lessonFields() []manifest.Field {
	fields := []struct{ ref, name, display, typ, desc string }{
		{fieldLessonIDs, "lesson_ids", "Lesson IDs", "array", "Stable lesson IDs included in this projection"},
		{fieldSummaries, "lesson_summaries", "Lesson Summaries", "array", "Concise lesson summaries"},
		{fieldDetails, "lesson_details", "Lesson Details", "array", "Detailed lesson context"},
		{fieldImportance, "importance_levels", "Importance Levels", "array", "Importance values for projected lessons"},
		{fieldLinkedFiles, "linked_files", "Linked Files", "array", "Files related by business or workflow logic"},
		{fieldCommands, "commands", "Commands", "array", "Commands referenced by lessons"},
		{fieldTopics, "topics", "Topics", "array", "Topic anchors"},
		{fieldTags, "tags", "Tags", "array", "User or agent-provided tags"},
		{fieldKeywords, "keywords", "Keywords", "array", "Queryable lesson keywords"},
		{fieldParentKeywords, "parent_keywords", "Parent Keywords", "array", "Broader lesson domains"},
		{fieldReviewChecks, "review_checks", "Review Checks", "array", "Checks to apply during review"},
		{fieldAIModelIDs, "ai_model_ids", "AI Model IDs", "array", "AI models that generated lessons"},
		{fieldLatestLessonAt, "latest_lesson_at", "Latest Lesson At", "string", "Timestamp of the most recent lesson for this file"},
		{fieldLatestLessonSummary, "latest_lesson_summary", "Latest Lesson Summary", "string", "Most recent lesson summary as a single string — safe for gsc rg overlays"},
		{fieldLessonCount, "lesson_count", "Lesson Count", "integer", "Total number of lessons projected to this file — use to decide whether to dig deeper"},
		{fieldPurpose, "purpose", "Purpose", "string", "Reserved for GitSense Chat — a synthesized description of this file's role based on all lessons"},
	}
	out := make([]manifest.Field, 0, len(fields))
	for _, field := range fields {
		out = append(out, manifest.Field{
			Ref:         field.ref,
			AnalyzerRef: "A1000",
			Name:        field.name,
			DisplayName: field.display,
			Type:        field.typ,
			Description: field.desc,
		})
	}
	return out
}

func syntheticTargets(record Record) []string {
	// Use new Topic field, fallback to legacy
	if record.Topic != "" {
		return []string{".gitsense/lessons/topics/" + record.Topic}
	}
	if len(record.AppliesTo.Topics) > 0 {
		return []string{".gitsense/lessons/topics/" + record.AppliesTo.Topics[0]}
	}
	if len(record.AppliesTo.Commands) > 0 {
		return []string{".gitsense/lessons/commands/" + slugify(record.AppliesTo.Commands[0])}
	}
	if len(record.Tags) > 0 {
		return []string{".gitsense/lessons/topics/" + record.Tags[0]}
	}
	return []string{".gitsense/lessons/topics/general"}
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
