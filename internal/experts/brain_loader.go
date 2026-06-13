/**
 * Component: Experts Brain Loader
 * Block-UUID: 09448aad-cd37-4dc9-a542-3f70a31a12dd
 * Parent-UUID: 02b749fa-776c-413f-84d2-bfc9d736eed3
 * Version: 1.5.0
 * Description: Updated DynamicBrainList rendering to include db: <name> inline so agents see the authoritative --db identifier alongside the manifest display name, eliminating the manifest-name-as-db ambiguity.
 * Language: Go
 * Created-at: 2026-05-02T00:50:45.329Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.1.1), Gemini 3 Flash (v1.1.2), Gemini 3 Flash (v1.1.3), GLM-4.7 (v1.1.4), GLM-4.7 (v1.1.5), GLM-4.7 (v1.1.6), GLM-4.7 (v1.1.7), GLM-4.7 (v1.1.8), GLM-4.7 (v1.1.9), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), claude-sonnet-4-6 (v1.5.0)
 */

package experts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// LoadBrains retrieves the schema and summary information for the specified brains.
func LoadBrains(ctx context.Context, cfg ExpertsConfig) ([]BrainSummary, error) {
	reg, err := registry.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	var targetDBs []registry.RegistryEntry
	if len(cfg.Databases) > 0 {
		for _, dbName := range cfg.Databases {
			if entry, found := reg.FindEntryByDBName(dbName); found {
				targetDBs = append(targetDBs, *entry)
			}
		}
	} else {
		targetDBs = reg.Databases
	}

	if len(targetDBs) == 0 {
		return []BrainSummary{}, nil
	}

	var brains []BrainSummary
	for _, entry := range targetDBs {
		schema, err := manifest.GetSchema(ctx, entry.DatabaseName)
		if err != nil {
			continue
		}

		// Resolve path and count entries inline using internal/db patterns
		dbPath, _ := db.ResolveManifestDBPath(entry.DatabaseName)
		count := 0
		if database, err := db.OpenDB(dbPath); err == nil {
			database.QueryRowContext(ctx, "SELECT COUNT(*) FROM files").Scan(&count)
			db.CloseDB(database)
		}

		var fields []FieldSummary
		for _, analyzer := range schema.Analyzers {
			for _, field := range analyzer.Fields {
				fields = append(fields, FieldSummary{
					Name:        field.Name,
					Type:        field.Type,
					Description: field.Description,
				})
			}
		}

		brains = append(brains, BrainSummary{
			Name:        entry.DatabaseName,
			DisplayName: entry.ManifestName,
			Description: entry.Description,
			Version:     entry.Version,
			EntryCount:  count,
			Fields:      fields,
		})
	}

	return brains, nil
}

