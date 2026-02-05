/*
 * Component: Grep Command
 * Block-UUID: 2d5a73d8-f430-4d64-a8f3-c0ef7f96b78a
 * Parent-UUID: 4f6c369b-0cf0-405f-8893-eae869a53152
 * Version: 3.3.0
 * Description: CLI command definition for 'gsc grep'. Updated to support metadata filtering, stats recording, and case-sensitive defaults. Updated to resolve database names from user input or config to physical names. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted Info logs to Debug and set SilenceUsage to true.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v3.0.0), GLM-4.7 (v3.1.0), GLM-4.7 (v3.2.0), GLM-4.7 (v3.3.0)
 */


package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/registry"
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
	grepLimit        int
	grepFilters      []string
	grepAnalyzed     string
	grepFiles        []string
	grepNoStats      bool
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
  (default)    Returns matches with context and metadata (expensive, detailed)

Filtering:
  --filter "field=value"    Filter by metadata fields (e.g., topic=security)
  --analyzed [true|false]   Show only analyzed or unanalyzed files
  --file "pattern"          Filter by file path (supports wildcards)`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		startTime := time.Now()

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

		// Resolve database name to physical name
		if dbName != "" {
			dbName, err = registry.ResolveDatabase(dbName)
			if err != nil {
				return err
			}
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
			logger.Debug("Failed to get repository info", "error", err)
			// Continue without repo info if it fails
			repoInfo = &git.RepositoryInfo{Name: "unknown", URL: "", Remote: ""}
		}

		// 5. Get System Info
		sysInfo, err := git.GetSystemInfo()
		if err != nil {
			logger.Debug("Failed to get system info", "error", err)
			// Continue with minimal info if it fails
			sysInfo = &git.SystemInfo{OS: "unknown", ProjectRoot: ""}
		}

		// 6. Parse Filters
		filters, err := search.ParseFilters(cmd.Context(), grepFilters, dbName)
		if err != nil {
			return fmt.Errorf("failed to parse filters: %w", err)
		}

		// 7. Execute Search
		engine := &search.RipgrepEngine{}
		options := search.SearchOptions{
			Pattern:       pattern,
			ContextLines:  contextLines,
			CaseSensitive: grepCaseSensitive,
			FileType:      grepFileType,
		}

		searchResult, err := engine.Search(cmd.Context(), options)
		if err != nil {
			return err
		}

		// 8. Enrich Matches (with filters)
		enrichedMatches, err := search.EnrichMatches(cmd.Context(), searchResult.Matches, dbName, filters, grepAnalyzed, grepFiles)
		if err != nil {
			return err
		}

		// 9. Aggregate Summary
		summary := search.AggregateMatches(enrichedMatches, grepLimit)

		// 10. Build Query Context
		mode := "full"
		if grepSummary {
			mode = "summary"
		}

		queryContext := search.QueryContext{
			Pattern:    pattern,
			Database:   dbName,
			Mode:       mode,
			Tool: search.ToolInfo{
				Name:      searchResult.ToolName,
				Version:   searchResult.ToolVersion,
				Arguments: optionsToArgs(options),
				TotalMs:   searchResult.DurationMs,
			},
			SearchScope: search.SearchScope{
				FileType:      grepFileType,
				ContextLines:  contextLines,
				CaseSensitive: grepCaseSensitive,
			},
			System: search.SystemInfo{
				OS:          sysInfo.OS,
				ProjectRoot: sysInfo.ProjectRoot,
			},
			Repository: search.RepositoryInfo{
				Name:   repoInfo.Name,
				URL:    repoInfo.URL,
				Remote: repoInfo.Remote,
			},
			Timestamp: time.Now(),
		}

		// 11. Format and Output
		if err := search.FormatResponse(queryContext, summary, enrichedMatches, grepSummary, grepFilters); err != nil {
			return err
		}

		// 12. Record Stats (Async/Fire-and-Forget)
		if !grepNoStats {
			duration := time.Since(startTime)
			
			// Serialize filters and file patterns for storage
			filtersJSON, _ := json.Marshal(grepFilters)
			filesJSON, _ := json.Marshal(grepFiles)

			searchRecord := search.SearchRecord{
				Timestamp:      time.Now(),
				Pattern:        pattern,
				ToolName:       searchResult.ToolName,
				ToolVersion:    searchResult.ToolVersion,
				DurationMs:     int(duration.Milliseconds()),
				TotalMatches:   summary.TotalMatches,
				TotalFiles:     summary.TotalFiles,
				AnalyzedFiles:  summary.AnalyzedFiles,
				FiltersUsed:    string(filtersJSON),
				DatabaseName:   dbName,
				CaseSensitive:  grepCaseSensitive,
				FileFilters:    string(filesJSON),
				AnalyzedFilter: grepAnalyzed,
			}

			// Record in background, don't block output
			go func() {
				if err := search.RecordSearch(cmd.Context(), searchRecord); err != nil {
					logger.Debug("Failed to record search stats", "error", err)
				}
			}()
		}

		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors
}

func init() {
	// Add flags
	grepCmd.Flags().StringVarP(&grepDB, "db", "d", "", "Database name for enrichment (inherits from profile)")
	grepCmd.Flags().StringVarP(&grepProfile, "profile", "p", "", "Profile name to use (overrides active profile)")
	grepCmd.Flags().BoolVar(&grepSummary, "summary", false, "Return only the summary (no matches)")
	grepCmd.Flags().IntVarP(&grepContext, "context", "C", 0, "Show N lines of context around matches")
	grepCmd.Flags().BoolVar(&grepCaseSensitive, "case-sensitive", true, "Case-sensitive search (default: true)")
	grepCmd.Flags().StringVarP(&grepFileType, "type", "t", "", "Filter by file type (e.g., js, py)")
	grepCmd.Flags().IntVar(&grepLimit, "limit", 50, "Limit the number of files in the summary (0 for no limit)")
	
	// New Filter Flags
	grepCmd.Flags().StringArrayVar(&grepFilters, "filter", []string{}, "Filter by metadata field (e.g., 'topic=security')")
	grepCmd.Flags().StringVar(&grepAnalyzed, "analyzed", "all", "Filter by analysis status: true, false, or all (default: all)")
	grepCmd.Flags().StringArrayVar(&grepFiles, "file", []string{}, "Filter by file path pattern (supports wildcards)")
	grepCmd.Flags().BoolVar(&grepNoStats, "no-stats", false, "Disable recording of search statistics")
}

// optionsToArgs converts SearchOptions to a slice of arguments for display.
func optionsToArgs(options search.SearchOptions) []string {
	args := []string{"--json", "--no-heading"}
	
	if options.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", options.ContextLines))
	}
	
	if !options.CaseSensitive {
		args = append(args, "--smart-case")
	}
	
	if options.FileType != "" {
		args = append(args, fmt.Sprintf("--type=%s", options.FileType))
	}
	
	args = append(args, options.Pattern)
	return args
}

// RegisterGrepCommand registers the grep command with the root command.
func RegisterGrepCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(grepCmd)
}
