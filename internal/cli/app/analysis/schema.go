/**
 * Component: Analysis Schema Command
 * Block-UUID: 8f7feef0-c7e2-42b7-acb6-16146b9f3dce
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc app analysis schema' command for displaying analyzer field definitions. Reads the analyzer prompt file and parses the "Custom Metadata Definitions" section to extract field names, types, and descriptions.
 * Language: Go
 * Created-at: 2026-06-16T15:00:00.000Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package analysis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	flagSchemaFormat string
	flagSchemaQuiet  bool
)

// SchemaCmd represents the 'gsc app analysis schema' command.
var SchemaCmd = &cobra.Command{
	Use:   "schema <analyzer>",
	Short: "Display analyzer field definitions",
	Long: `Reads the analyzer prompt file and displays the metadata field definitions (schema).
This shows what fields the analyzer extracts, their types, and descriptions.

The schema is parsed from the "Custom Metadata Definitions" section of the analyzer prompt.

Examples:
  # Show schema for typescript-event-coupling analyzer
  gsc app analysis schema typescript-event-coupling

  # Show schema in JSON format
  gsc app analysis schema typescript-event-coupling --format json

  # List available analyzers
  gsc app analysis schema --list`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSchema,
}

func init() {
	SchemaCmd.Flags().StringVar(&flagSchemaFormat, "format", "table", "Output format: table, json")
	SchemaCmd.Flags().BoolVar(&flagSchemaQuiet, "quiet", false, "Suppress headers and hints")
	SchemaCmd.Flags().Bool("list", false, "List all available analyzers")
}

// FieldDefinition represents a single metadata field definition.
type FieldDefinition struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// AnalyzerSchema represents the complete schema for an analyzer.
type AnalyzerSchema struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Fields      []FieldDefinition `json:"fields"`
}

// runSchema is the main entry point for the schema command.
func runSchema(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Check if --list flag is set
	listAnalyzers, _ := cmd.Flags().GetBool("list")
	if listAnalyzers {
		return listAvailableAnalyzers()
	}

	// Require analyzer name if not listing
	if len(args) == 0 {
		return fmt.Errorf("analyzer name is required. Use --list to see available analyzers")
	}

	analyzerName := args[0]

	// Resolve analyzer file path
	analyzerPath, err := resolveAnalyzerPath(analyzerName)
	if err != nil {
		return err
	}

	// Parse schema from file
	schema, err := parseAnalyzerSchema(analyzerPath, analyzerName)
	if err != nil {
		return fmt.Errorf("failed to parse analyzer schema: %w", err)
	}

	// Output results
	return outputSchema(schema)
}

// resolveAnalyzerPath constructs the path to the analyzer prompt file.
func resolveAnalyzerPath(analyzerName string) (string, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// Path pattern: $GSC_HOME/data/analyzers/<name>/file-content/default/1.md
	analyzerPath := filepath.Join(gscHome, "data", "analyzers", analyzerName, "file-content", "default", "1.md")

	// Verify file exists
	if _, err := os.Stat(analyzerPath); os.IsNotExist(err) {
		return "", fmt.Errorf("analyzer '%s' not found at %s", analyzerName, analyzerPath)
	}

	return analyzerPath, nil
}

// parseAnalyzerSchema reads the analyzer file and extracts field definitions.
func parseAnalyzerSchema(filePath string, analyzerName string) (*AnalyzerSchema, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open analyzer file: %w", err)
	}
	defer file.Close()

	schema := &AnalyzerSchema{
		Name: analyzerName,
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	inMetadataSection := false
	inCodeBlock := false
	var currentSection strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Track code blocks
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		// Look for "Custom Metadata Definitions" section
		if strings.Contains(line, "Custom Metadata Definitions") {
			inMetadataSection = true
			currentSection.Reset()
			continue
		}

		// If we're in the metadata section, collect lines
		if inMetadataSection {
			// Stop at next major section (starts with # or ##)
			if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") {
				// Parse what we've collected so far
				if currentSection.Len() > 0 {
					parseMetadataSection(currentSection.String(), schema)
				}
				inMetadataSection = false
				continue
			}

			// Also stop at JSON generation rules section (indicates end of definitions)
			if strings.Contains(line, "JSON Generation and Validation Rules") {
				if currentSection.Len() > 0 {
					parseMetadataSection(currentSection.String(), schema)
				}
				inMetadataSection = false
				continue
			}

			currentSection.WriteString(line)
			currentSection.WriteString("\n")
		}
	}

	// Parse any remaining content
	if inMetadataSection && currentSection.Len() > 0 {
		parseMetadataSection(currentSection.String(), schema)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading analyzer file: %w", err)
	}

	return schema, nil
}

// parseMetadataSection parses the collected metadata definitions text.
func parseMetadataSection(section string, schema *AnalyzerSchema) {
	lines := strings.Split(section, "\n")

	// Regex to match field definitions like:
	// *   `language` (string): One of `typescript`, `tsx`, `javascript`, `jsx`, or `other`.
	// *   `emits_events` (array of strings): Statically visible event type names...
	// *   `confidence` (string): One of `high`, `medium`, or `low`.
	fieldRegex := regexp.MustCompile(`^\*\s+\x60([^\x60]+)\x60\s+\(([^)]+)\):\s*(.+)$`)

	// Also try alternative format without backticks
	altFieldRegex := regexp.MustCompile(`^\*\s+(\w+)\s+\(([^)]+)\):\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try primary regex
		matches := fieldRegex.FindStringSubmatch(line)
		if matches == nil {
			// Try alternative regex
			matches = altFieldRegex.FindStringSubmatch(line)
		}

		if matches != nil && len(matches) >= 4 {
			fieldName := strings.TrimSpace(matches[1])
			fieldType := strings.TrimSpace(matches[2])
			fieldDesc := strings.TrimSpace(matches[3])

			// Clean up description (remove trailing period)
			fieldDesc = strings.TrimSuffix(fieldDesc, ".")

			schema.Fields = append(schema.Fields, FieldDefinition{
				Name:        fieldName,
				Type:        fieldType,
				Description: fieldDesc,
			})
		}
	}
}

// listAvailableAnalyzers lists all analyzers in $GSC_HOME/data/analyzers.
func listAvailableAnalyzers() error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	analyzersDir := filepath.Join(gscHome, "data", "analyzers")

	entries, err := os.ReadDir(analyzersDir)
	if err != nil {
		return fmt.Errorf("failed to read analyzers directory: %w", err)
	}

	var analyzers []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), "_") {
			// Check if the analyzer has a prompt file
			promptPath := filepath.Join(analyzersDir, entry.Name(), "file-content", "default", "1.md")
			if _, err := os.Stat(promptPath); err == nil {
				analyzers = append(analyzers, entry.Name())
			}
		}
	}

	if len(analyzers) == 0 {
		fmt.Println("No analyzers found.")
		return nil
	}

	fmt.Println("Available analyzers:")
	for _, a := range analyzers {
		fmt.Printf("  - %s\n", a)
	}

	return nil
}

// outputSchema outputs the schema in the requested format.
func outputSchema(schema *AnalyzerSchema) error {
	switch flagSchemaFormat {
	case "json":
		return outputSchemaJSON(schema)
	default:
		return outputSchemaTable(schema)
	}
}

// outputSchemaJSON outputs the schema as JSON.
func outputSchemaJSON(schema *AnalyzerSchema) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(schema)
}

// outputSchemaTable outputs the schema as a human-readable table.
func outputSchemaTable(schema *AnalyzerSchema) error {
	if !flagSchemaQuiet {
		fmt.Printf("Analyzer: %s\n", schema.Name)
		fmt.Println()
	}

	if len(schema.Fields) == 0 {
		fmt.Println("No field definitions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if !flagSchemaQuiet {
		fmt.Fprintln(w, "FIELD\tTYPE\tDESCRIPTION")
		fmt.Fprintln(w, "-----\t----\t-----------")
	}

	// Data rows
	for _, field := range schema.Fields {
		// Truncate description if too long
		desc := field.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", field.Name, field.Type, desc)
	}

	return w.Flush()
}
