/**
 * Component: Knowledge Topics
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000008
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Aggregates topic statistics across lessons, notes, and rules for gsc knowledge topics.
 * Language: Go
 * Created-at: 2026-06-24T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package knowledge

import (
	"sort"
	"strings"
	"time"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	topicspkg "github.com/gitsense/gsc-cli/internal/topics"
)

// TopicStats represents aggregated statistics for a topic.
type TopicStats struct {
	Slug          string    `json:"slug"`
	Description   string    `json:"description"`
	Lessons       int       `json:"lessons"`
	Notes         int       `json:"notes"`
	Rules         int       `json:"rules"`
	Total         int       `json:"total"`
	LatestUpdate  time.Time `json:"latest_update,omitempty"`
}

// TopicsOptions controls topics listing behavior.
type TopicsOptions struct {
	Sort  string // "count" (default) or "name"
	Asc   bool   // Sort ascending (default: descending for count, ascending for name)
	Empty bool   // Include topics with 0 items
}

// TopicsResponse is the response from knowledge topics.
type TopicsResponse struct {
	Topics []TopicStats `json:"topics"`
}

// Topics returns aggregated topic statistics.
func Topics(opts TopicsOptions) (*TopicsResponse, error) {
	// Load all topics
	topicRecords, err := topicspkg.LoadRecords()
	if err != nil {
		return nil, err
	}

	// Build a map of topic slug -> stats
	statsMap := make(map[string]*TopicStats)
	for _, t := range topicRecords {
		statsMap[t.Slug] = &TopicStats{
			Slug:        t.Slug,
			Description: t.Description,
		}
	}

	// Count lessons per topic
	lessons, err := lessonspkg.LoadRecords()
	if err != nil {
		return nil, err
	}
	for _, l := range lessons {
		if l.Topic == "" {
			continue
		}
		stats, ok := statsMap[l.Topic]
		if !ok {
			stats = &TopicStats{Slug: l.Topic}
			statsMap[l.Topic] = stats
		}
		stats.Lessons++
		stats.Total++
		if l.UpdatedAt.After(stats.LatestUpdate) {
			stats.LatestUpdate = l.UpdatedAt
		}
	}

	// Count notes per topic (primary and related)
	notes, err := notespkg.LoadRecords()
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		topics := []string{n.Topic}
		topics = append(topics, n.RelatedTopics...)
		for _, t := range topics {
			if t == "" {
				continue
			}
			stats, ok := statsMap[t]
			if !ok {
				stats = &TopicStats{Slug: t}
				statsMap[t] = stats
			}
			stats.Notes++
			stats.Total++
			if n.UpdatedAt.After(stats.LatestUpdate) {
				stats.LatestUpdate = n.UpdatedAt
			}
		}
	}

	// Count rules per topic
	rules, err := rulespkg.LoadRecords()
	if err != nil {
		return nil, err
	}
	for _, r := range rules {
		if r.Topic == "" {
			continue
		}
		stats, ok := statsMap[r.Topic]
		if !ok {
			stats = &TopicStats{Slug: r.Topic}
			statsMap[r.Topic] = stats
		}
		stats.Rules++
		stats.Total++
		if r.UpdatedAt.After(stats.LatestUpdate) {
			stats.LatestUpdate = r.UpdatedAt
		}
	}

	// Convert to slice
	var allStats []TopicStats
	for _, s := range statsMap {
		allStats = append(allStats, *s)
	}

	// Filter empty if requested
	if !opts.Empty {
		var filtered []TopicStats
		for _, s := range allStats {
			if s.Total > 0 {
				filtered = append(filtered, s)
			}
		}
		allStats = filtered
	}

	// Sort
	SortTopicStats(allStats, opts.Sort, opts.Asc)

	return &TopicsResponse{Topics: allStats}, nil
}

// SortTopicStats sorts topic stats by the given field and direction.
func SortTopicStats(stats []TopicStats, field string, asc bool) {
	if field == "" {
		field = "count"
	}

	sort.Slice(stats, func(i, j int) bool {
		var less bool
		switch field {
		case "name":
			// Name sort defaults to ascending
			less = strings.ToLower(stats[i].Slug) < strings.ToLower(stats[j].Slug)
			if asc {
				return less
			}
			return !less
		case "count":
			if stats[i].Total == stats[j].Total {
				// Secondary sort by name ascending
				return strings.ToLower(stats[i].Slug) < strings.ToLower(stats[j].Slug)
			}
			less = stats[i].Total > stats[j].Total
		default:
			less = stats[i].Total > stats[j].Total
		}
		if asc {
			return !less
		}
		return less
	})
}
