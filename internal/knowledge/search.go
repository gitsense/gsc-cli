/**
 * Component: Knowledge Search
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000003
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements unified search across lessons, notes, and rules with tokenized matching and relevance ranking.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package knowledge

import (
	"sort"
	"strings"
)

// SearchOptions controls search behavior.
type SearchOptions struct {
	Types []string // Filter by entity types
	Topic string   // Filter by topic
	Limit int      // Max results (0 = unlimited)
}

// Search performs a unified search across all knowledge documents.
func Search(query string, opts SearchOptions) (*SearchResponse, error) {
	// Build index
	indexOpts := IndexOptionsFromTypes(opts.Types)
	docs, err := BuildIndex(indexOpts)
	if err != nil {
		return nil, err
	}

	// Filter by topic if specified
	if opts.Topic != "" {
		var filtered []Document
		for _, doc := range docs {
			if strings.EqualFold(doc.Topic, opts.Topic) {
				filtered = append(filtered, doc)
			}
		}
		docs = filtered
	}

	// Tokenize query
	tokens := tokenize(query)

	// Score each document
	var results []SearchResult
	for _, doc := range docs {
		score, matchedBy := scoreDocument(doc, tokens)
		if score > 0 {
			results = append(results, SearchResult{
				Type:       doc.Type,
				ID:         doc.ID,
				Topic:      doc.Topic,
				Summary:    doc.Summary,
				Importance: doc.Importance,
				MatchedBy:  matchedBy,
				Score:      score,
			})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	// Build facets
	facets := buildFacets(results)

	return &SearchResponse{Items: results, Facets: facets}, nil
}

// tokenize splits a query into lowercase tokens.
func tokenize(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	// Split on whitespace
	parts := strings.Fields(q)
	var tokens []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

// scoreDocument scores a document against query tokens.
func scoreDocument(doc Document, tokens []string) (float64, []string) {
	score := 0.0
	var matchedBy []string

	// Exact topic match
	for _, token := range tokens {
		if strings.EqualFold(doc.Topic, token) {
			score += 1.0
			matchedBy = append(matchedBy, "topic:"+doc.Topic)
			break
		}
	}

	// Exact tag match
	for _, tag := range doc.Tags {
		for _, token := range tokens {
			if strings.EqualFold(tag, token) {
				score += 0.8
				matchedBy = append(matchedBy, "tag:"+tag)
				break
			}
		}
	}

	// Summary term matches
	summaryMatches := countTermMatches(doc.Summary, tokens)
	if summaryMatches > 0 {
		score += 0.6 * float64(summaryMatches)
		matchedBy = append(matchedBy, "summary")
	}

	// Body term matches
	bodyMatches := countTermMatches(doc.Body, tokens)
	if bodyMatches > 0 {
		score += 0.4 * float64(bodyMatches)
		matchedBy = append(matchedBy, "body")
	}

	// Importance boost
	if doc.Importance == "high" {
		score *= 1.2
	}

	return score, matchedBy
}

// countTermMatches counts how many tokens appear in the text.
func countTermMatches(text string, tokens []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, token := range tokens {
		if strings.Contains(lower, token) {
			count++
		}
	}
	return count
}

// buildFacets counts items by type.
func buildFacets(results []SearchResult) map[string]int {
	facets := map[string]int{
		"lessons": 0,
		"notes":   0,
		"rules":   0,
	}
	for _, r := range results {
		switch r.Type {
		case TypeLesson:
			facets["lessons"]++
		case TypeNote:
			facets["notes"]++
		case TypeRule:
			facets["rules"]++
		}
	}
	return facets
}
