/**
 * Component: Search Engine Interface
 * Block-UUID: 4598f3d7-910c-4fcc-aa8f-4af4d3cf659c
 * Parent-UUID: 32d253b4-b9f2-48bd-9cea-277419cfea67
 * Version: 2.2.0
 * Description: Defines the abstraction for search engines (ripgrep, git grep) and the shared data structures. Updated to return SearchResult with metadata.
 * Language: Go
 * Created-at: 2026-02-06T01:47:53.987Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), Gemini 3 Flash (v2.1.0), Gemini 3 Flash (v2.2.0)
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
	Pattern       string // The search pattern (regex or literal)
	ContextLines  int    // Number of context lines before and after matches (-C)
	CaseSensitive bool   // Whether the search is case-sensitive
	FileType      string // Optional file type filter (e.g., "go", "js")
	RequestedFields []string // Metadata fields to include in enrichment
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
