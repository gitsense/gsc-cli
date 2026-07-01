/**
 * Component: Knowledge Index Builder
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000002
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added UpdatedAt population for sorting by last modified date.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v1.1.0)
 */


package knowledge

import (
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
)

// BuildIndex loads all lessons, notes, and rules and returns normalized documents.
func BuildIndex(opts IndexOptions) ([]Document, error) {
	var docs []Document

	// Load lessons
	if opts.IncludeLessons {
		lessons, err := lessonspkg.LoadRecords()
		if err != nil {
			return nil, err
		}
		for _, l := range lessons {
			docs = append(docs, Document{
				Type:          TypeLesson,
				ID:            l.ID,
				Topic:         l.Topic,
				RelatedTopics: l.RelatedTopics,
				Tags:          l.Tags,
				Summary:       l.Summary,
				Body:          l.Details,
				Importance:    l.Importance,
				Files:         l.AppliesTo.Files,
				UpdatedAt:     l.UpdatedAt,
			})
		}
	}

	// Load notes
	if opts.IncludeNotes {
		notes, err := notespkg.LoadRecords()
		if err != nil {
			return nil, err
		}
		for _, n := range notes {
			docs = append(docs, Document{
				Type:          TypeNote,
				ID:            n.ID,
				Topic:         n.Topic,
				RelatedTopics: n.RelatedTopics,
				Tags:          n.Tags,
				Summary:       n.Summary,
				Body:          n.Content,
				Importance:    n.Importance,
				GlobPatterns:  n.GlobPatterns,
				UpdatedAt:     n.UpdatedAt,
			})
		}
	}

	// Load rules
	if opts.IncludeRules {
		rules, err := rulespkg.LoadRecords()
		if err != nil {
			return nil, err
		}
		for _, r := range rules {
			docs = append(docs, Document{
				Type:          TypeRule,
				ID:            r.ID,
				Topic:         r.Topic,
				RelatedTopics: r.RelatedTopics,
				Tags:          r.Tags,
				Summary:       r.Summary,
				Body:          r.Details,
				Importance:    r.Importance,
				Files:         r.AppliesTo.Files,
				GlobPatterns:  r.GlobPatterns,
				UpdatedAt:     r.UpdatedAt,
			})
		}
	}

	return docs, nil
}

// IndexOptions controls which entity types to include in the index.
type IndexOptions struct {
	IncludeLessons bool
	IncludeNotes   bool
	IncludeRules   bool
}

// DefaultIndexOptions includes all entity types.
func DefaultIndexOptions() IndexOptions {
	return IndexOptions{
		IncludeLessons: true,
		IncludeNotes:   true,
		IncludeRules:   true,
	}
}

// IndexOptionsFromTypes creates IndexOptions from a list of type names.
func IndexOptionsFromTypes(types []string) IndexOptions {
	opts := IndexOptions{}
	for _, t := range types {
		switch t {
		case "lessons":
			opts.IncludeLessons = true
		case "notes":
			opts.IncludeNotes = true
		case "rules":
			opts.IncludeRules = true
		}
	}
	// If no types specified, include all
	if !opts.IncludeLessons && !opts.IncludeNotes && !opts.IncludeRules {
		return DefaultIndexOptions()
	}
	return opts
}
