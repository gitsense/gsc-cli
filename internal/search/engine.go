/**
 * Component: Search Engine Interface
 * Block-UUID: bbf8da98-126f-46ea-8ab2-ba61cfb02282
 * Parent-UUID: 4598f3d7-910c-4fcc-aa8f-4af4d3cf659c
 * Version: 2.3.0
 * Description: Defines the abstraction for search engines (ripgrep, git grep) and the shared data structures. Extended SearchOptions with new ripgrep-compatible flags: IgnoreCase, InvertMatch, WordRegexp, FixedStrings, Globs, MaxCount, Hidden, NoIgnore, and MultilinePatterns.
 * Language: Go
 * Created-at: 2026-02-06T01:47:53.987Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), Gemini 3 Flash (v2.1.0), Gemini 3 Flash (v2.2.0), GLM-4.7 (v2.3.0)
 */


package search

import "context"

// SearchEngine defines the interface for code search implementations.
type SearchEngine interface {
	// Search executes a search with the given options and returns search results.
	Search(ctx context.Context, options SearchOptions) (SearchResult, error)
}

// SearchOptions configures the search behavior.
type SearchOptions struct {
	Pattern          string   // The search pattern (regex or literal)
	ContextLines     int      // Number of context lines before and after matches (-C)
	CaseSensitive    bool     // Whether the search is case-sensitive
	IgnoreCase       bool     // Explicit case-insensitive search (-i flag)
	FileType         string   // Optional file type filter (e.g., "go", "js")
	RequestedFields  []string // Metadata fields to include in enrichment
	InvertMatch      bool     // Show non-matching lines (-v flag)
	WordRegexp       bool     // Match whole words only (-w flag)
	FixedStrings     bool     // Treat pattern as literal string, not regex (-F flag)
	Globs            []string // File path glob patterns (-g flag)
	MaxCount         int      // Limit matches per file (-m flag)
	Hidden           bool     // Include hidden files and directories (--hidden flag)
	NoIgnore         bool     // Don't respect .gitignore (--no-ignore flag)
	MultilinePatterns []string // Additional search patterns for OR logic (-e flag)
}

// RawMatch represents a single match result from a search engine.
// It contains the line text and context, but no metadata enrichment yet.
type RawMatch struct {
	FilePath      string   // The path to the file
	LineNumber    int      // The line number of the match
	LineText      string   // The full text of the matching line
	Submatches    []MatchOffset // Byte offsets for highlighting
	ContextBefore []string // Context lines before the match
	ContextAfter  []string // Context lines after the match
}

// SearchResult represents the output of a search engine execution.
type SearchResult struct {
	Matches     []RawMatch // The raw matches found
	ToolName    string     // The name of the tool used (e.g., "ripgrep")
	ToolVersion string     // The version of the tool
	DurationMs  int        // Execution time in milliseconds
}
