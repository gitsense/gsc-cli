/**
 * Component: Experts Brain Loader Tests
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-ef1234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for Experts brain loader including orientation message generation, template fallback behavior, and provenance header stripping.
 * Language: Go
 * Created-at: 2026-06-20T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */

package experts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrientationMessageRequiresContextBeforeContinuing(t *testing.T) {
	repoPath := filepath.Join("tmp", "repo")
	contextPath := filepath.Join(repoPath, ".gitsense", ContextFileName)

	message := OrientationMessage(ExpertsContext{RepoPath: repoPath}, contextPath, true)

	checks := []string{
		"REQUIRED BEFORE CONTINUING:",
		"1. Run: cat .gitsense" + string(filepath.Separator) + ContextFileName,
		"Read the context file to understand available tools and rules.",
		"When you need to use a tool, load the relevant guide automatically.",
	}
	for _, check := range checks {
		if !strings.Contains(message, check) {
			t.Errorf("OrientationMessage() missing %q\nmessage:\n%s", check, message)
		}
	}
}

func TestGenerateFallsBackWhenLocalTemplateLacksToolGates(t *testing.T) {
	gscHome := t.TempDir()
	t.Setenv("GSC_HOME", gscHome)
	localTemplateDir := filepath.Join(gscHome, "cli", "templates", "experts")
	if err := os.MkdirAll(localTemplateDir, 0755); err != nil {
		t.Fatal(err)
	}
	staleTemplate := "# Legacy briefing for {{.RepoName}}\n"
	if err := os.WriteFile(filepath.Join(localTemplateDir, "GSC_EXPERTS_SYSTEM_PROMPT.md"), []byte(staleTemplate), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(t.TempDir(), ContextFileName)
	if err := Generate(context.Background(), ExpertsContext{RepoPath: "/tmp/repo"}, outputPath); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "# Tool Gates") {
		t.Fatalf("Generate() used stale local template; output:\n%s", output)
	}
}

func TestGenerateFallsBackWhenLocalTemplateLacksAdvisoryRulesDefault(t *testing.T) {
	gscHome := t.TempDir()
	t.Setenv("GSC_HOME", gscHome)
	localTemplateDir := filepath.Join(gscHome, "cli", "templates", "experts")
	if err := os.MkdirAll(localTemplateDir, 0755); err != nil {
		t.Fatal(err)
	}
	staleTemplate := "<!--\nGSC-Experts-Capability: compact-on-demand-tool-gates-v1\n-->\n# Legacy briefing for {{.RepoName}}\n"
	if err := os.WriteFile(filepath.Join(localTemplateDir, "GSC_EXPERTS_SYSTEM_PROMPT.md"), []byte(staleTemplate), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(t.TempDir(), ContextFileName)
	ctx := ExpertsContext{RepoPath: "/tmp/repo", HasRules: true, RulesMode: "advisory"}
	if err := Generate(context.Background(), ctx, outputPath); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	if strings.Contains(text, "Legacy briefing") {
		t.Fatalf("Generate() used stale local template; output:\n%s", text)
	}
	if !strings.Contains(text, "Do not ask the user before consulting rules") {
		t.Fatalf("Generate() missing advisory-by-default guidance:\n%s", text)
	}
	if !strings.Contains(text, "--creator agent") {
		t.Fatalf("Generate() missing agent creator checklist guidance:\n%s", text)
	}
}

func TestGenerateOmitsRenderedFieldVocabulary(t *testing.T) {
	t.Setenv("GSC_HOME", filepath.Join(t.TempDir(), "missing"))
	outputPath := filepath.Join(t.TempDir(), ContextFileName)
	ctx := ExpertsContext{
		RepoPath: "/tmp/repo",
		Brains: []BrainSummary{
			{
				Name:        "code-intent",
				DisplayName: "Code Intent",
				Description: "Code intent metadata",
				Version:     "1.0",
				Fields: []FieldSummary{
					{Name: "purpose", Type: "string", Description: "Large field description"},
				},
			},
		},
	}

	if err := Generate(context.Background(), ctx, outputPath); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, unexpected := range []string{
		"Large field description",
		"{{.DynamicVocabulary}}",
		"### Code Intent",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("Generate() rendered field vocabulary %q in output:\n%s", unexpected, text)
		}
	}
	for _, expected := range []string{
		"gsc brains --json",
		"Code Intent",
		"# Tool Gates",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("Generate() missing %q in output:\n%s", expected, text)
		}
	}
}

func TestStripProvenanceHeaders(t *testing.T) {
	input := "<!--\nVersion: 1.0.0\n-->\n\n# Guide\n"
	want := "\n# Guide\n"

	if got := StripProvenanceHeaders(input); got != want {
		t.Fatalf("StripProvenanceHeaders() = %q, want %q", got, want)
	}
}
