/**
 * Component: Grep/RG Command
 * Block-UUID: 7c935aed-ccff-41b1-a648-955f5a89add2
 * Parent-UUID: 93068c1b-d9bd-4545-9489-1baa8e3d5c3e
 * Version: 4.14.1
 * Description: Added 'rg' as a primary alias for the grep command to improve AI alignment with ripgrep syntax. Implemented fail-fast validation to detect POSIX-style alternation (\|) and provide helpful error messages. Updated help text to emphasize ripgrep syntax.
 * Language: Go
 * Created-at: 2026-06-02T14:57:27.776Z
 * Authors: GLM-4.7 (v4.8.0), Gemini 3 Flash (v4.9.0), GLM-4.7 (v4.10.0), GLM-4.7 (v4.11.0), GLM-4.7 (v4.12.0), GLM-4.7 (v4.13.0), GLM-4.7 (v4.14.0), DeepSeek V4 Pro (v4.14.1)
 */


package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/internal/search"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	grepDB           string
	grepProfile      string // INTERNAL: Retained for potential future use, but flag is hidden
	grepSummary      bool
	grepContext      int
	grepCaseSensitive bool
	grepIgnoreCase   bool
	grepFileType     string
	grepLimit        int
	grepFilters      []string
	grepFields       []string
	grepAnalyzed     string
	grepFieldSingular []string
	grepFiles        []string
	grepScope        string
	grepNoStats      bool
	grepFormat       string
	grepNoFields     bool
	grepInvert       bool
	grepWord         bool
	grepFixed        bool
	grepGlobs        []string
	grepMaxCount     int
	grepHidden       bool
	grepNoIgnore     bool
	grepMinMatches   int
	grepPatterns     []string
)

