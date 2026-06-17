/**
 * Component: Lessons Tags Command
 * Block-UUID: 5c0a9e72-3f41-4d8b-9a6e-8b2d1c7f4e09
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons tags to enumerate the tag vocabulary connecting committed lessons.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func tagsCmd() *cobra.Command {
	var (
		format string
		limit  int
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
			records, err := lessonspkg.LoadRecords()
			if err != nil {
				return err
			}
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
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of tags to return (0 = all)")
	return cmd
}
