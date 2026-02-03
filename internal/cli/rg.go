/**
 * Component: Ripgrep Command
 * Block-UUID: 13db18c2-83fe-40ef-8421-908ef3129174
 * Parent-UUID: 75329ea5-b5a0-435e-97e7-5c798d72526c
 * Version: 2.1.0
 * Description: CLI command definition for 'gsc rg', executing ripgrep searches and enriching results with manifest metadata. Updated to use effective configuration (profiles), support quiet mode, and display prominent workspace headers in TTY.
 * Language: Go
 * Created-at: 2026-02-03T02:48:26.532Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v2.0.0), GLM-4.7 (v2.0.1), GLM-4.7 (v2.1.0)
 */


package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/output"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	rgDB           string
	rgFormat       string
	rgContext      int
	rgCaseSensitive bool
	rgFileType     string
	rgQuiet        bool
)

// rgCmd represents the rg command
var rgCmd = &cobra.Command{
	Use:   "rg <pattern>",
	Short: "Search code with metadata enrichment",
	Long: `Search for patterns in code using ripgrep and enrich results with metadata
from a manifest database. This allows you to see search results alongside
contextual information like risk levels, topics, or business impact.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		pattern := args[0]

		// 1. Load Effective Config (Merges active profile)
		config, err := manifest.GetEffectiveConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// 2. Resolve Database Name (flag > profile default > global default)
		dbName := rgDB
		if dbName == "" {
			dbName = config.Global.DefaultDatabase
		}

		// 3. Validate Database
		if dbName == "" {
			return fmt.Errorf("database is required. Use --db flag or set a profile with 'gsc config use <name>'")
		}

		// 4. Resolve Format (flag > profile default)
		format := rgFormat
		if format == "" {
			format = config.RG.DefaultFormat
		}

		// 5. Resolve Context (flag > profile default)
		contextLines := rgContext
		if contextLines == 0 {
			contextLines = config.RG.DefaultContext
		}

		// 6. Construct Options
		options := manifest.RgOptions{
			Pattern:       pattern,
			Database:      dbName,
			ContextLines:  contextLines,
			CaseSensitive: rgCaseSensitive,
			FileType:      rgFileType,
		}

		// 7. Execute Ripgrep
		logger.Info("Searching for pattern", "pattern", pattern, "database", dbName)
		matches, err := manifest.ExecuteRipgrep(options)
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			fmt.Println("No matches found.")
			return nil
		}

		// 8. Enrich Matches
		enriched, err := manifest.EnrichMatches(ctx, matches, dbName)
		if err != nil {
			return err
		}

		// 9. Format Output
		switch strings.ToLower(format) {
		case "json":
			formatRgJSON(enriched)
		case "table":
			// Resolve profile name for context headers
			profileName := config.ActiveProfile
			if profileName == "" {
				profileName = "default"
			}
			// Pass config to formatter for workspace headers
			formatRgTable(enriched, rgQuiet, profileName, config)
		default:
			return fmt.Errorf("unsupported format: %s", format)
		}

		return nil
	},
}

func init() {
	// Add flags
	rgCmd.Flags().StringVarP(&rgDB, "db", "d", "", "Database name for enrichment (inherits from profile)")
	rgCmd.Flags().StringVarP(&rgFormat, "format", "f", "table", "Output format (json, table)")
	rgCmd.Flags().IntVarP(&rgContext, "context", "C", 0, "Show N lines of context around matches")
	rgCmd.Flags().BoolVar(&rgCaseSensitive, "case-sensitive", false, "Case-sensitive search")
	rgCmd.Flags().StringVarP(&rgFileType, "type", "t", "", "Filter by file type (e.g., js, py)")
	rgCmd.Flags().BoolVar(&rgQuiet, "quiet", false, "Suppress headers, footers, and hints (clean output)")
}

// formatRgJSON formats enriched matches as JSON.
func formatRgJSON(matches []manifest.EnrichedMatch) {
	bytes, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		logger.Error("Failed to format JSON: %v", err)
		return
	}
	fmt.Println(string(bytes))
}

// formatRgTable formats enriched matches as a text table.
// It attempts to display common metadata fields if available.
// Updated to accept config for workspace header generation.
func formatRgTable(matches []manifest.EnrichedMatch, quiet bool, profileName string, config *manifest.QueryConfig) {
	if len(matches) == 0 {
		return
	}

	// Determine which metadata fields are common across all matches
	// to decide which columns to show
	commonFields := getCommonMetadataFields(matches)

	// Build headers
	headers := []string{"File Path", "Line", "Match"}
	headers = append(headers, commonFields...)

	// Build rows
	var rows [][]string
	for _, m := range matches {
		row := []string{
			m.FilePath,
			fmt.Sprintf("%d", m.LineNumber),
			truncateString(m.Match, 50), // Truncate long matches
		}

		// Add metadata values
		for _, field := range commonFields {
			if val, ok := m.Metadata[field]; ok {
				row = append(row, fmt.Sprintf("%v", val))
			} else {
				row = append(row, "")
			}
		}

		rows = append(rows, row)
	}

	table := output.FormatTable(headers, rows)
	
	if quiet {
		fmt.Println(table)
		return
	}

	// Check if we are in a terminal
	if output.IsTerminal() {
		// Prepend the prominent header
		header := manifest.FormatWorkspaceHeader(config)
		fmt.Printf("%s%s\n[Context: %s] | Switch: gsc config use <name>", header, table, profileName)
		return
	}

	// Fallback to simple header if piping
	fmt.Printf("[Context: %s]\n%s\n[Context: %s] | Switch: gsc config use <name>", profileName, table, profileName)
}

// getCommonMetadataFields finds metadata fields that appear in at least 50% of matches.
func getCommonMetadataFields(matches []manifest.EnrichedMatch) []string {
	if len(matches) == 0 {
		return []string{}
	}

	fieldCounts := make(map[string]int)
	for _, m := range matches {
		for field := range m.Metadata {
			fieldCounts[field]++
		}
	}

	threshold := len(matches) / 2
	var common []string
	for field, count := range fieldCounts {
		if count >= threshold {
			common = append(common, field)
		}
	}

	// Prioritize useful fields
	priorityFields := []string{"risk_level", "parent_topics", "topics", "purpose"}
	var ordered []string
	for _, pf := range priorityFields {
		for _, cf := range common {
			if cf == pf {
				ordered = append(ordered, cf)
				break
			}
		}
	}

	// Add remaining common fields
	for _, cf := range common {
		found := false
		for _, of := range ordered {
			if cf == of {
				found = true
				break
			}
		}
		if !found {
			ordered = append(ordered, cf)
		}
	}

	return ordered
}

// truncateString shortens a string to a maximum length.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// RegisterRgCommand registers the rg command with the root command.
func RegisterRgCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(rgCmd)
}
