/*
 * Component: Grep Command
 * Block-UUID: 8ac98d11-e90a-483b-a406-5c34eac7786b
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for 'gsc grep'. Orchestrates search, enrichment, aggregation, and formatting.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/search"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	grepDB           string
	grepProfile      string
	grepSummary      bool
	grepContext      int
	grepCaseSensitive bool
	grepFileType     string
)

// grepCmd represents the grep command
var grepCmd = &cobra.Command{
	Use:   "grep <pattern>",
	Short: "Search code with metadata enrichment",
	Long: `Search for patterns in code using ripgrep and enrich results with metadata
from a manifest database. This allows you to see search results alongside
contextual information like risk levels, topics, or business impact.

The output is JSON formatted for AI agent consumption.

Modes:
  --summary    Returns only aggregated metadata (cheap, fast)
  (default)    Returns matches with context and metadata (expensive, detailed)`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		// 1. Load Effective Config (Merges active profile)
		config, err := manifest.GetEffectiveConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// 2. Resolve Database Name (flag > profile default > global default)
		dbName := grepDB
		if dbName == "" {
			dbName = config.Global.DefaultDatabase
		}

		if dbName == "" {
			return fmt.Errorf("database is required. Use --db flag or set a profile with 'gsc config use <name>'")
		}

		// 3. Resolve Context (flag > profile default)
		contextLines := grepContext
		if contextLines == 0 {
			// Fallback to RG settings if available, otherwise 0
			contextLines = config.RG.DefaultContext
		}

		// 4. Get Repository Info
		repoInfo, err := git.GetRepositoryInfo()
		if err != nil {
			logger.Debug("Failed to get repository info: %v", err)
			// Continue without repo info if it fails
			repoInfo = &git.RepositoryInfo{Name: "unknown", URL: "", Remote: ""}
		}

		// 5. Build Query Context
		queryContext := search.QueryContext{
			Pattern:    pattern,
			Database:   dbName,
			Profile:    grepProfile, // Use the flag value or empty string
			Repository: search.RepositoryInfo{
				Name:   repoInfo.Name,
				URL:    repoInfo.URL,
				Remote: repoInfo.Remote,
			},
			Timestamp: time.Now(),
		}

		// 6. Execute Search
		engine := &search.RipgrepEngine{}
		options := search.SearchOptions{
			Pattern:       pattern,
			ContextLines:  contextLines,
			CaseSensitive: grepCaseSensitive,
			FileType:      grepFileType,
		}

		rawMatches, err := engine.Search(cmd.Context(), options)
		if err != nil {
			return err
		}

		// 7. Enrich Matches
		enrichedMatches, err := search.EnrichMatches(cmd.Context(), rawMatches, dbName)
		if err != nil {
			return err
		}

		// 8. Aggregate Summary
		summary := search.AggregateMatches(enrichedMatches)

		// 9. Format and Output
		return search.FormatResponse(queryContext, &summary, enrichedMatches, grepSummary)
	},
}

func init() {
	// Add flags
	grepCmd.Flags().StringVarP(&grepDB, "db", "d", "", "Database name for enrichment (inherits from profile)")
	grepCmd.Flags().StringVarP(&grepProfile, "profile", "p", "", "Profile name to use (overrides active profile)")
	grepCmd.Flags().BoolVar(&grepSummary, "summary", false, "Return only the summary (no matches)")
	grepCmd.Flags().IntVarP(&grepContext, "context", "C", 0, "Show N lines of context around matches")
	grepCmd.Flags().BoolVar(&grepCaseSensitive, "case-sensitive", false, "Case-sensitive search")
	grepCmd.Flags().StringVarP(&grepFileType, "type", "t", "", "Filter by file type (e.g., js, py)")
}

// RegisterGrepCommand registers the grep command with the root command.
func RegisterGrepCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(grepCmd)
}
