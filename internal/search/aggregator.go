/*
 * Component: Search Result Aggregator
 * Block-UUID: 1cdcd69a-6f7b-43ec-bf1d-6269ef6f5b1f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Aggregates enriched search matches into a quantitative summary, calculating metadata distributions and coverage.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package search

import (
	"fmt"
	"strings"
)

// AggregateMatches transforms raw enriched matches into a GrepSummary.
func AggregateMatches(matches []MatchResult) GrepSummary {
	summary := GrepSummary{
		TotalMatches:      len(matches),
		FieldDistribution: make(map[string]map[string]int),
	}

	fileMap := make(map[string]bool)
	analyzedFileMap := make(map[string]bool)

	for _, m := range matches {
		fileMap[m.FilePath] = true

		if len(m.Metadata) > 0 {
			analyzedFileMap[m.FilePath] = true
			
			// Aggregate categorical fields
			for field, val := range m.Metadata {
				valStr := fmt.Sprintf("%v", val)
				
				// Heuristic: Only aggregate short strings or lists (Categorical)
				if len(valStr) > 50 || strings.Contains(valStr, "\n") {
					continue
				}

				if summary.FieldDistribution[field] == nil {
					summary.FieldDistribution[field] = make(map[string]int)
				}
				summary.FieldDistribution[field][valStr]++
			}
		}
	}

	summary.TotalFiles = len(fileMap)
	summary.AnalyzedFiles = len(analyzedFileMap)
	summary.UnanalyzedFiles = summary.TotalFiles - summary.AnalyzedFiles

	return summary
}
