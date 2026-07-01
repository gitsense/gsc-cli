/**
 * Component: Lessons List Command
 * Block-UUID: 42414a2a-9e83-4315-b595-a97370dc506a
 * Parent-UUID: N/A
 * Version: 3.0.0
 * Description: Added --scope flag for scoped lesson listing.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v2.0.0), MiMo-v2.5-pro (v3.0.0)
 */

package lessons

import (
	"encoding/json"
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var (
		tag        string
		topic      string
		file       string
		importance string
		format     string
		limit      int
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List committed lessons as a filterable table",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			if err := lessonspkg.ValidateImportance(importance); err != nil {
				return err
			}
			sourcedRecords, err := lessonspkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			sourcedRecords = lessonspkg.FilterSourcedRecords(sourcedRecords, lessonspkg.ListFilter{
				Tag:        tag,
				Topic:      topic,
				File:       file,
				Importance: importance,
			})
			sourcedRecords = limitSourcedLessons(sourcedRecords, limit)
			return renderSourcedRecordList(sourcedRecords, format)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Only lessons whose tags match this value")
	cmd.Flags().StringVar(&topic, "topic", "", "Only lessons whose topics match this value")
	cmd.Flags().StringVar(&file, "file", "", "Only lessons that apply to a matching file path")
	cmd.Flags().StringVar(&importance, "importance", "", "Only lessons with this importance (high, medium, low)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of lessons to return (0 = all)")
	return cmd
}

func limitSourcedLessons(records []lessonspkg.SourcedLesson, limit int) []lessonspkg.SourcedLesson {
	if limit <= 0 || len(records) <= limit {
		return records
	}
	return records[:limit]
}

func renderSourcedRecordList(records []lessonspkg.SourcedLesson, format string) error {
	switch format {
	case "", "table":
		repoRecords, personalRecords := splitSourcedLessonsBySource(records)
		if len(repoRecords) > 0 {
			fmt.Println("Repo lessons:")
			fmt.Print(lessonspkg.RenderRecordsTable(lessonspkg.UnwrapSourcedLessons(repoRecords)))
		}
		if len(personalRecords) > 0 {
			if len(repoRecords) > 0 {
				fmt.Println()
			}
			fmt.Println("Personal lessons:")
			fmt.Print(lessonspkg.RenderRecordsTable(lessonspkg.UnwrapSourcedLessons(personalRecords)))
		}
		if len(records) == 0 {
			fmt.Print(lessonspkg.RenderRecordsTable(nil))
		}
		return nil
	case "json":
		if records == nil {
			records = []lessonspkg.SourcedLesson{}
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

func splitSourcedLessonsBySource(records []lessonspkg.SourcedLesson) (repo, personal []lessonspkg.SourcedLesson) {
	for _, record := range records {
		if record.Source == gitsensescope.SourceRepo {
			repo = append(repo, record)
		} else {
			personal = append(personal, record)
		}
	}
	return repo, personal
}

// renderRecordList prints records as a table or JSON, shared by list and search.
func renderRecordList(records []lessonspkg.Record, format string) error {
	switch format {
	case "", "table":
		fmt.Print(lessonspkg.RenderRecordsTable(records))
		return nil
	case "json":
		if records == nil {
			records = []lessonspkg.Record{}
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
