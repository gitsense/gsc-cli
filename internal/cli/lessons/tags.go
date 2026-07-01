/**
 * Component: Lessons Tags Command
 * Block-UUID: 5c0a9e72-3f41-4d8b-9a6e-8b2d1c7f4e09
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Added --scope flag for scoped lesson tags.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func tagsCmd() *cobra.Command {
	var (
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List lesson tags and how many lessons use each",
		Long: `List the distinct tags across committed lessons, with lesson counts.

Tags are the connective vocabulary between lessons. Use this to discover what
tags exist, then "gsc lessons list --tag <tag>" to see the lessons under one.
JSON output includes the lesson IDs under each tag for programmatic linking.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			sourcedRecords, err := lessonspkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			records := lessonspkg.UnwrapSourcedLessons(sourcedRecords)
			facets := lessonspkg.LimitFacets(lessonspkg.CountFacet(records, "tags"), limit)
			switch format {
			case "", "table":
				fmt.Print(lessonspkg.RenderFacetTable(facets, "TAG"))
				return nil
			case "json":
				if facets == nil {
					facets = []lessonspkg.Facet{}
				}
				data, err := json.MarshalIndent(facets, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			default:
				return fmt.Errorf("unknown format %q (use table or json)", format)
			}
		},
	}
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of tags to return (0 = all)")
	return cmd
}
