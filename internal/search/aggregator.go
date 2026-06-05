/**
 * Component: Search Result Aggregator
 * Block-UUID: b81a8aae-518f-469e-9eff-c5ecdaec8926
 * Parent-UUID: cdf2c02f-b2a1-45d2-987d-23568dd8cffd
 * Version: 2.2.0
 * Description: Aggregates enriched search matches into a quantitative summary. Updated to group matches by file, generate file summaries, and handle truncation limits.
 * Language: Go
 * Created-at: 2026-06-02T16:19:38.614Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), DeepSeek V4 Pro (v2.0.1), GLM-4.7 (v2.1.0), DeepSeek V4 Pro (v2.2.0)
 */


package search

import (
	"fmt"
	"sort"
	"strings"
)

// AggregateMatches transforms raw enriched matches into a GrepSummary.
// It groups matches by file, calculates statistics, and applies truncation limits.
func AggregateMatches(matches []MatchResult, limit int, minMatches int) GrepSummary {
	summary := GrepSummary{
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
				id := m.ChatID
				fs.ChatID = &id
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

	// 6. Sort files by match count (descending) to prioritize interesting files
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].MatchCount > allFiles[j].MatchCount
	})

	// 7. Filter files by minimum match count
	if minMatches > 0 {
		var filtered []FileSummary
		for _, fs := range allFiles {
			if fs.MatchCount >= minMatches {
				filtered = append(filtered, fs)
			}
		}
		allFiles = filtered
	}

	// 8. Calculate statistics AFTER filtering
	summary.TotalFiles = len(allFiles)
	summary.TotalMatches = 0
	for _, fs := range allFiles {
		summary.TotalMatches += fs.MatchCount
		if fs.Analyzed {
			summary.AnalyzedFiles++
		}
	}

	// 9. Apply truncation limit
	if limit > 0 && len(allFiles) > limit {
		summary.Files = allFiles[:limit]
		summary.IsTruncated = true
	} else if len(allFiles) > 0 {
		summary.Files = allFiles
		summary.IsTruncated = false
	}

	return summary
}
