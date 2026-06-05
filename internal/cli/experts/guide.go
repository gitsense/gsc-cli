/**
 * Component: Experts Guide Command
 * Block-UUID: e91e0ee8-67de-4089-b898-99186edd98da
 * Parent-UUID: 5034a927-3965-400a-9d7a-bdbde3290971
 * Version: 1.2.2
 * Description: Implements the 'gsc experts guide' command. Loads the static GSC_GUIDE.md template, fetches active brains using manifest.ListDatabases(), and injects brain data (or "no brains" message) into the template before printing to stdout. This provides the Main Chat AI with context to act as a strategic consultant before triggering Inline Agents.
 * Language: Go
 * Created-at: 2026-05-25T15:54:42.297Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1), GLM-4.7 (v1.2.2)
 */


package experts

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

// NewGuideCmd creates and returns the 'gsc experts guide' command.
func NewGuideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "guide",
		Short: "Print the gsc consultation guide for AI assistants",
		Long: `Prints a structured consultation guide describing the gsc toolset paradigm,
command taxonomy, intent workflow model, and token-efficiency rules.

Designed to be pasted into a chat session to give an AI assistant
full context about what gsc is capable of - enabling it to act as a
strategic consultant before triggering Inline Agents.

Unlike 'gsc experts init', this command does not require a git
repository or active Brains.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuide()
		},
	}
}

func runGuide() error {
	// 1. Load the static GSC_GUIDE.md template
	templateContent, err := loadGuideTemplate()
	if err != nil {
		return fmt.Errorf("failed to load guide template: %w", err)
	}

	// 2. Get active brains using existing logic
	ctx := context.Background()
	databases, err := manifest.ListDatabases(ctx)

	// 3. Prepare active intelligence section
	var activeIntelligence string
	if err != nil || len(databases) == 0 {
		// No brains available
		activeIntelligence = getNoBrainsMessage()
	} else {
		// Brains available - format as JSON for AI parsing
		activeIntelligence = formatBrainsAsJSON(ctx, databases)
	}

	// 4. Inject into template and print
	output, err := renderGuideTemplate(templateContent, activeIntelligence)
	if err != nil {
		return fmt.Errorf("failed to render guide template: %w", err)
	}

	// Strip code block headers from the output
	// The consultation guide is meant for AI consumption, not version control
	output = stripCodeBlockHeaders(output)

	fmt.Print(output)
	return nil
}

// loadGuideTemplate loads the GSC_GUIDE.md template with "Local First, Embedded Fallback" strategy
func loadGuideTemplate() ([]byte, error) {
	// Try local $GSC_HOME first
	gscHome, err := settings.GetGSCHome(false)
	if err == nil {
		localPath := filepath.Join(gscHome, "cli", "templates", "experts", "GSC_GUIDE.md")
		content, err := os.ReadFile(localPath)
		if err == nil {
			return content, nil
		}
	}

	// Fallback to embedded filesystem
	return settings.TemplateFS.ReadFile("templates/experts/GSC_GUIDE.md")
}

// getNoBrainsMessage returns the message to display when no brains are active
func getNoBrainsMessage() string {
	return `⚠️ **No Brains are currently active in this repository.**

Without Brains, searches will rely on:
- Text patterns (grep)
- File paths and directory structure
- Standard file system operations

To enable intelligent querying, the user should run:\n`+
"```gsc manifest import <manifest-uri>```"
}

// formatBrainsAsJSON formats the database list with full field details as JSON for AI parsing
func formatBrainsAsJSON(ctx context.Context, databases []manifest.DatabaseInfo) string {
	// Create a simplified structure for AI consumption
	type FieldSummary struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}

	type BrainSummary struct {
		Name        string         `json:"name"`
		DisplayName string         `json:"display_name"`
		Description string         `json:"description"`
		EntryCount  int            `json:"entry_count"`
		Fields      []FieldSummary `json:"fields"`
	}

	var brains []BrainSummary
	for _, db := range databases {
		// Fetch the full schema for this brain to get field details
		var fields []FieldSummary
		schema, err := manifest.GetSchema(ctx, db.DatabaseName)
		if err != nil {
			logger.Warning("Failed to fetch schema for brain", "brain", db.DatabaseName, "error", err)
			// Continue with empty fields if schema fetch fails
		} else {
			// Extract field details from the schema
			for _, analyzer := range schema.Analyzers {
				for _, field := range analyzer.Fields {
					fields = append(fields, FieldSummary{
						Name:        field.Name,
						Type:        field.Type,
						Description: field.Description,
					})
				}
			}
		}

		brains = append(brains, BrainSummary{
			Name:        db.DatabaseName,
			DisplayName: db.ManifestName,
			Description: db.Description,
			EntryCount:  db.EntryCount,
			Fields:      fields,
		})
	}

	jsonBytes, err := json.MarshalIndent(brains, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting brains: %v", err)
	}

	return fmt.Sprintf("```json\n%s\n```", string(jsonBytes))
}

// renderGuideTemplate injects the active intelligence into the template
func renderGuideTemplate(templateContent []byte, activeIntelligence string) (string, error) {
	// Simple template replacement for the {{.ActiveIntelligence}} placeholder
	// We use strings.Replace for simplicity since we only have one placeholder
	output := strings.ReplaceAll(string(templateContent), "{{.ActiveIntelligence}}", activeIntelligence)
	return output, nil
}

// stripCodeBlockHeaders removes code block metadata headers from the guide output
// This reduces token usage and avoids confusion when the guide is pasted into a chat
func stripCodeBlockHeaders(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	skip := false

	for _, line := range lines {
		// Detect start of code block header (HTML comment style)
		if strings.HasPrefix(line, "<!--") {
			skip = true
			continue
		}
		// Detect end of code block header
		if strings.HasPrefix(line, "-->") {
			skip = false
			continue
		}
		// Keep lines that are not part of the header
		if !skip {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