// Generate writes the expert context markdown file.
func Generate(ctx context.Context, expertsCtx ExpertsContext, outputPath string) error {
	// Resolve template path with "Local First, Embedded Fallback" strategy
	// 1. Try $GSC_HOME/cli/templates/experts (allows live updates from GitSense Chat App)
	// 2. Fallback to embedded filesystem (ensures binary works standalone)
	gscHome, err := settings.GetGSCHome(false)
	var templateBase string
	var useEmbedded bool

	if err == nil {
		templateBase = filepath.Join(gscHome, "cli", "templates", "experts")
	} else {
		templateBase = "templates/experts"
		useEmbedded = true
	}

	var templateContent []byte
	if useEmbedded {
		templateContent, err = settings.TemplateFS.ReadFile(templateBase + "/GSC_EXPERTS_SYSTEM_PROMPT.md")
	} else {
		templateContent, err = os.ReadFile(filepath.Join(templateBase, "GSC_EXPERTS_SYSTEM_PROMPT.md"))
	}
	if err != nil {
		return fmt.Errorf("failed to read system prompt template: %w", err)
	}

	// Build Rich Vocabulary
	var brainListBuilder strings.Builder
	var vocabBuilder strings.Builder
	primaryBrain := ""
	for _, brain := range expertsCtx.Brains {
		if primaryBrain == "" {
			primaryBrain = brain.Name
		}
		brainListBuilder.WriteString(fmt.Sprintf("- **%s** (db: `%s`, v%s): %s\n", brain.DisplayName, brain.Name, brain.Version, brain.Description))

		vocabBuilder.WriteString(fmt.Sprintf("### %s\n", brain.DisplayName))
		for _, f := range brain.Fields {
			vocabBuilder.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", f.Name, f.Type, f.Description))
		}
		vocabBuilder.WriteString("\n")
	}

	templateData := struct {
		RepoName          string
		UserLevel         string
		DynamicBrainList  string
		DynamicVocabulary string
		HasBrains         bool
		PrimaryBrain      string
	}{
		RepoName:          filepath.Base(expertsCtx.RepoPath),
		UserLevel:         expertsCtx.UserLevel,
		DynamicBrainList:  brainListBuilder.String(),
		DynamicVocabulary: vocabBuilder.String(),
		HasBrains:         len(expertsCtx.Brains) > 0,
		PrimaryBrain:      primaryBrain,
	}

	tmpl, err := template.New("system_prompt").Parse(string(templateContent))
	if err != nil {
		return err
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, templateData); err != nil {
		return err
	}

	// Append Guides
	guides := []string{"GSC_OVERVIEW.md", "GSC_QUERY_GUIDE.md", "GSC_VISUALIZATION_GUIDE.md", "GSC_BRAIN_MANAGEMENT_GUIDE.md"}
	for _, guide := range guides {
		var content []byte
		if useEmbedded {
			content, err = settings.TemplateFS.ReadFile(templateBase + "/" + guide)
		} else {
			content, err = os.ReadFile(filepath.Join(templateBase, guide))
		}

		if err == nil {
			rendered.WriteString("\n\n---\n\n")
			rendered.Write(content)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	return os.WriteFile(outputPath, []byte(rendered.String()), 0644)
}

// OrientationMessage returns the handshake message.
func OrientationMessage(ctx ExpertsContext, contextFilePath string) string {
	// Ensure path is relative to repo root for easy AI 'cat'
	relPath, _ := filepath.Rel(ctx.RepoPath, contextFilePath)

	var sb strings.Builder
	sb.WriteString("✅ GitSense Expert Context Initialized.\n\n")

	sb.WriteString("⚠️  MANDATORY SETUP:\n")
	sb.WriteString("Before responding to ANY of the following, you MUST run:\n")
	sb.WriteString(fmt.Sprintf("  cat %s\n\n", relPath))

	sb.WriteString("Triggers:\n")
	sb.WriteString("- Any question about the codebase, files, or architecture\n")
	sb.WriteString("- Any request to find, search, or locate code\n")
	sb.WriteString("- Any question about available brains or commands\n")
	sb.WriteString("- Any 'gsc ...' command you are about to execute\n")
	sb.WriteString("- Any time you are unsure which gsc command to use\n\n")

	sb.WriteString("FORBIDDEN (do not attempt):\n")
	sb.WriteString("- gsc brain ...          (not a valid command)\n")
	sb.WriteString("- gsc experts list       (not a valid command)\n")
	sb.WriteString("- go run . experts init  (use the 'gsc' binary, not go run)\n")
	sb.WriteString("- Guessing command names  (always read the context file first)\n\n")

	sb.WriteString("Active Brains:\n")
	if len(ctx.Brains) == 0 {
		sb.WriteString("  - None. Use gsc for command guidance and text search; import a manifest to enable Brain-backed metadata queries.\n")
	} else {
		for _, b := range ctx.Brains {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", b.DisplayName, b.Description))
		}
	}
	sb.WriteString(fmt.Sprintf("\nExpert instructions written to: %s\n\n", relPath))
	sb.WriteString("ACTION REQUIRED NOW:\n")
	sb.WriteString(fmt.Sprintf("cat %s\n", relPath))
	return sb.String()
}

// ForgetMessage returns the reset message.
func ForgetMessage() string {
	return "🧹 SYSTEM RESET\n\n" +
		"Your expert instructions have been cleared.\n" +
		"Please disregard all rules from '.gitsense/experts-context.md' and revert to standard coding assistant behavior."
}
