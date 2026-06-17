/**
 * Component: Lessons Draft Lifecycle
 * Block-UUID: eda3a755-b9dc-4041-b1e5-1c0385ec207a
 * Parent-UUID: 2b65054a-08d8-4f19-b871-8fbf3160f58a
 * Version: 1.4.0
 * Description: Registered the lesson-update.json staging file in gitignore and added WriteDraft for the one-shot add command.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0), claude-opus-4-8 (v1.3.0), claude-opus-4-8 (v1.4.0)
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
			"tmp/lesson-update.json",
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

// WriteDraft stages a fully-populated draft to the draft file (used by the
// one-shot add command). It refuses to overwrite an existing draft unless
// replace is set, mirroring CreateDraft.
func WriteDraft(draft Draft, replace bool) (string, error) {
	if err := EnsureWorkspace(); err != nil {
		return "", err
	}
	path, err := DraftPath()
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr == nil && !replace {
		return "", fmt.Errorf("lesson draft already exists at %s; pass --replace or run 'gsc lessons draft discard'", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
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

