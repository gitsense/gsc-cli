/**
 * Component: Notes List Command
 * Block-UUID: d6e7f8a9-b0c1-2345-defa-567890123456
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc notes list with tag/importance filters and table/json output.
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

func listCmd() *cobra.Command {
	var (
		tag        string
		topic      string
		importance string
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List notes as a filterable table",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			sourcedRecords, err := notespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			sourcedRecords = notespkg.FilterSourcedRecords(sourcedRecords, notespkg.ListFilter{
				Tag:        tag,
				Topic:      topic,
				Importance: importance,
			})
			if limit > 0 && len(sourcedRecords) > limit {
				sourcedRecords = sourcedRecords[:limit]
			}
			return renderSourcedRecordList(sourcedRecords, format)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Only notes whose tags match this value")
	cmd.Flags().StringVar(&topic, "topic", "", "Only notes whose topic matches this value")
	cmd.Flags().StringVar(&importance, "importance", "", "Only notes with this importance (high, medium, low)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of notes to return (0 = all)")
	return cmd
}

func renderSourcedRecordList(records []notespkg.SourcedNote, format string) error {
	switch format {
	case "", "table":
		repoRecords, personalRecords := splitSourcedNotesBySource(records)
		if len(repoRecords) > 0 {
			fmt.Println("Repo notes:")
			fmt.Print(notespkg.RenderNotesTable(notespkg.UnwrapSourcedNotes(repoRecords)))
		}
		if len(personalRecords) > 0 {
			if len(repoRecords) > 0 {
				fmt.Println()
			}
			fmt.Println("Personal notes:")
			fmt.Print(notespkg.RenderNotesTable(notespkg.UnwrapSourcedNotes(personalRecords)))
		}
		if len(records) == 0 {
			fmt.Print(notespkg.RenderNotesTable(nil))
		}
		return nil
	case "json":
		if records == nil {
			records = []notespkg.SourcedNote{}
		}
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unknown format %q (use table or json)", format)
	}
}

func splitSourcedNotesBySource(records []notespkg.SourcedNote) (repo, personal []notespkg.SourcedNote) {
	for _, record := range records {
		if record.Source == gitsensescope.SourceRepo {
			repo = append(repo, record)
		} else {
			personal = append(personal, record)
		}
	}
	return repo, personal
}

func renderRecordList(records []notespkg.Note, format string) error {
	switch format {
	case "", "table":
		fmt.Print(notespkg.RenderNotesTable(records))
		return nil
	case "json":
		if records == nil {
			records = []notespkg.Note{}
		}
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unknown format %q (use table or json)", format)
	}
}
