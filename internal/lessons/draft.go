/**
 * Component: Lessons Draft Lifecycle
 * Block-UUID: eda3a755-b9dc-4041-b1e5-1c0385ec207a
 * Parent-UUID: 2b65054a-08d8-4f19-b871-8fbf3160f58a
 * Version: 1.2.0
 * Description: Removed ArchiveDraft, DiscardDraft, and the archive parameter from CreateDraft. Discard command eliminated; drafts are now replaced or deleted directly.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/gitignore"
	"github.com/gitsense/gsc-cli/internal/manifest"
)

func EnsureWorkspace() error {
	if err := manifest.InitializeGitSense(); err != nil {
		return err
	}
	dir, err := gitsenseDir()
	if err != nil {
		return err
	}
	return gitignore.EnsureUpdated(dir, gitignore.Registration{
		Source: gitignore.SourceLessons,
		Patterns: []string{
			"tmp/lesson-draft.json",
			"lessons/archive/",
		},
	})
}

func CreateDraft(replace bool) (string, error) {
	if err := EnsureWorkspace(); err != nil {
		return "", err
	}
	path, err := DraftPath()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(path); err == nil {
		if replace {
			if err := os.Remove(path); err != nil {
				return "", fmt.Errorf("failed to replace existing draft: %w", err)
			}
		} else {
			return "", fmt.Errorf("lesson draft already exists at %s", path)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}

	draft := Draft{
		Summary:    "",
		Details:    "",
		Importance: "medium",
		AppliesTo: AppliesTo{
			Files:       []string{},
			LinkedFiles: []string{},
			Commands:    []string{},
			Topics:      []string{},
		},
		Tags:         []string{},
		ReviewChecks: []string{},
		AI: AIProvenance{
			Provider: "unknown",
			ModelID:  "unknown",
			Agent:    "unknown",
		},
	}
	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return "", err
	}
	return path, nil
}

