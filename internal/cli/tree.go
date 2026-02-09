/**
 * Component: Tree Command
 * Block-UUID: be91509e-f36e-44af-8b35-716b9bc76fbb
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for 'gsc tree'. Supports hierarchical visualization of tracked files with metadata enrichment, CWD-awareness, and CLI Bridge integration.
 * Language: Go
 * Created-at: 2026-02-09T20:01:20.286Z
 * Authors: Gemini 3 Flash (v1.0.0)
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
	treeDB       string
	treeFields   []string
	treeIndent   int
	treeTruncate int
	treeFormat   string
	treePrune    bool
)

// treeCmd represents the tree command
var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display a hierarchical view of tracked files with metadata",
	Long: `Display a hierarchical view of tracked files enriched with metadata from a 
manifest database. Unlike the standard 'tree' command, this respects .gitignore 
and focuses on the repository's intelligence map.

The command is context-aware and will start the tree from your current working 
directory. Use --fields to include specific metadata like 'purpose' or 'risk'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// 0. Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				return err
			}
		}

		// Validate format
		treeFormat = strings.ToLower(treeFormat)
		if treeFormat != "human" && treeFormat != "json" {
			return fmt.Errorf("invalid format: %s. Supported formats: human, json", treeFormat)
		}

		// 1. Get Repository Context
		repoRoot, cwdOffset, err := git.GetRepoContext()
		if err != nil {
			return fmt.Errorf("failed to get repository context: %w", err)
		}

		// 2. Get Tracked Files
		files, err := git.GetTrackedFiles(cmd.Context(), repoRoot)
		if err != nil {
			return fmt.Errorf("failed to get tracked files: %w", err)
		}

		// 3. Build Initial Tree
		rootNode := tree.BuildTree(files, cwdOffset)

		// 4. Resolve Database and Fetch Metadata if requested
		dbName := treeDB
		if dbName == "" {
			config, _ := manifest.GetEffectiveConfig()
			dbName = config.Global.DefaultDatabase
		}

		if dbName != "" {
			dbName, err = registry.ResolveDatabase(dbName)
			if err != nil {
				return err
			}

			// Resolve DB Path and Open
			dbPath, err := manifest.ResolveDBPath(dbName)
			if err != nil {
				return err
			}

			// We use the search package's internal logic to fetch metadata in batch.
			// Note: We assume fetchMetadataMap will be exported as FetchMetadataMap 
			// or wrapped in a public function in the next step.
			metadataMap, _, err := search.FetchMetadataMap(cmd.Context(), dbPath, files, "all", nil, treeFields, nil)
			if err != nil {
				logger.Debug("Failed to fetch metadata for tree", "error", err)
			} else {
				// 5. Enrich Tree
				tree.EnrichTree(rootNode, "", metadataMap)
			}
		}

		// 6. Prune if requested
		if treePrune {
			tree.PruneTree(rootNode)
		}

		// 7. Calculate Stats
		stats := tree.CalculateStats(rootNode)

		// 8. Render Output
		var outputStr string
		if treeFormat == "json" {
			outputStr, err = tree.RenderJSON(rootNode, stats, dbName, treeFields, treePrune, cwdOffset)
			if err != nil {
				return fmt.Errorf("failed to render JSON: %w", err)
			}
		} else {
			outputStr = tree.RenderHuman(rootNode, treeIndent, treeTruncate, treeFields)
			
			// Append Summary Report
			outputStr += fmt.Sprintf("\nTree Coverage Summary:\n")
			outputStr += fmt.Sprintf("  Total Tracked Files: %d\n", stats.TotalFiles)
			outputStr += fmt.Sprintf("  Analyzed:            %d (%.1f%%)\n", stats.AnalyzedFiles, stats.Coverage)
			outputStr += fmt.Sprintf("  Unmapped:            %d\n", stats.TotalFiles-stats.AnalyzedFiles)
			outputStr += fmt.Sprintf("\nNote: This tree only includes files tracked by Git.\n")

			if dbName == "" && len(treeFields) == 0 {
				outputStr += "Hint: To include metadata, use --db and --fields.\n"
			}
		}

		// 9. CLI Bridge Integration
		if bridgeCode != "" {
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			
			// Check size and provide hints if needed
			if len(outputStr) > 1024*1024 { // 1MB limit
				fmt.Fprintf(os.Stderr, "Hint: Output is large. Try reducing --indent, increasing --truncate, or narrowing your directory.\n")
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
	treeCmd.Flags().StringVar(&treeFormat, "format", "human", "Output format: human or json")
	treeCmd.Flags().BoolVar(&treePrune, "prune", false, "Hide files/dirs that lack the requested metadata")
}

// RegisterTreeCommand registers the tree command with the root command.
func RegisterTreeCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(treeCmd)
}
