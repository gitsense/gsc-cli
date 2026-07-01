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

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

// guideTopic pairs an on-demand reference guide with the template that backs it.
type guideTopic struct {
	name        string
	description string
	file        string
	aliases     []string
}

// guideTopics lists the reference guides that `gsc experts guide <topic>` can
// serve on demand. These were previously concatenated into experts-context.md
// by `gsc experts init`; they are now pulled only when a task needs the depth.
var guideTopics = []guideTopic{
	{"overview", "What gsc and Brains are; capability groups", "GSC_OVERVIEW.md", nil},
	{"query", "Filtering, coverage, insights, and lessons queries", "GSC_QUERY_GUIDE.md", []string{"lessons", "filter", "insights", "coverage"}},
	{"visualization", "tree + rg search and ripgrep syntax", "GSC_VISUALIZATION_GUIDE.md", []string{"rg", "grep", "tree", "search"}},
	{"brain-mgmt", "Importing manifests and constructing Brains", "GSC_BRAIN_MANAGEMENT_GUIDE.md", []string{"import", "manifest"}},
	{"rules", "Creating, querying, and managing rules for agents", "GSC_RULES_GUIDE.md", nil},
	{"triggers", "Executable triggers: V1 contract, runtimes, testing", "GSC_TRIGGERS_GUIDE.md", nil},
	{"trigger-creation", "Guide for creating executable triggers step-by-step", "GSC_TRIGGER_CREATION_GUIDE.md", nil},
	{"rule-authoring", "Safe rule and trigger authoring with scope/target", "GSC_RULE_AUTHORING_GUIDE.md", []string{"safe-rules", "knowledge-authoring"}},
	{"notes", "Creating, querying, and managing notes for agents", "GSC_NOTES_GUIDE.md", nil},
	{"topics", "Topic registry and unified knowledge discovery", "GSC_TOPICS_GUIDE.md", []string{"knowledge"}},
}

// resolveGuideTopic maps a user-supplied topic (canonical name or alias) to its
// backing template file, case-insensitively.
func resolveGuideTopic(topic string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(topic))
	for _, gt := range guideTopics {
		if gt.name == t {
			return gt.file, true
		}
		for _, a := range gt.aliases {
			if a == t {
				return gt.file, true
			}
		}
	}
	return "", false
}

// NewGuideCmd creates and returns the 'gsc experts guide' command.
func NewGuideCmd() *cobra.Command {
	var list bool

	cmd := &cobra.Command{
		Use:   "guide [topic]",
		Short: "Print the gsc consultation guide, or a reference guide on demand",
		Long: `With no argument, prints a structured consultation guide describing the gsc
toolset paradigm, command taxonomy, intent workflow model, and token-efficiency
rules - designed to give an AI assistant full context about what gsc can do.

With a topic argument, prints a specific reference guide on demand
(e.g. 'gsc experts guide query'). This lets a Brain-aware agent pull depth
only when a task needs it, instead of loading every guide up front.
Run 'gsc experts guide --list' to see the available topics.

Unlike 'gsc experts init', this command does not require a git
repository or active Brains.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				printGuideTopics()
				return nil
			}
			if len(args) == 1 {
				return runTopicGuide(args[0])
			}
			return runGuide()
		},
	}

	cmd.Flags().BoolVar(&list, "list", false, "List the available reference guide topics")
	return cmd
}

// runTopicGuide prints a single reference guide with provenance headers stripped.
func runTopicGuide(topic string) error {
	file, ok := resolveGuideTopic(topic)
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown guide topic: %q\n\n", topic)
		printGuideTopics()
		return fmt.Errorf("unknown guide topic: %s", topic)
	}

	content, err := loadExpertTemplate(file)
	if err != nil {
		return fmt.Errorf("failed to load guide %q: %w", topic, err)
	}

	fmt.Print(experts.StripProvenanceHeaders(string(content)))
	return nil
}

// printGuideTopics lists the on-demand reference guides.
func printGuideTopics() {
	fmt.Println("Available guide topics (use: gsc experts guide <topic>):")
	for _, gt := range guideTopics {
		fmt.Printf("  %-14s %s\n", gt.name, gt.description)
		if len(gt.aliases) > 0 {
			fmt.Printf("  %-14s (aliases: %s)\n", "", strings.Join(gt.aliases, ", "))
		}
	}
}

func runGuide() error {
	// 1. Load the static GSC_GUIDE.md template
	templateContent, err := loadExpertTemplate("GSC_GUIDE.md")
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
	output = experts.StripProvenanceHeaders(output)

	fmt.Print(output)
	return nil
}

// loadExpertTemplate loads an experts template by filename using the
// "Local First, Embedded Fallback" strategy: prefer a live copy under
// $GSC_HOME/cli/templates/experts, otherwise fall back to the embedded copy.
func loadExpertTemplate(filename string) ([]byte, error) {
	// Try local $GSC_HOME first
	gscHome, err := settings.GetGSCHome(false)
	if err == nil {
		localPath := filepath.Join(gscHome, "cli", "templates", "experts", filename)
		content, err := os.ReadFile(localPath)
		if err == nil {
			return content, nil
		}
	}

	// Fallback to embedded filesystem
	return settings.TemplateFS.ReadFile("templates/experts/" + filename)
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
