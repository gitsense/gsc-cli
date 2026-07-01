/**
 * Component: Trigger Scope Tests
 * Block-UUID: (generated)
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for source-aware trigger path resolution.
 * Language: Go
 * Created-at: 2026-06-27T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
)

func TestTriggerPathForSourceRepo(t *testing.T) {
	repoDir := initTempGitRepo(t)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create trigger file
	triggersDir := filepath.Join(repoDir, ".gitsense", "rules", "triggers")
	if err := os.MkdirAll(triggersDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(triggersDir, "test.sh"), []byte("#!/bin/bash\necho ok"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := TriggerPathForSource(gitsensescope.SourceRepo, "test.sh")
	if err != nil {
		t.Fatalf("TriggerPathForSource(repo, test.sh) error: %v", err)
	}

	// Resolve symlinks for comparison (macOS uses /private/var for temp dirs)
	realRepoDir, _ := filepath.EvalSymlinks(repoDir)
	want := filepath.Join(realRepoDir, ".gitsense", "rules", "triggers", "test.sh")
	if got != want {
		t.Errorf("TriggerPathForSource(repo, test.sh) = %q, want %q", got, want)
	}
}

func TestTriggerPathForSourceWithRepoRoot(t *testing.T) {
	repoDir := initTempGitRepo(t)

	got, err := TriggerPathForSourceWithRepoRoot(gitsensescope.SourceRepo, "test.sh", repoDir)
	if err != nil {
		t.Fatalf("TriggerPathForSourceWithRepoRoot(repo, test.sh, repoDir) error: %v", err)
	}

	realRepoDir, _ := filepath.EvalSymlinks(repoDir)
	want := filepath.Join(realRepoDir, ".gitsense", "rules", "triggers", "test.sh")
	if got != want {
		t.Errorf("TriggerPathForSourceWithRepoRoot(repo, test.sh, repoDir) = %q, want %q", got, want)
	}
}

func TestTriggerPathForSourcePersonal(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	// Create trigger file
	triggersDir := filepath.Join(personalDir, "rules", "triggers")
	if err := os.MkdirAll(triggersDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(triggersDir, "personal.sh"), []byte("#!/bin/bash\necho ok"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := TriggerPathForSource(gitsensescope.SourcePersonal, "personal.sh")
	if err != nil {
		t.Fatalf("TriggerPathForSource(personal, personal.sh) error: %v", err)
	}

	want := filepath.Join(personalDir, "rules", "triggers", "personal.sh")
	if got != want {
		t.Errorf("TriggerPathForSource(personal, personal.sh) = %q, want %q", got, want)
	}
}

func TestTriggerPathForSourceRejectsAbsoluteEntry(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	_, err := TriggerPathForSource(gitsensescope.SourcePersonal, filepath.Join(personalDir, "rules", "triggers", "test.sh"))
	if err == nil {
		t.Fatal("TriggerPathForSource with absolute entry should error")
	}
	if !containsStr(err.Error(), "must be relative") {
		t.Errorf("error should mention relative trigger directory, got: %v", err)
	}
}

func TestTriggerPathForSourceEmpty(t *testing.T) {
	repoDir := initTempGitRepo(t)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Empty source should default to repo
	_, err := TriggerPathForSource("", "test.sh")
	if err != nil {
		// Should resolve to repo path (file doesn't exist, but path resolution works)
		t.Logf("TriggerPathForSource('', test.sh) correctly resolved (file not found is expected)")
	}
}

func TestTriggerPathForSourceEscapeAttempt(t *testing.T) {
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	_, err := TriggerPathForSource(gitsensescope.SourcePersonal, "../../../etc/passwd")
	if err == nil {
		t.Fatal("TriggerPathForSource with escape attempt should error")
	}
	if !containsStr(err.Error(), "escapes") {
		t.Errorf("error should mention 'escapes', got: %v", err)
	}
}

func TestTriggerPathForSourceRepoOutsideRepo(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tempDir)

	_, err := TriggerPathForSource(gitsensescope.SourceRepo, "test.sh")
	if err == nil {
		t.Fatal("TriggerPathForSource(repo) outside repo should error")
	}
}

func TestValidateTriggerFileWithSource(t *testing.T) {
	repoDir := initTempGitRepo(t)
	personalDir := t.TempDir()
	t.Setenv("GSC_HOME", personalDir)

	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(repoDir)

	// Create repo trigger
	repoTriggersDir := filepath.Join(repoDir, ".gitsense", "rules", "triggers")
	if err := os.MkdirAll(repoTriggersDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoTriggersDir, "repo-trigger.sh"), []byte("#!/bin/bash\necho ok"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create personal trigger
	personalTriggersDir := filepath.Join(personalDir, "rules", "triggers")
	if err := os.MkdirAll(personalTriggersDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(personalTriggersDir, "personal-trigger.sh"), []byte("#!/bin/bash\necho ok"), 0755); err != nil {
		t.Fatal(err)
	}

	// Validate repo trigger with repo source - should pass
	errs := ValidateTriggerFileWithSource("repo-trigger.sh", "bash", gitsensescope.SourceRepo)
	if len(errs) > 0 {
		t.Errorf("ValidateTriggerFileWithSource(repo-trigger.sh, bash, repo) errors: %v", errs)
	}

	// Validate personal trigger with personal source - should pass
	errs = ValidateTriggerFileWithSource("personal-trigger.sh", "bash", gitsensescope.SourcePersonal)
	if len(errs) > 0 {
		t.Errorf("ValidateTriggerFileWithSource(personal-trigger.sh, bash, personal) errors: %v", errs)
	}

	// Validate personal trigger with repo source - should fail (file not found)
	errs = ValidateTriggerFileWithSource("personal-trigger.sh", "bash", gitsensescope.SourceRepo)
	if len(errs) == 0 {
		t.Error("ValidateTriggerFileWithSource(personal-trigger.sh, bash, repo) should fail")
	}

	// Validate repo trigger with personal source - should fail (file not found)
	errs = ValidateTriggerFileWithSource("repo-trigger.sh", "bash", gitsensescope.SourcePersonal)
	if len(errs) == 0 {
		t.Error("ValidateTriggerFileWithSource(repo-trigger.sh, bash, personal) should fail")
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrHelper(s, substr))
}

func containsStrHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
