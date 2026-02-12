/**
 * Component: Tree Command
 * Block-UUID: 493a3569-9291-4f0f-a212-7be085d37d51
 * Parent-UUID: 91d2049a-51dd-4d7b-8cf5-1b6ac8038300
 * Version: 1.3.0
 * Description: Added a guard clause to prevent empty tree output when no database or --no-compact flag is provided. This reinforces the "Intelligence Layer" identity by requiring explicit intent for raw structural views.
 * Language: Go
 * Created-at: 2026-02-12T02:02:39.994Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), Gemini 3 Flash (v1.3.0)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/bridge"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
	"github.com/yourusername/gsc-cli/internal/search"
	"github.com/yourusername/gsc-cli/internal/tree"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	treeDB        string
	treeFields    []string
	treeIndent    int
	treeTruncate  int
	treeFormat    string
	treePrune     bool
	treeFilters   []string
	treeFocus     []string
	treeNoCompact bool
)

// treeCmd represents the tree command
var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display a hierarchical view of tracked files with metadata",
	Long: `Display a hierarchical view of tracked files enriched with metadata from a 
manifest database. Unlike the standard 'tree' command, this respects .gitignore 
and focuses on the repository's intelligence map.

The command is context-aware and will start the tree from your current working 
directory. Use --fields to include specific metadata like 'purpose' or 'risk'.

Filtering & Focus:
  --filter "field=value"    Filter files by metadata (e.g., layer=api)
  --focus "path/**"         Restrict the tree to specific paths or globs
  --no-compact              Show filenames for non-matching files in the heat map`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// 0. Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				return err
			}
		}

		// 1. Resolve Database (Check if we have a signal source)
		dbName := treeDB
		if dbName == "" {
			config, _ := manifest.GetEffectiveConfig()
			dbName = config.Global.DefaultDatabase
		}

		// 2. Guard Clause: Prevent empty/confusing output
		// If no DB is provided and the user hasn't explicitly asked for a raw tree (--no-compact),
		// we provide guidance instead of an empty tree.
		if dbName == "" && !treeNoCompact && treeFormat == "human" {
			fmt.Println("No manifest database specified.")
			fmt.Println("\n'gsc tree' is designed to visualize your repository's intelligence layer.")
			fmt.Println("To proceed, choose one of the following:")
			fmt.Println("\n1. View the Intelligence Map (Recommended):")
			fmt.Println("   Specify a database to see purpose, risk, and other metadata.")
			fmt.Println("   $ gsc tree --db <name> --fields purpose")
			fmt.Println("\n2. View the Raw File Tree:")
			fmt.Println("   Show all tracked files without metadata enrichment.")
			fmt.Println("   $ gsc tree --no-compact")
			fmt.Println("\nRun 'gsc manifest list' to see available databases in this workspace.")
			
			// We return nil here because this is a helpful guidance state, not a binary failure.
			return nil
		}

		// Validate format
		treeFormat = strings.ToLower(treeFormat)
		if treeFormat != "human" && treeFormat != "json" && treeFormat != "ai-portable" {
			return fmt.Errorf("invalid format: %s. Supported formats: human, json, ai-portable", treeFormat)
		}

		// 3. Get Repository Context
		repoRoot, cwdOffset, err := git.GetRepoContext()
		if err != nil {
			return fmt.Errorf("failed to get repository context: %w", err)
		}

		// 4. Get Tracked Files
		files, err := git.GetTrackedFiles(cmd.Context(), repoRoot)
		if err != nil {
			return fmt.Errorf("failed to get tracked files: %w", err)
		}

		// 5. Build Initial Tree (with Structural Focus)
		rootNode := tree.BuildTree(files, cwdOffset, treeFocus)

		var filters []search.FilterCondition
		if dbName != "" {
			// Resolve DB Name
			dbName, err = registry.ResolveDatabase(dbName)
			if err != nil {
				return err
			}

			// 6. Parse Semantic Filters
			filters, err = search.ParseFilters(cmd.Context(), treeFilters, dbName)
			if err != nil {
				return fmt.Errorf("failed to parse filters: %w", err)
			}

			// Resolve DB Path and Open
			dbPath, err := manifest.ResolveDBPath(dbName)
			if err != nil {
				return err
			}

			// 7. Fetch Metadata
			metadataMap, _, err := search.FetchMetadataMap(cmd.Context(), dbPath, files, "all", nil, treeFields, filters)
			if err != nil {
				logger.Debug("Failed to fetch metadata for tree", "error", err)
			} else {
				// 8. Enrich Tree & Evaluate Filters
				tree.EnrichTree(rootNode, "", metadataMap, filters)
			}
		} else if len(treeFilters) > 0 {
			return fmt.Errorf("database (--db) is required when using --filter")
		}

		// 9. Calculate Visibility (Propagate match status up the tree)
		tree.CalculateVisibility(rootNode)

		// 10. Prune if requested
		if treePrune {
			tree.PruneTree(rootNode)
		}

		// 11. Calculate Stats
		stats := tree.CalculateStats(rootNode)

		// 12. Render Output
		var outputStr string
		if treeFormat == "json" {
			outputStr, err = tree.RenderJSON(rootNode, stats, dbName, treeFields, filters, treeFocus, treePrune, cwdOffset)
			if err != nil {
				return fmt.Errorf("failed to render JSON: %w", err)
			}
		} else if treeFormat == "ai-portable" {
			outputStr, err = tree.RenderPortableJSON(rootNode, stats, treeFields, treePrune, cwdOffset)
			if err != nil {
				return fmt.Errorf("failed to render Portable JSON: %w", err)
			}
		} else {
			outputStr = tree.RenderHuman(rootNode, treeIndent, treeTruncate, treeFields, treeNoCompact)
			
			// Append Summary Report
			outputStr += fmt.Sprintf("\nTree Coverage Summary:\n")
			outputStr += fmt.Sprintf("  Total Tracked Files: %d\n", stats.TotalFiles)
			outputStr += fmt.Sprintf("  Analyzed:            %d (%.1f%%)\n", stats.AnalyzedFiles, stats.Coverage)
			outputStr += fmt.Sprintf("  Matched:             %d\n", stats.MatchedFiles)
			outputStr += fmt.Sprintf("\nNote: This tree only includes files tracked by Git.\n")

			if dbName == "" && len(treeFields) == 0 {
				outputStr += "Hint: To include metadata, use --db and --fields.\n"
			}
		}

		// 13. CLI Bridge Integration
		if bridgeCode != "" {
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			
			// Check size and provide hints if needed
			if len(outputStr) > 1024*1024 { // 1MB limit
				fmt.Fprintf(os.Stderr, "Hint: Output is large. Try reducing --indent, increasing --truncate, or your directory.\n")
			}

			fmt.Print(outputStr)
			return bridge.Execute(bridgeCode, outputStr, treeFormat, cmdStr, time.Since(startTime), dbName, forceInsert)
		}

		// Standard Output
		fmt.Print(outputStr)
		return nil
	},
}

func init() {
	treeCmd.Flags().StringVarP(&treeDB, "db", "d", "", "Database name for metadata enrichment")
	treeCmd.Flags().StringSliceVar(&treeFields, "fields", []string{}, "Metadata fields to display (comma-separated)")
	treeCmd.Flags().IntVar(&treeIndent, "indent", 4, "Indentation width in spaces")
	treeCmd.Flags().IntVar(&treeTruncate, "truncate", 60, "Maximum length for metadata values (0 for no truncation)")
	treeCmd.Flags().StringVar(&treeFormat, "format", "human", "Output format: human, json, or ai-portable")
	treeCmd.Flags().BoolVar(&treePrune, "prune", false, "Hide files/dirs that lack the requested metadata")
	
	// New Filter & Focus Flags
	treeCmd.Flags().StringArrayVarP(&treeFilters, "filter", "F", []string{}, "Filter by metadata field (e.g., 'layer=api')")
	treeCmd.Flags().StringArrayVarP(&treeFocus, "focus", "f", []string{}, "Restrict tree to specific paths or globs")
	treeCmd.Flags().BoolVar(&treeNoCompact, "no-compact", false, "Show filenames for non-matching files in the heat map")
}

// RegisterTreeCommand registers the tree command with the root command.
func RegisterTreeCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(treeCmd)
}
