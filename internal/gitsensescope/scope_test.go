/**
 * Component: GitSense Scope Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for scope/target parsing, path resolution, and sourced directory ordering.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package gitsensescope

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// --- ParseScope tests ---

func TestParseScopeEmpty(t *testing.T) {
	got, err := ParseScope("")
	if err != nil {
		t.Fatalf("ParseScope(\"\") error: %v", err)
	}
	if got != ScopeAll {
		t.Errorf("ParseScope(\"\") = %q, want %q", got, ScopeAll)
	}
}

func TestParseScopeValid(t *testing.T) {
	tests := []struct {
		input string
		want  Scope
	}{
		{"repo", ScopeRepo},
		{"personal", ScopePersonal},
		{"all", ScopeAll},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseScope(tt.input)
			if err != nil {
				t.Fatalf("ParseScope(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseScope(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseScopeInvalid(t *testing.T) {
	_, err := ParseScope("invalid")
	if err == nil {
		t.Fatal("ParseScope(\"invalid\") expected error, got nil")
	}
	if !contains(err.Error(), "repo") || !contains(err.Error(), "personal") || !contains(err.Error(), "all") {
		t.Errorf("ParseScope(\"invalid\") error should list valid values, got: %v", err)
	}
}

// --- ParseTarget tests ---

func TestParseTargetEmpty(t *testing.T) {
	_, err := ParseTarget("")
	if err == nil {
		t.Fatal("ParseTarget(\"\") expected error, got nil")
	}
	if !contains(err.Error(), "required") {
		t.Errorf("ParseTarget(\"\") error should mention required, got: %v", err)
	}
}

func TestParseTargetValid(t *testing.T) {
	tests := []struct {
		input string
		want  Target
	}{
		{"repo", TargetRepo},
		{"personal", TargetPersonal},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if err != nil {
				t.Fatalf("ParseTarget(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseTarget(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTargetInvalid(t *testing.T) {
	_, err := ParseTarget("invalid")
	if err == nil {
		t.Fatal("ParseTarget(\"invalid\") expected error, got nil")
	}
	if !contains(err.Error(), "repo") || !contains(err.Error(), "personal") {
		t.Errorf("ParseTarget(\"invalid\") error should list valid values, got: %v", err)
	}
}

// --- Path tests ---

func TestPersonalGitSenseDirUsesGSCHome(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GSC_HOME", tempDir)

	got, err := PersonalGitSenseDir()
	if err != nil {
		t.Fatalf("PersonalGitSenseDir() error: %v", err)
	}
	if got != tempDir {
		t.Errorf("PersonalGitSenseDir() = %q, want %q", got, tempDir)
	}
}

func TestPersonalGitSenseDirFallback(t *testing.T) {
	// Clear GSC_HOME to test fallback
	t.Setenv("GSC_HOME", "")

	got, err := PersonalGitSenseDir()
	if err != nil {
		t.Fatalf("PersonalGitSenseDir() error: %v", err)
	}

	homeDir, _ := os.UserHomeDir()
	want := filepath.Join(homeDir, ".gitsense")
	if got != want {
		t.Errorf("PersonalGitSenseDir() = %q, want %q (fallback to ~/.gitsense)", got, want)
	}
}

func TestRepoGitSenseDirOutsideRepo(t *testing.T) {
	// Use a temp dir that is not a git repo
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	_, err := RepoGitSenseDir()
	if err == nil {
		t.Fatal("RepoGitSenseDir() expected error outside repo, got nil")
	}
}

func TestGitSenseDirsAllOutsideRepo(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("GSC_HOME", tempDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	dirs, err := GitSenseDirs(ScopeAll)
	if err != nil {
		t.Fatalf("GitSenseDirs(ScopeAll) error: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("GitSenseDirs(ScopeAll) outside repo returned %d dirs, want 1", len(dirs))
	}
	if dirs[0].Source != SourcePersonal {
		t.Errorf("GitSenseDirs(ScopeAll) outside repo source = %q, want %q", dirs[0].Source, SourcePersonal)
	}
	if dirs[0].Path != tempDir {
		t.Errorf("GitSenseDirs(ScopeAll) outside repo path = %q, want %q", dirs[0].Path, tempDir)
	}
}

func TestGitSenseDirsRepoOutsideRepo(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	_, err := GitSenseDirs(ScopeRepo)
	if err == nil {
		t.Fatal("GitSenseDirs(ScopeRepo) outside repo expected error, got nil")
	}
}

func TestGitSenseDirsAllInRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)
	personalDir := t.TempDir()

	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	dirs, err := GitSenseDirs(ScopeAll)
	if err != nil {
		t.Fatalf("GitSenseDirs(ScopeAll) error: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("GitSenseDirs(ScopeAll) in repo returned %d dirs, want 2", len(dirs))
	}

	// Resolve symlinks for comparison (macOS uses /private/var for temp dirs)
	realRepoDir, _ := filepath.EvalSymlinks(repoDir)

	// First should be repo
	if dirs[0].Source != SourceRepo {
		t.Errorf("dirs[0].Source = %q, want %q", dirs[0].Source, SourceRepo)
	}
	wantRepoPath := filepath.Join(realRepoDir, ".gitsense")
	if dirs[0].Path != wantRepoPath {
		t.Errorf("dirs[0].Path = %q, want %q", dirs[0].Path, wantRepoPath)
	}

	// Second should be personal
	if dirs[1].Source != SourcePersonal {
		t.Errorf("dirs[1].Source = %q, want %q", dirs[1].Source, SourcePersonal)
	}
	if dirs[1].Path != personalDir {
		t.Errorf("dirs[1].Path = %q, want %q", dirs[1].Path, personalDir)
	}
}

func TestGitSenseDirForTarget(t *testing.T) {
	repoDir := initTempGitRepo(t)
	personalDir := t.TempDir()

	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// TargetRepo
	repo, err := GitSenseDirForTarget(TargetRepo)
	if err != nil {
		t.Fatalf("GitSenseDirForTarget(TargetRepo) error: %v", err)
	}
	if repo.Source != SourceRepo {
		t.Errorf("repo.Source = %q, want %q", repo.Source, SourceRepo)
	}

	// TargetPersonal
	personal, err := GitSenseDirForTarget(TargetPersonal)
	if err != nil {
		t.Fatalf("GitSenseDirForTarget(TargetPersonal) error: %v", err)
	}
	if personal.Source != SourcePersonal {
		t.Errorf("personal.Source = %q, want %q", personal.Source, SourcePersonal)
	}
	if personal.Path != personalDir {
		t.Errorf("personal.Path = %q, want %q", personal.Path, personalDir)
	}
}

// --- Knowledge path helpers ---

func TestRecordsPath(t *testing.T) {
	base := SourcedDir{Source: SourceRepo, Path: "/repo/.gitsense"}

	tests := []struct {
		kind Kind
		want string
	}{
		{KindNotes, "/repo/.gitsense/notes/records.jsonl"},
		{KindRules, "/repo/.gitsense/rules/records.jsonl"},
		{KindLessons, "/repo/.gitsense/lessons/records.jsonl"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			got := RecordsPath(base, tt.kind)
			if got != tt.want {
				t.Errorf("RecordsPath(base, %q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestManifestPath(t *testing.T) {
	base := SourcedDir{Source: SourcePersonal, Path: "/home/user/.gitsense"}
	got := ManifestPath(base, "gsc-notes")
	want := "/home/user/.gitsense/manifests/gsc-notes.json"
	if got != want {
		t.Errorf("ManifestPath(base, \"gsc-notes\") = %q, want %q", got, want)
	}
}

func TestArchiveDir(t *testing.T) {
	base := SourcedDir{Source: SourceRepo, Path: "/repo/.gitsense"}

	tests := []struct {
		kind Kind
		want string
	}{
		{KindNotes, "/repo/.gitsense/notes/archive"},
		{KindRules, "/repo/.gitsense/rules/archive"},
		{KindLessons, "/repo/.gitsense/lessons/archive"},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			got := ArchiveDir(base, tt.kind)
			if got != tt.want {
				t.Errorf("ArchiveDir(base, %q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

func TestRulesTriggersDir(t *testing.T) {
	tests := []struct {
		source Source
		path   string
		want   string
	}{
		{SourceRepo, "/repo/.gitsense", "/repo/.gitsense/rules/triggers"},
		{SourcePersonal, "/home/user/.gitsense", "/home/user/.gitsense/rules/triggers"},
	}
	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			base := SourcedDir{Source: tt.source, Path: tt.path}
			got := RulesTriggersDir(base)
			if got != tt.want {
				t.Errorf("RulesTriggersDir(base) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRulesFixturesDir(t *testing.T) {
	tests := []struct {
		source Source
		path   string
		want   string
	}{
		{SourceRepo, "/repo/.gitsense", "/repo/.gitsense/rules/fixtures"},
		{SourcePersonal, "/home/user/.gitsense", "/home/user/.gitsense/rules/fixtures"},
	}
	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			base := SourcedDir{Source: tt.source, Path: tt.path}
			got := RulesFixturesDir(base)
			if got != tt.want {
				t.Errorf("RulesFixturesDir(base) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepoGitSenseSubdir(t *testing.T) {
	got := RepoGitSenseSubdir()
	if got != ".gitsense" {
		t.Errorf("RepoGitSenseSubdir() = %q, want %q", got, ".gitsense")
	}
}

// --- Helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}
	return dir
}
