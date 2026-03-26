/**
 * Component: Tree Command
 * Block-UUID: 40a19b61-70d6-42dd-9f3a-8cf6c7397e04
 * Parent-UUID: ddaceecb-fa19-4713-870d-0d889804d903
 * Version: 1.7.2
 * Description: Implemented 'prune by default when filtering' behavior. Added --no-prune flag to allow users to see the full heat map. Updated EnrichTree call to pass requested fields for metadata projection. Updated help text for --prune to reflect new defaults.
 * Language: Go
 * Created-at: 2026-03-11T15:31:46.846Z
 * Authors: GLM-4.7 (v1.7.1), GLM-4.7 (v1.7.2)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/internal/search"
	"github.com/gitsense/gsc-cli/internal/tree"
	"github.com/gitsense/gsc-cli/pkg/logger"
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
	treeFieldSingular []string
	treeNoPrune   bool
	treeUUID      string
	treeAuthCode  string
)

// treeCmd represents the tree command
var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Display a hierarchical view of tracked files with metadata",
	Long: `Display a hierarchical view of tracked files enriched with metadata from a 
manifest database. Unlike the standard 'tree' command, this respects .gitignore 
and focuses on the repository's intelligence map.

The command is context-aware and will start the tree from your current working 
directory. Use --fields to include specific metadata like 'purpose' or 'layer'.

Filtering & Pruning:
  --filter "field=val"      Filter by metadata. Supports 'in' for multiple values (e.g., layer in cli,logic)
  --prune                   Explicitly hide non-matching files (default when filtering)
  --no-prune                Show all files in the tree, marking matches (Heat Map mode)
  --focus "path/**"         Restrict the tree to specific paths or globs
  --no-compact              Show filenames for non-matching files in the heat map`,
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Handle UUID-based execution
		if treeUUID != "" {
			if treeAuthCode == "" {
				return fmt.Errorf("--authcode is required when using --uuid")
			}

			originalCwd, _ := os.Getwd()
			logger.Info("Executing tree command via contract", "uuid", treeUUID, "original_cwd", originalCwd)

			// Load contract using existing function
			meta, err := contract.GetContract(treeUUID)
			if err != nil {
				return fmt.Errorf("failed to load contract: %w", err)
			}

			// Validate authcode
			if meta.Authcode != treeAuthCode {
				return fmt.Errorf("invalid authorization code for contract %s", treeUUID)
			}

			if len(meta.Workdirs) == 0 {
				return fmt.Errorf("contract has no working directories defined")
			}
			workdir := meta.Workdirs[0].Path

			// Verify workdir exists before changing
			if info, err := os.Stat(workdir); err != nil || !info.IsDir() {
				return fmt.Errorf("contract workdir does not exist or is not a directory: %s", workdir)
			}

			// Change to workdir
			if err := os.Chdir(workdir); err != nil {
				return fmt.Errorf("failed to change to workdir %s: %w", workdir, err)
			}

			newCwd, _ := os.Getwd()
			logger.Info("Successfully changed working directory", "target", workdir, "actual", newCwd)
		}

		logger.Debug("Starting tree command execution", "db", treeDB, "format", treeFormat)

		// 0. Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				return err
			}
		}

		// Check for common typo: --field instead of --fields
		if cmd.Flags().Changed("field") {
			return fmt.Errorf("unknown flag: --field. Did you mean --fields?")
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
				// Pass treeFields to ensure only requested fields are projected into the node metadata
				tree.EnrichTree(rootNode, "", metadataMap, filters, treeFields)
			}
		} else if len(treeFilters) > 0 {
			return fmt.Errorf("database (--db) is required when using --filter")
		}

		// 9. Calculate Visibility (Propagate match status up the tree)
		tree.CalculateVisibility(rootNode)

		// 10. Determine Pruning Strategy
		// Default to pruning if filters are active, unless --no-prune is explicitly set
		shouldPrune := treePrune || (len(treeFilters) > 0 && !treeNoPrune)

		// 11. Prune if requested
		if shouldPrune {
			tree.PruneTree(rootNode)
		}

		// 12. Calculate Stats
		stats := tree.CalculateStats(rootNode)

		// 13. Render Output
		var outputStr string
		if treeFormat == "json" {
			outputStr, err = tree.RenderJSON(rootNode, stats, dbName, treeFields, filters, treeFocus, shouldPrune, cwdOffset)
			if err != nil {
				return fmt.Errorf("failed to render JSON: %w", err)
			}
		} else if treeFormat == "ai-portable" {
			outputStr, err = tree.RenderPortableJSON(rootNode, stats, treeFields, shouldPrune, cwdOffset)
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

		// 14. CLI Bridge Integration
		if bridgeCode != "" {
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			
			// Check size and provide hints if needed
			if len(outputStr) > 1024*1024 { // 1MB limit
				fmt.Fprintf(os.Stderr, "Hint: Output is large. Try reducing --indent, increasing --truncate, or your directory.\n")
			}

			fmt.Print(outputStr)
			return bridge.Execute(bridgeCode, outputStr, treeFormat, cmdStr, time.Since(startTime), dbName, 0, forceInsert)
		}

		// Standard Output
		fmt.Print(outputStr)
		return nil
	},
}

func init() {
	treeCmd.Flags().StringVarP(&treeDB, "db", "d", "", "Database name for metadata enrichment")
	treeCmd.Flags().StringSliceVar(&treeFields, "fields", []string{}, "Metadata fields to display (comma-separated)")
	treeCmd.Flags().StringSliceVar(&treeFieldSingular, "field", []string{}, "Did you mean --fields?")
	treeCmd.Flags().IntVar(&treeIndent, "indent", 4, "Indentation width in spaces")
	treeCmd.Flags().IntVar(&treeTruncate, "truncate", 60, "Maximum length for metadata values (0 for no truncation)")
	treeCmd.Flags().StringVar(&treeFormat, "format", "human", "Output format: human, json, or ai-portable")
	treeCmd.Flags().BoolVar(&treePrune, "prune", false, "Hide files/dirs that don't match the filters (default when filtering)")
	
	// New Filter & Focus Flags
	treeCmd.Flags().StringArrayVarP(&treeFilters, "filter", "F", []string{}, "Filter by metadata field. Supports 'in' (e.g., 'layer in cli,logic')")
	treeCmd.Flags().StringArrayVarP(&treeFocus, "focus", "f", []string{}, "Restrict tree to specific paths or globs")
	treeCmd.Flags().BoolVar(&treeNoCompact, "no-compact", false, "Show filenames for non-matching files in the heat map")
	treeCmd.Flags().BoolVar(&treeNoPrune, "no-prune", false, "Show all files in the tree, marking matches (Heat Map mode)")
	treeCmd.Flags().StringVar(&treeUUID, "uuid", "", "Contract UUID for remote execution")
	treeCmd.Flags().StringVar(&treeAuthCode, "authcode", "", "Contract authorization code")
}

// RegisterTreeCommand registers the tree command with the root command.
func RegisterTreeCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(treeCmd)
}
