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
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var requiredSystemPromptCapabilities = []string{
	"GSC-Experts-Capability: compact-on-demand-tool-gates-v1",
	"GSC-Experts-Capability: advisory-rules-default-v1",
	"GSC-Experts-Capability: agent-rule-creator-checklist-v2",
}

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
		// Resolve path and count entries inline using internal/db patterns
		dbPath, _ := db.ResolveManifestDBPath(entry.DatabaseName)
		count := 0
		if database, err := db.OpenDB(dbPath); err == nil {
			database.QueryRowContext(ctx, "SELECT COUNT(*) FROM files").Scan(&count)
			db.CloseDB(database)
		}

		brains = append(brains, BrainSummary{
			Name:        entry.DatabaseName,
			DisplayName: entry.ManifestName,
			Description: entry.Description,
			Version:     entry.Version,
			EntryCount:  count,
		})
	}

	return brains, nil
}

// Render generates the expert context markdown as a string.
func Render(ctx context.Context, expertsCtx ExpertsContext) (string, error) {
	templateContent, err := loadSystemPromptTemplate()
	if err != nil {
		return "", err
	}

	var brainListBuilder strings.Builder
	primaryBrain := ""
	for _, brain := range expertsCtx.Brains {
		if primaryBrain == "" {
			primaryBrain = brain.Name
		}
		brainListBuilder.WriteString(fmt.Sprintf("- **%s** (db: `%s`, v%s): %s\n", brain.DisplayName, brain.Name, brain.Version, brain.Description))
	}

	repoName := "personal"
	if expertsCtx.RepoPath != "" {
		repoName = filepath.Base(expertsCtx.RepoPath)
	}

	templateData := struct {
		RepoName         string
		UserLevel        string
		DynamicBrainList string
		HasBrains        bool
		PrimaryBrain     string
		HasRules         bool
		RulesMode        string
		InRepo           bool
	}{
		RepoName:         repoName,
		UserLevel:        expertsCtx.UserLevel,
		DynamicBrainList: brainListBuilder.String(),
		HasBrains:        len(expertsCtx.Brains) > 0,
		PrimaryBrain:     primaryBrain,
		HasRules:         expertsCtx.HasRules,
		RulesMode:        expertsCtx.RulesMode,
		InRepo:           expertsCtx.RepoPath != "",
	}

	tmpl, err := template.New("system_prompt").Parse(string(templateContent))
	if err != nil {
		return "", err
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, templateData); err != nil {
		return "", err
	}

	return StripProvenanceHeaders(rendered.String()), nil
}

// Generate writes the expert context markdown file (legacy, use Render + write instead).
func Generate(ctx context.Context, expertsCtx ExpertsContext, outputPath string) error {
	output, err := Render(ctx, expertsCtx)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	return os.WriteFile(outputPath, []byte(output), 0644)
}

// loadSystemPromptTemplate loads the system prompt template using Local First, Embedded Fallback.
func loadSystemPromptTemplate() ([]byte, error) {
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
	if !useEmbedded {
		templateContent, err = os.ReadFile(filepath.Join(templateBase, "GSC_EXPERTS_SYSTEM_PROMPT.md"))
		if err != nil || !hasRequiredSystemPromptCapabilities(string(templateContent)) {
			useEmbedded = true
			templateBase = "templates/experts"
		}
	}
	if useEmbedded {
		templateContent, err = settings.TemplateFS.ReadFile(templateBase + "/GSC_EXPERTS_SYSTEM_PROMPT.md")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read system prompt template: %w", err)
	}
	return templateContent, nil
}

func hasRequiredSystemPromptCapabilities(content string) bool {
	for _, capability := range requiredSystemPromptCapabilities {
		if !strings.Contains(content, capability) {
			return false
		}
	}
	return true
}

// StripProvenanceHeaders removes HTML-comment provenance blocks (<!-- ... -->)
// from rendered expert content so the AI-facing output does not waste context
// tokens on Block-UUIDs, versions, and author lists. Shared by Generate (init)
// and the `gsc experts guide` command.
func StripProvenanceHeaders(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		if strings.HasPrefix(line, "<!--") {
			skip = true
			continue
		}
		if strings.HasPrefix(line, "-->") {
			skip = false
			continue
		}
		if !skip {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

// OrientationMessage returns the handshake message.
func OrientationMessage(ctx ExpertsContext, outputPath string, wroteToFile bool) string {
	var sb strings.Builder
	sb.WriteString("✅ GitSense Expert Context Initialized.\n\n")

	if wroteToFile {
		displayPath := outputPath
		if ctx.RepoPath != "" {
			if relPath, err := filepath.Rel(ctx.RepoPath, outputPath); err == nil {
				displayPath = relPath
			}
		}
		sb.WriteString("REQUIRED BEFORE CONTINUING:\n")
		sb.WriteString(fmt.Sprintf("1. Run: cat %s\n", displayPath))
		sb.WriteString("2. Read the context file to understand available tools and rules.\n")
		sb.WriteString("3. When you need to use a tool, load the relevant guide automatically.\n\n")
		sb.WriteString("Do not ask the user which guide to load — load them as needed.\n\n")
	} else {
		sb.WriteString("Context printed to stdout above.\n")
		sb.WriteString("When you need to use a tool, load the relevant guide automatically.\n\n")
		sb.WriteString("Do not ask the user which guide to load — load them as needed.\n\n")
	}

	if ctx.RepoPath == "" {
		sb.WriteString("Scope: personal only (not in a git repository).\n")
		sb.WriteString("  - Repo scope is unavailable until you change into a git repo.\n")
		sb.WriteString("  - Use --scope personal for reads, --target personal for writes.\n\n")
	} else {
		sb.WriteString("Scope: repo + personal (default --scope all).\n")
		sb.WriteString("  - Reads default to --scope all (repo + personal).\n")
		sb.WriteString("  - Writes require --target repo or --target personal.\n\n")
	}

	sb.WriteString("Active Brains:\n")
	if len(ctx.Brains) == 0 {
		sb.WriteString("  - None. The briefing covers text/search basics; import a manifest to enable metadata queries.\n")
	} else {
		for _, b := range ctx.Brains {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", b.DisplayName, b.Description))
		}
	}
	if ctx.HasRules {
		sb.WriteString(fmt.Sprintf("\nRules Mode: %s\n", ctx.RulesMode))
	}
	if wroteToFile {
		sb.WriteString(fmt.Sprintf("\nExpert instructions written to: %s\n", outputPath))
	}
	return sb.String()
}

// ForgetMessage returns the reset message.
func ForgetMessage() string {
	return "🧹 SYSTEM RESET\n\n" +
		"Your expert instructions have been cleared.\n" +
		"Please disregard all rules from '.gitsense/experts-context.md' and revert to standard coding assistant behavior."
}
