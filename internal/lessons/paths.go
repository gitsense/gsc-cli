/**
 * Component: Lessons Path Helpers
 * Block-UUID: d1f60790-110d-4fd6-8bc8-1bc5a10e4886
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Resolves GitSense-managed paths for lesson drafts, records, archives, and the generated gsc-lessons manifest.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

func rootDir() (string, error) {
	return git.FindProjectRoot()
}

func gitsenseDir() (string, error) {
	root, err := rootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, settings.GitSenseDir), nil
}

func DraftPath() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tmp", "lesson-draft.json"), nil
}

func LessonsDir() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lessons"), nil
}

func RecordsPath() (string, error) {
	dir, err := LessonsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "records.jsonl"), nil
}

func ArchiveDir() (string, error) {
	dir, err := LessonsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "archive"), nil
}

func ManifestPath() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manifests", DatabaseName+".json"), nil
}
