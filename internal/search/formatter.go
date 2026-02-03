/*
 * Component: Search Response Formatter
 * Block-UUID: ba32a464-227a-46ec-a660-643f1f80087f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Formats search results into the final JSON response structure, handling both summary-only and full match modes.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package search

import (
	"encoding/json"
	"fmt"
)

// FormatResponse constructs the final JSON response and prints it to stdout.
func FormatResponse(context QueryContext, summary *GrepSummary, matches []MatchResult, summaryOnly bool) error {
	response := GrepResponse{
		Context: context,
	}

	if summaryOnly {
		response.Summary = summary
	} else {
		response.Matches = matches
	}

	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON response: %w", err)
	}

	fmt.Println(string(data))
	return nil
}
