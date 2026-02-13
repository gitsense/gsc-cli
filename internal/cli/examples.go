/**
 * Component: CLI Examples Registry
 * Block-UUID: de6e0582-2023-4b14-ad40-da52ae272064
 * Parent-UUID: 5f82a390-3c7f-4432-9082-dcac558984a5
 * Version: 1.1.0
 * Description: Central registry for gsc usage examples. Uses real-world patterns from the gsc-architect manifest to demonstrate discovery ergonomics.
 * Language: Go
 * Created-at: 2026-02-13T06:21:05.847Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Example represents a single command usage pattern
type Example struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Intent      string   `json:"intent"`
	Command     string   `json:"command"`
	Description string   `json:"description"`
	AITip       string   `json:"ai_tip"`
}

// GetExamples returns the curated list of real-world examples
func GetExamples() []Example {
	return []Example{
		{
			ID:          "discovery-fields",
			Category:    "Discovery",
			Intent:      "See what metadata is available to query",
			Command:     "gsc fields",
			Description: "Lists all fields across all databases. Use this to see if you can filter by 'risk', 'topic', or 'layer'.",
			AITip:       "Run this first to understand the schema before constructing complex filters.",
		},
		{
			ID:          "discovery-brains",
			Category:    "Discovery",
			Intent:      "List available brains or inspect their schemas",
			Command:     "gsc brains",
			Description: "Lists all loaded brains (databases) or shows the schema for a specific brain.",
			AITip:       "Use this to verify which analyzers are active before querying.",
		},
		{
			ID:          "discovery-insights",
			Category:    "Discovery",
			Intent:      "Analyze the distribution of technical topics",
			Command:     "gsc insights --db gsc --field topics",
			Description: "Shows how many files are tagged with 'bridge', 'sqlite', or 'ripgrep'.",
			AITip:       "Use insights to identify the largest components of a feature set.",
		},
		{
			ID:          "discovery-coverage",
			Category:    "Discovery",
			Intent:      "Identify 'blind spots' in the architectural analysis",
			Command:     "gsc coverage --db gsc",
			Description: "Shows which directories have been analyzed and which are missing from the manifest.",
			AITip:       "High coverage means the AI's answers will be more deterministic and reliable.",
		},
		{
			ID:          "search-semantic",
			Category:    "Search",
			Intent:      "Search for code within a specific architectural layer",
			Command:     "gsc grep \"handshake\" --filter \"layer=internal-logic\"",
			Description: "Finds 'handshake' logic but ignores matches in tests or CLI boilerplate.",
			AITip:       "Filtering by layer is the fastest way to reduce noise in large results.",
		},
		{
			ID:          "viz-heat-map",
			Category:    "Visualization",
			Intent:      "Create a heat map of the CLI layer in a mono-repo",
			Command:     "gsc tree --db gsc --filter \"layer=cli\" --no-compact",
			Description: "Shows the full tree but highlights CLI files. --no-compact ensures you see where the CLI logic sits relative to other files.",
			AITip:       "Use --no-compact to help the user visualize 'what is NOT matched' to provide context.",
		},
		{
			ID:          "ai-bridge",
			Category:    "AI Integration",
			Intent:      "Inject a purpose-mapped tree into GitSense Chat",
			Command:     "gsc tree --db gsc --fields purpose --format json --code 123456",
			Description: "Sends a structured map of the repository's intent directly to your chat session.",
			AITip:       "Always use --format json with --code to ensure the AI receives data, not just text.",
		},
	}
}

// RenderExamples handles the output for both humans and AI
func RenderExamples(format string) (string, error) {
	examples := GetExamples()

	if strings.ToLower(format) == "json" {
		data := map[string]interface{}{
			"version":  "1.0.0",
			"tool":     "gsc",
			"examples": examples,
		}
		bytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}

	// Human-Readable Output
	var sb strings.Builder
	sb.WriteString("GSC CAPABILITY EXAMPLES\n")
	sb.WriteString("=======================\n\n")

	currentCat := ""
	for _, ex := range examples {
		if ex.Category != currentCat {
			currentCat = ex.Category
			sb.WriteString(fmt.Sprintf("[%s]\n", strings.ToUpper(currentCat)))
		}
		sb.WriteString(fmt.Sprintf("  Intent:  %s\n", ex.Intent) )
		sb.WriteString(fmt.Sprintf("  Command: %s\n", ex.Command))
		sb.WriteString(fmt.Sprintf("  Note:    %s\n\n", ex.Description))
	}

	return sb.String(), nil
}
