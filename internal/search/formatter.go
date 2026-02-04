/*
 * Component: Search Response Formatter
 * Block-UUID: de1c4a8d-cdb9-46fb-a99b-b58e7a23021b
 * Parent-UUID: c3e82257-f44e-4fb0-8126-fe2826661f19
 * Version: 2.1.0
 * Description: Formats search results into the final JSON response structure. Updated to accept and include filter strings in the QueryContext.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0)
 */


package search

import (
	"encoding/json"
	"fmt"
)

// FormatResponse constructs the final JSON response and prints it to stdout.
func FormatResponse(context QueryContext, summary GrepSummary, matches []MatchResult, summaryOnly bool, filters []string) error {
	// Populate filters in the context to ensure they appear in the JSON output
	context.Filters = filters

	response := GrepResponse{
		Context: context,
		Summary: summary,
	}

	if !summaryOnly {
		// Group matches by file for the full response
		response.Files = GroupMatchesByFile(matches)
	}

	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON response: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// GroupMatchesByFile converts a flat list of matches into a list of file results.
func GroupMatchesByFile(matches []MatchResult) []FileResult {
	fileMap := make(map[string]*FileResult)

	for _, m := range matches {
		if _, exists := fileMap[m.FilePath]; !exists {
			fr := FileResult{
				FilePath: m.FilePath,
				Analyzed: len(m.Metadata) > 0,
				Matches:  []MatchDetail{},
			}
			if fr.Analyzed {
				fr.ChatID = &m.ChatID
				fr.Metadata = m.Metadata
			}
			fileMap[m.FilePath] = &fr
		}

		// Append match detail
		fileMap[m.FilePath].Matches = append(fileMap[m.FilePath].Matches, MatchDetail{
			LineNumber:    m.LineNumber,
			LineText:      m.LineText,
			ContextBefore: m.ContextBefore,
			ContextAfter:  m.ContextAfter,
		})
	}

	// Convert map to slice
	var results []FileResult
	for _, fr := range fileMap {
		results = append(results, *fr)
	}

	return results
}
