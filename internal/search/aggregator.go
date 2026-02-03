/*
 * Component: Search Result Aggregator
 * Block-UUID: 3054573b-5794-4365-8040-babcc6680464
 * Parent-UUID: 1cdcd69a-6f7b-43ec-bf1d-6269ef6f5b1f
 * Version: 2.0.0
 * Description: Aggregates enriched search matches into a quantitative summary. Updated to group matches by file, generate file summaries, and handle truncation limits.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0)
 */


package search

import (
	"fmt"
	"sort"
	"strings"
)

// AggregateMatches transforms raw enriched matches into a GrepSummary.
// It groups matches by file, calculates statistics, and applies truncation limits.
func AggregateMatches(matches []MatchResult, limit int) GrepSummary {
	summary := GrepSummary{
		TotalMatches:      len(matches),
		FieldDistribution: make(map[string]map[string]int),
		Files:             []FileSummary{},
	}

	fileMap := make(map[string]*FileSummary)

	for _, m := range matches {
		// 1. Group matches by file
		if _, exists := fileMap[m.FilePath]; !exists {
			fs := FileSummary{
				FilePath:   m.FilePath,
				Analyzed:   len(m.Metadata) > 0,
				MatchCount: 0,
			}
			if fs.Analyzed {
				fs.ChatID = &m.ChatID
				fs.Metadata = m.Metadata
			}
			fileMap[m.FilePath] = &fs
		}
		
		// 2. Increment match count for the file
		fileMap[m.FilePath].MatchCount++

		// 3. Aggregate categorical fields
		if len(m.Metadata) > 0 {
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

	// 4. Convert map to slice
	var allFiles []FileSummary
	for _, fs := range fileMap {
		allFiles = append(allFiles, *fs)
	}

	// 5. Calculate statistics
	summary.TotalFiles = len(allFiles)
	for _, fs := range allFiles {
		if fs.Analyzed {
			summary.AnalyzedFiles++
		}
	}

	// 6. Sort files by match count (descending) to prioritize interesting files
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].MatchCount > allFiles[j].MatchCount
	})

	// 7. Apply truncation limit
	if limit > 0 && len(allFiles) > limit {
		summary.Files = allFiles[:limit]
		summary.IsTruncated = true
	} else {
		summary.Files = allFiles
		summary.IsTruncated = false
	}

	return summary
}
