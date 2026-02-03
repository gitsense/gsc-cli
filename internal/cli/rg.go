/**
 * Component: Ripgrep Command
 * Block-UUID: ca03e93d-3714-43aa-bce9-7a40baeb4e4a
 * Parent-UUID: f529e88f-d338-4dc2-9a44-689c2014f92f
 * Version: 3.0.1
 * Description: CLI command definition for 'gsc rg'. Refactored to dual-pass workflow: Pass 1 (JSON) for file discovery, Pass 2 (Raw) for display. Appends YAML/JSON metadata section. Removed old table formatting logic.
 * Language: Go
 * Created-at: 2026-02-03T07:55:23.506Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v2.0.0), GLM-4.7 (v2.0.1), GLM-4.7 (v2.1.0), GLM-4.7 (v3.0.0), GLM-4.7 (v3.0.1)
 */


package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
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
contextual information like risk levels, topics, or business impact.

The output consists of two parts:
1. Standard ripgrep output (preserving colors and formatting)
2. A metadata appendix (YAML or JSON) showing details for matched files`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
		// This now controls the metadata appendix format (yaml/json)
		format := rgFormat
		if format == "" {
			format = "yaml" // Default to YAML for the appendix
		}

		// 5. Resolve Context (flag > profile default)
		contextLines := rgContext
		if contextLines == 0 {
			contextLines = config.RG.DefaultContext
		}

		// 6. Pass 1: Discovery (JSON Mode)
		// We run ripgrep in JSON mode to reliably extract file paths
		discoveryOptions := manifest.RgOptions{
			Pattern:       pattern,
			Database:      dbName,
			ContextLines:  contextLines,
			CaseSensitive: rgCaseSensitive,
			FileType:      rgFileType,
		}

		matches, err := manifest.ExecuteRipgrep(discoveryOptions)
		if err != nil {
			return err
		}

		// Extract unique file paths from matches
		uniqueFiles := getUniqueFilePaths(matches)

		// 7. Pass 2: Display (Raw Mode)
		// We run ripgrep again with color output to preserve formatting
		rawOutput, err := manifest.ExecuteRipgrepRaw(pattern, contextLines, rgCaseSensitive, rgFileType)
		if err != nil {
			return err
		}

		// Print raw ripgrep output directly
		fmt.Print(rawOutput)

		// 8. Enrichment & Appendix
		// Only append metadata if not in quiet mode
		if !rgQuiet {
			// Fetch metadata for the unique files
			metadataMap, err := manifest.GetMetadataForFiles(cmd.Context(), uniqueFiles, dbName)
			if err != nil {
				logger.Warning("Failed to retrieve metadata: %v", err)
				// Continue without metadata if lookup fails
			} else {
				// Print the appendix
				printMetadataAppendix(metadataMap, format)
			}
		}

		return nil
	},
}

func init() {
	// Add flags
	rgCmd.Flags().StringVarP(&rgDB, "db", "d", "", "Database name for enrichment (inherits from profile)")
	rgCmd.Flags().StringVarP(&rgFormat, "format", "f", "yaml", "Metadata appendix format (yaml, json)")
	rgCmd.Flags().IntVarP(&rgContext, "context", "C", 0, "Show N lines of context around matches")
	rgCmd.Flags().BoolVar(&rgCaseSensitive, "case-sensitive", false, "Case-sensitive search")
	rgCmd.Flags().StringVarP(&rgFileType, "type", "t", "", "Filter by file type (e.g., js, py)")
	rgCmd.Flags().BoolVar(&rgQuiet, "quiet", false, "Suppress metadata appendix (pure ripgrep output)")
}

// getUniqueFilePaths extracts a sorted list of unique file paths from matches.
func getUniqueFilePaths(matches []manifest.RgMatch) []string {
	seen := make(map[string]bool)
	var files []string

	for _, match := range matches {
		if !seen[match.FilePath] {
			seen[match.FilePath] = true
			files = append(files, match.FilePath)
		}
	}

	// Sort for consistent ordering
	sort.Strings(files)
	return files
}

// printMetadataAppendix prints the metadata section at the end of the output.
func printMetadataAppendix(metadataMap map[string]manifest.FileMetadataResult, format string) {
	// Print separator
	fmt.Println("────────────────────────────────────────")
	fmt.Println("GitSense Chat Metadata")
	fmt.Println("────────────────────────────────────────")
	fmt.Println()

	// Format and print metadata
	switch format {
	case "json":
		fmt.Print(manifest.FormatMetadataJSON(metadataMap))
	case "yaml":
		fmt.Print(manifest.FormatMetadataYAML(metadataMap))
	default:
		// Fallback to YAML if format is unknown
		fmt.Print(manifest.FormatMetadataYAML(metadataMap))
	}
}

// RegisterRgCommand registers the rg command with the root command.
func RegisterRgCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(rgCmd)
}
