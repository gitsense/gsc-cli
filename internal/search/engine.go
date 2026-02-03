/*
 * Component: Search Engine Interface
 * Block-UUID: 35ddc0ad-d190-41cd-a4a7-b45d30d16956
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the abstraction for search engines (ripgrep, git grep) and the shared data structures for raw search results.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package search

import "context"

// SearchEngine defines the interface for code search implementations.
type SearchEngine interface {
	// Search executes a search with the given options and returns raw matches.
	Search(ctx context.Context, options SearchOptions) ([]RawMatch, error)
}

// SearchOptions configures the search behavior.
type SearchOptions struct {
	Pattern       string // The search pattern (regex or literal)
	ContextLines  int    // Number of context lines before and after matches (-C)
	CaseSensitive bool   // Whether the search is case-sensitive
	FileType      string // Optional file type filter (e.g., "go", "js")
}

// RawMatch represents a single match result from a search engine.
// It contains the line text and context, but no metadata enrichment yet.
type RawMatch struct {
	FilePath      string   // The path to the file
	LineNumber    int      // The line number of the match
	LineText      string   // The full text of the matching line
	ContextBefore []string // Context lines before the match
	ContextAfter  []string // Context lines after the match
}