// grepCmd represents the grep command
var grepCmd = &cobra.Command{
	Use:     "grep <pattern>",
	Aliases: []string{"rg"},
	Short:   "Search code with metadata enrichment (Ripgrep syntax)",
	Long: `Search for patterns in code using ripgrep and enrich results with metadata
from a manifest database. 

⚠️  IMPORTANT: This command uses Ripgrep (Rust/ERE) syntax, NOT POSIX grep.
   Use | for alternation, not \|.
   Example: "pattern1|pattern2"

The output is human-readable by default, featuring a "Record/Card" layout with 
color-coded status indicators (✓/x), bold headers, and aligned metadata. 
Use --format json for AI consumption.

Modes:
  --summary    Returns only aggregated metadata (cheap, fast)
  (default)    Returns matches with context and metadata (expensive, detailed)

Filtering:
  --filter "field=value"    Filter by metadata fields (e.g., topic=security)
  --analyzed [true|false]   Show only analyzed or unanalyzed files
  --file "pattern"          Filter by file path (supports wildcards)

Ripgrep-compatible flags:
  -i, --ignore-case        Case-insensitive search
  -v, --invert-match       Show non-matching lines
  -w, --word-regexp        Match whole words only
  -F, --fixed-strings      Treat pattern as literal string (not regex)
  -g <glob>                Filter files by glob pattern
  -m <N>, --max-count <N>  Limit matches per file
  --hidden                 Include hidden files and directories
  --no-ignore              Don't respect .gitignore
  -e <pattern>             Add multiple search patterns (OR logic)`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Support both single pattern and multiple patterns via -e flag
		pattern := args[0]
		
		// Fail-fast validation for POSIX alternation
		if strings.Contains(pattern, `\|`) {
			return fmt.Errorf("invalid regex syntax: found '\\|' (POSIX BRE). gsc uses ripgrep (ERE) syntax. Use '|' for alternation instead. Example: \"pattern1|pattern2\"")
		}
		
		// Also check for escaped pipe in raw string format
		if strings.Contains(pattern, `\|`) {
			return fmt.Errorf("invalid regex syntax: found '\\|' (POSIX BRE). gsc uses ripgrep (ERE) syntax. Use '|' for alternation instead. Example: \"pattern1|pattern2\"")
		}

		if len(grepPatterns) > 0 {
			// If -e flags are used, combine them with the positional pattern
			grepPatterns = append([]string{pattern}, grepPatterns...)
			// Validate all patterns
			for _, p := range grepPatterns {
				if strings.Contains(p, `\|`) {
					return fmt.Errorf("invalid regex syntax in -e pattern: found '\\|'. Use '|' instead.")
				}
				// Also check for escaped pipe in raw string format
				if strings.Contains(p, `\|`) {
					return fmt.Errorf("invalid regex syntax in -e pattern: found '\\|'. Use '|' instead.")
				}
			}
		}

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

		// Validate format
		grepFormat = strings.ToLower(grepFormat)
		if grepFormat != "human" && grepFormat != "json" {
			return fmt.Errorf("invalid format: %s. Supported formats: human, json", grepFormat)
		}

		if grepNoFields && len(grepFields) > 0 {
			return fmt.Errorf("cannot use --fields and --no-fields together")
		}

		startTime := time.Now()

		// 1. Load Effective Config (Merges active profile internally)
		// INTERNAL: Profile logic is handled here but hidden from the user.
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
			return fmt.Errorf("database is required. Use --db flag.")
		}

		// 3. Resolve Context (flag > profile default)
		contextLines := grepContext
		if contextLines == 0 {
			// Fallback to RG settings if available, otherwise 0
			contextLines = config.RG.DefaultContext
		}

		// Resolve Fields (flag > profile default)
		requestedFields := grepFields
		if len(requestedFields) == 0 {
			requestedFields = config.RG.DefaultFields
		}

		// 3.5 Resolve Focus Scope
		// INTERNAL: We use the active profile name internally for scope resolution, but don't expose it.
		activeProfileName, _ := manifest.GetActiveProfileName()
		scope, err := manifest.ResolveScopeForQuery(cmd.Context(), activeProfileName, grepScope)
		if err != nil {
			return err
		}

		// 4. Get Repository Context (Root and CWD Offset)
		repoRoot, cwdOffset, err := git.GetRepoContext()
		if err != nil {
			logger.Debug("Failed to get repository context", "error", err)
			// Fallback to empty offset if not in a git repo
			cwdOffset = ""
		}

		// 4. Get Repository Info
		repoInfo, err := git.GetRepositoryInfo()
		if err != nil {
			logger.Debug("Failed to get repository info", "error", err)
			// Continue without repo info if it fails
			repoInfo = &git.RepositoryInfo{Name: "unknown", URL: "", Remote: ""}
		}

		// 5. Get System Info
		// We already have the repoRoot from GetRepoContext, so we can use it directly
		sysInfo, err := git.GetSystemInfo()
		if err != nil {
			logger.Debug("Failed to get system info", "error", err)
			// Continue with minimal info if it fails
			sysInfo = &git.SystemInfo{OS: "unknown", ProjectRoot: repoRoot}
		}

		// 6. Parse Filters
		filters, err := search.ParseFilters(cmd.Context(), grepFilters, dbName)
		if err != nil {
			return fmt.Errorf("failed to parse filters: %w", err)
		}

		// 7. Execute Search
		engine := &search.RipgrepEngine{}
		options := search.SearchOptions{
			Pattern:          pattern,
			ContextLines:     contextLines,
			CaseSensitive:    grepCaseSensitive,
			IgnoreCase:       grepIgnoreCase,
			FileType:         grepFileType,
			RequestedFields:  requestedFields,
			InvertMatch:      grepInvert,
			WordRegexp:       grepWord,
			FixedStrings:     grepFixed,
			Globs:            grepGlobs,
			MaxCount:         grepMaxCount,
			Hidden:           grepHidden,
			NoIgnore:         grepNoIgnore,
			MultilinePatterns: grepPatterns,
		}

		searchResult, err := engine.Search(cmd.Context(), options)
		if err != nil {
			return err
		}

		// 8. Enrich Matches (with filters and scope)
		// Note: We combine explicit --file flags with the Focus Scope
		filePatterns := grepFiles
		if scope != nil && len(scope.Include) > 0 {
			filePatterns = append(filePatterns, scope.Include...)
		}

		enrichedMatches, availableFields, matchesOutsideScope, err := search.EnrichMatches(cmd.Context(), searchResult.Matches, dbName, filters, grepAnalyzed, filePatterns, requestedFields, cwdOffset)
		if err != nil {
			return err
		}

		// 9. Aggregate Summary
		summary := search.AggregateMatches(enrichedMatches, grepLimit, grepMinMatches)
		summary.MatchesOutsideScope = matchesOutsideScope

		// 9.5. Filter detailed matches to match the summary files (respects --min-matches)
		summaryFileSet := make(map[string]bool)
		for _, fs := range summary.Files {
			summaryFileSet[fs.FilePath] = true
		}
		var filteredMatches []search.MatchResult
		for _, m := range enrichedMatches {
			if summaryFileSet[m.FilePath] {
				filteredMatches = append(filteredMatches, m)
			}
		}
		enrichedMatches = filteredMatches

		// If the summary was truncated due to limit, further filter to only the top N files
		if grepLimit > 0 && summary.IsTruncated {
			var filteredMatches []search.MatchResult
			// Iterate over the sorted summary files to maintain priority order
			for _, fs := range summary.Files {
				for _, m := range enrichedMatches {
					if m.FilePath == fs.FilePath {
						filteredMatches = append(filteredMatches, m)
					}
				}
			}
			enrichedMatches = filteredMatches
		}

		// 10. Build Query Context
		mode := "full"
		if grepSummary {
			mode = "summary"
		}

		scopeSummary := ""
		if scope != nil {
			scopeSummary = scope.GetSummary(cmd.Context(), repoRoot)
		}

		queryContext := search.QueryContext{
			Pattern:    pattern,
			Database:   dbName,
			ProfileName: activeProfileName, // INTERNAL: Kept for tracking, but not exposed in UI
			ScopeSummary: scopeSummary,
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
				IgnoreCase:    grepIgnoreCase,
				InvertMatch:   grepInvert,
				WordRegexp:    grepWord,
				FixedStrings:  grepFixed,
				Globs:         grepGlobs,
				MaxCount:      grepMaxCount,
				Hidden:        grepHidden,
				NoIgnore:      grepNoIgnore,
			},
			System: search.SystemInfo{
				OS:          sysInfo.OS,
				ProjectRoot: repoRoot, // Use the canonical root we discovered
			},
			Repository: search.RepositoryInfo{
				Name:   repoInfo.Name,
				URL:    repoInfo.URL,
				Remote: repoInfo.Remote,
			},
			Timestamp: time.Now(),
			RequestedFields: requestedFields,
		}

		// 11. Format and Output
		formatOpts := search.FormatOptions{
			Format:          grepFormat,
			SummaryOnly:     grepSummary,
			NoFields:        grepNoFields,
			RequestedFields: requestedFields,
			Filters:         grepFilters,
			NoColor:         bridgeCode != "",
			AvailableFields: availableFields,
		}

		// CLI Bridge Integration
		if bridgeCode != "" {
			// Debug: Log the bridge code received
			logger.Debug("CLI Bridge code received", "code", bridgeCode)

			// 1. Generate the formatted string
			outputStr, err := search.FormatResponseToString(queryContext, summary, enrichedMatches, formatOpts)
			if err != nil {
				return fmt.Errorf("failed to format bridge output: %w", err)
			}

			// 2. Print to stdout (as per spec: "display output as we normally would")
			fmt.Print(outputStr)

			// 3. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, grepFormat, cmdStr, time.Since(startTime), dbName, 0, forceInsert)
		}

		// Standard Output Mode
		if err := search.FormatResponse(queryContext, summary, enrichedMatches, formatOpts); err != nil {
			return err
		}

		// 12. Record Stats (Async/Fire-and-Forget)
		if !grepNoStats {
			duration := time.Since(startTime)
			
			// Serialize filters and file patterns for storage
			filtersJSON, _ := json.Marshal(grepFilters)
			filesJSON, _ := json.Marshal(grepFiles)
			fieldsJSON, _ := json.Marshal(requestedFields)
			globsJSON, _ := json.Marshal(grepGlobs)

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
				IgnoreCase:     grepIgnoreCase,
				InvertMatch:    grepInvert,
				WordRegexp:     grepWord,
				FixedStrings:   grepFixed,
				FileFilters:    string(filesJSON),
				AnalyzedFilter: grepAnalyzed,
				RequestedFields: string(fieldsJSON),
				Globs:          string(globsJSON),
				MaxCount:       grepMaxCount,
				Hidden:         grepHidden,
				NoIgnore:       grepNoIgnore,
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
	grepCmd.Flags().StringVarP(&grepDB, "db", "d", "", "Database name for enrichment")
	// INTERNAL: --profile flag removed to hide the feature from users
	// grepCmd.Flags().StringVarP(&grepProfile, "profile", "p", "", "Profile name to use (overrides active profile)")
	grepCmd.Flags().BoolVar(&grepSummary, "summary", false, "Return only the summary (no matches)")
	grepCmd.Flags().StringVar(&grepScope, "scope", "", "Temporary scope override (e.g., include=src/**;exclude=tests/**)")
	grepCmd.Flags().IntVarP(&grepContext, "context", "C", 0, "Show N lines of context around matches")
	grepCmd.Flags().BoolVar(&grepCaseSensitive, "case-sensitive", true, "Case-sensitive search (default: true)")
	grepCmd.Flags().BoolVarP(&grepIgnoreCase, "ignore-case", "i", false, "Case-insensitive search")
	grepCmd.Flags().StringVarP(&grepFileType, "type", "t", "", "Filter by file type (e.g., js, py)")
	grepCmd.Flags().IntVar(&grepLimit, "limit", 500, "Limit the number of files in the summary (0 for no limit)")
	
	// New Filter Flags
	grepCmd.Flags().StringArrayVar(&grepFilters, "filter", []string{}, "Filter by metadata field (e.g., 'topic=security')")
	grepCmd.Flags().StringSliceVar(&grepFields, "fields", []string{}, "Metadata fields to include in results (comma-separated)")
	grepCmd.Flags().StringSliceVar(&grepFieldSingular, "field", []string{}, "Did you mean --fields?")
	grepCmd.Flags().StringVar(&grepAnalyzed, "analyzed", "all", "Filter by analysis status: true, false, or all (default: all)")
	grepCmd.Flags().StringArrayVar(&grepFiles, "file", []string{}, "Filter by file path pattern (supports wildcards)")
	grepCmd.Flags().BoolVar(&grepNoStats, "no-stats", false, "Disable recording of search statistics")
	grepCmd.Flags().StringVar(&grepFormat, "format", "human", "Output format: human or json (default: human)")
	grepCmd.Flags().BoolVar(&grepNoFields, "no-fields", false, "Do not show metadata fields in the output")
	
	// New Ripgrep-compatible Flags
	grepCmd.Flags().BoolVarP(&grepInvert, "invert-match", "v", false, "Show non-matching lines")
	grepCmd.Flags().BoolVarP(&grepWord, "word-regexp", "w", false, "Match whole words only")
	grepCmd.Flags().BoolVarP(&grepFixed, "fixed-strings", "F", false, "Treat pattern as literal string (not regex)")
	grepCmd.Flags().StringArrayVarP(&grepGlobs, "glob", "g", []string{}, "Filter files by glob pattern")
	grepCmd.Flags().IntVarP(&grepMaxCount, "max-count", "m", 0, "Limit matches per file (0 for no limit)")
	grepCmd.Flags().BoolVar(&grepHidden, "hidden", false, "Include hidden files and directories")
	grepCmd.Flags().BoolVar(&grepNoIgnore, "no-ignore", false, "Don't respect .gitignore")
	grepCmd.Flags().StringArrayVarP(&grepPatterns, "regexp", "e", []string{}, "Add multiple search patterns (OR logic)")
	grepCmd.Flags().IntVar(&grepMinMatches, "min-matches", 0, "Only show files with at least N matches (0 for no limit)")
}

// optionsToArgs converts SearchOptions to a slice of arguments for display.
func optionsToArgs(options search.SearchOptions) []string {
	args := []string{"--json", "--no-heading"}
	
	if options.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", options.ContextLines))
	}
	
	// Case sensitivity (three-way logic)
	if options.IgnoreCase {
		args = append(args, "-i")
	} else if !options.CaseSensitive {
		args = append(args, "--smart-case")
	}
	
	if options.InvertMatch {
		args = append(args, "-v")
	}
	
	if options.WordRegexp {
		args = append(args, "-w")
	}
	
	if options.FixedStrings {
		args = append(args, "-F")
	}
	
	for _, glob := range options.Globs {
		args = append(args, "-g", glob)
	}
	
	if options.MaxCount > 0 {
		args = append(args, fmt.Sprintf("-m%d", options.MaxCount))
	}
	
	if options.Hidden {
		args = append(args, "--hidden")
	}
	
	if options.NoIgnore {
		args = append(args, "--no-ignore")
	}
	
	for _, pattern := range options.MultilinePatterns {
		args = append(args, "-e", pattern)
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
