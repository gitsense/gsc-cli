/**
 * Component: Notes Tags Command
 * Block-UUID: f8a9b0c1-d2e3-4567-fabc-789012345678
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc notes tags to list note tags with counts.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package notes

import (
	"encoding/json"
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	"github.com/spf13/cobra"
)

func tagsCmd() *cobra.Command {
	var (
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List note tags and how many notes use each",
		Long: `List note tags from repo, personal, or both scopes.

JSON output remains a plain array of tag facets for compatibility. With
--scope all, tag counts are merged across repo and personal notes.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}
			sourcedRecords, err := notespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			records := notespkg.UnwrapSourcedNotes(sourcedRecords)
			tags := notespkg.CountTags(records)
			switch format {
			case "json":
				if tags == nil {
					tags = []notespkg.TagFacet{}
				}
				data, err := json.MarshalIndent(tags, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			case "", "table":
				if len(tags) == 0 {
					fmt.Println("No tags found.")
					return nil
				}
				fmt.Printf("Scope: %s\n\n", notesScopeLabel(scope))
				if scope == gitsensescope.ScopeAll {
					repoRecords, personalRecords := splitSourcedNotesBySource(sourcedRecords)
					if len(repoRecords) > 0 {
						fmt.Println("Repo note tags:")
						fmt.Print(notespkg.RenderTagTable(notespkg.CountTags(notespkg.UnwrapSourcedNotes(repoRecords))))
					}
					if len(personalRecords) > 0 {
						if len(repoRecords) > 0 {
							fmt.Println()
						}
						fmt.Println("Personal note tags:")
						fmt.Print(notespkg.RenderTagTable(notespkg.CountTags(notespkg.UnwrapSourcedNotes(personalRecords))))
					}
					return nil
				}
				fmt.Print(notespkg.RenderTagTable(tags))
			default:
				return fmt.Errorf("unknown format %q (use table or json)", format)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}
