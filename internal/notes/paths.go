/**
 * Component: Notes Path Helpers
 * Block-UUID: d4e5f6a7-b8c9-0123-defa-123456789012
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Resolves GitSense-managed paths for note records.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package notes

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

func NotesDir() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "notes"), nil
}

func RecordsPath() (string, error) {
	dir, err := NotesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "records.jsonl"), nil
}

func ManifestPath() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manifests", DatabaseName+".json"), nil
}
