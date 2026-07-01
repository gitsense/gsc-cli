/**
 * Component: GitSense Path Resolution
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Resolves GitSense storage paths for repo and personal scopes, including records, manifests, archives, triggers, and fixtures.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package gitsensescope

import (
	"fmt"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// SourcedDir pairs a storage source with its resolved path.
type SourcedDir struct {
	Source Source
	Path   string
}

// RepoGitSenseSubdir returns the name of the .gitsense directory within a repo.
// This is settings.GitSenseDir at call time.
func RepoGitSenseSubdir() string {
	return settings.GitSenseDir
}

// RepoGitSenseDir returns the absolute path to the repo-level .gitsense directory.
// Uses settings.GitSenseDir at call time. Hard error if not in a git repo.
func RepoGitSenseDir() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return RepoGitSenseDirForRoot(root), nil
}

// RepoGitSenseDirForRoot returns the repo-level GitSense directory for an
// already-discovered repository root.
func RepoGitSenseDirForRoot(repoRoot string) string {
	if realRoot, err := filepath.EvalSymlinks(repoRoot); err == nil {
		repoRoot = realRoot
	}
	return filepath.Join(repoRoot, settings.GitSenseDir)
}

// PersonalGitSenseDir returns the absolute path to the personal GSC_HOME directory.
// Falls back to ~/.gitsense if GSC_HOME is not set.
func PersonalGitSenseDir() (string, error) {
	return settings.GetGSCHome(false)
}

// GitSenseDirs returns the resolved directories for the given scope.
// - ScopeRepo: repo only (error if not in repo)
// - ScopePersonal: personal only
// - ScopeAll: repo first, then personal; if not in repo, personal only (no error)
func GitSenseDirs(scope Scope) ([]SourcedDir, error) {
	switch scope {
	case ScopeRepo:
		repoPath, err := RepoGitSenseDir()
		if err != nil {
			return nil, err
		}
		return []SourcedDir{{Source: SourceRepo, Path: repoPath}}, nil

	case ScopePersonal:
		personalPath, err := PersonalGitSenseDir()
		if err != nil {
			return nil, err
		}
		return []SourcedDir{{Source: SourcePersonal, Path: personalPath}}, nil

	case ScopeAll:
		var dirs []SourcedDir
		repoPath, err := RepoGitSenseDir()
		if err == nil {
			dirs = append(dirs, SourcedDir{Source: SourceRepo, Path: repoPath})
		}
		personalPath, err := PersonalGitSenseDir()
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, SourcedDir{Source: SourcePersonal, Path: personalPath})
		return dirs, nil

	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
}

// GitSenseDirForTarget returns the resolved directory for a write target.
// - TargetRepo: repo (error if not in repo)
// - TargetPersonal: personal
func GitSenseDirForTarget(target Target) (SourcedDir, error) {
	switch target {
	case TargetRepo:
		repoPath, err := RepoGitSenseDir()
		if err != nil {
			return SourcedDir{}, err
		}
		return SourcedDir{Source: SourceRepo, Path: repoPath}, nil

	case TargetPersonal:
		personalPath, err := PersonalGitSenseDir()
		if err != nil {
			return SourcedDir{}, err
		}
		return SourcedDir{Source: SourcePersonal, Path: personalPath}, nil

	default:
		return SourcedDir{}, fmt.Errorf("invalid target: %s", target)
	}
}

// RecordsPath returns the path to the JSONL records file for a given knowledge kind.
// Example: <base.Path>/notes/records.jsonl
func RecordsPath(base SourcedDir, kind Kind) string {
	return filepath.Join(base.Path, string(kind), "records.jsonl")
}

// ManifestPath returns the path to a manifest JSON file.
// Example: <base.Path>/manifests/<dbName>.json
func ManifestPath(base SourcedDir, dbName string) string {
	return filepath.Join(base.Path, "manifests", dbName+".json")
}

// ArchiveDir returns the path to the archive directory for a given knowledge kind.
// Example: <base.Path>/notes/archive
func ArchiveDir(base SourcedDir, kind Kind) string {
	return filepath.Join(base.Path, string(kind), "archive")
}

// RulesTriggersDir returns the path to the rules triggers directory.
// Example: <base.Path>/rules/triggers
func RulesTriggersDir(base SourcedDir) string {
	return filepath.Join(base.Path, "rules", "triggers")
}

// RulesFixturesDir returns the path to the rules fixtures directory.
// Example: <base.Path>/rules/fixtures
func RulesFixturesDir(base SourcedDir) string {
	return filepath.Join(base.Path, "rules", "fixtures")
}
