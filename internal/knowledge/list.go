/**
 * Component: Knowledge List
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000004
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added sorting by updated, importance, or type with --sort and --asc flags.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0)
 */


package knowledge

import (
	"sort"
	"strings"
)

// SortField represents the field to sort by.
type SortField string

const (
	SortUpdated    SortField = "updated"
	SortImportance SortField = "importance"
	SortType       SortField = "type"
)

// importanceRank returns a numeric rank for importance sorting (lower = more important).
func importanceRank(imp string) int {
	switch strings.ToLower(imp) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4 // unknown/empty
	}
}

// ListOptions controls list behavior.
type ListOptions struct {
	Topic  string    // Filter by topic (required)
	Types  []string  // Filter by entity types
	Limit  int       // Max results (0 = unlimited)
	Fields []string  // Fields to include in output
	Sort   SortField // Sort field (updated, importance, type)
	Asc    bool      // Sort ascending (default: descending)
}

// List returns knowledge items for a specific topic.
func List(opts ListOptions) (*ListResponse, error) {
	// Build index
	indexOpts := IndexOptionsFromTypes(opts.Types)
	docs, err := BuildIndex(indexOpts)
	if err != nil {
		return nil, err
	}

	// Filter by topic (primary or related)
	var filtered []Document
	for _, doc := range docs {
		if strings.EqualFold(doc.Topic, opts.Topic) {
			filtered = append(filtered, doc)
			continue
		}
		for _, rt := range doc.RelatedTopics {
			if strings.EqualFold(rt, opts.Topic) {
				filtered = append(filtered, doc)
				break
			}
		}
	}

	// Convert to results
	var results []SearchResult
	for _, doc := range filtered {
		results = append(results, SearchResult{
			Type:       doc.Type,
			ID:         doc.ID,
			Topic:      doc.Topic,
			Summary:    doc.Summary,
			Importance: doc.Importance,
			MatchedBy:  []string{"topic:" + doc.Topic},
			Score:      1.0, // All topic matches have equal score
			UpdatedAt:  doc.UpdatedAt,
		})
	}

	// Sort results
	SortResults(results, opts.Sort, opts.Asc)

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	// Build facets
	facets := buildFacets(results)

	return &ListResponse{Items: results, Facets: facets}, nil
}

// SortResults sorts search results by the given field and direction.
func SortResults(results []SearchResult, field SortField, asc bool) {
	if field == "" {
		field = SortUpdated // default
	}

	sort.Slice(results, func(i, j int) bool {
		var less bool
		switch field {
		case SortUpdated:
			less = results[i].UpdatedAt.After(results[j].UpdatedAt)
		case SortImportance:
			less = importanceRank(results[i].Importance) < importanceRank(results[j].Importance)
		case SortType:
			if results[i].Type == results[j].Type {
				// Secondary sort by updated descending
				less = results[i].UpdatedAt.After(results[j].UpdatedAt)
			} else {
				less = results[i].Type < results[j].Type
			}
		default:
			less = results[i].UpdatedAt.After(results[j].UpdatedAt)
		}
		if asc {
			return !less
		}
		return less
	})
}
