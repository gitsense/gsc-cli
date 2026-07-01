/**
 * Component: Rules Path Helpers
 * Block-UUID: b2c3d4e5-f6a7-8901-bcde-f12345678901
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Resolves GitSense-managed paths for rule records, drafts, triggers, and fixtures.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package rules

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

func RulesDir() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "rules"), nil
}

func RecordsPath() (string, error) {
	dir, err := RulesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "records.jsonl"), nil
}

func ArchiveDir() (string, error) {
	dir, err := RulesDir()
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

// TriggersDir returns the path to .gitsense/rules/triggers/
func TriggersDir() (string, error) {
	dir, err := RulesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "triggers"), nil
}

// FixturesDir returns the path to .gitsense/rules/fixtures/
func FixturesDir() (string, error) {
	dir, err := RulesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "fixtures"), nil
}

// TriggerPath returns the absolute path to a trigger file given its entry path.
func TriggerPath(entry string) (string, error) {
	triggersDir, err := TriggersDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(triggersDir, entry), nil
}
