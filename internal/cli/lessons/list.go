/**
 * Component: Lessons List Command
 * Block-UUID: 42414a2a-9e83-4315-b595-a97370dc506a
 * Parent-UUID: N/A
 * Version: 2.1.0
 * Description: Added tag/topic/file/importance filters, table/json output, a limit, and importance validation for lesson discovery.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v2.0.0), claude-opus-4-8 (v2.1.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"

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
	)
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List committed lessons as a filterable table",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := lessonspkg.ValidateImportance(importance); err != nil {
				return err
			}
			records, err := lessonspkg.LoadRecords()
			if err != nil {
				return err
			}
			records = lessonspkg.FilterRecords(records, lessonspkg.ListFilter{
				Tag:        tag,
				Topic:      topic,
				File:       file,
				Importance: importance,
			})
			records = lessonspkg.LimitRecords(records, limit)
			return renderRecordList(records, format)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Only lessons whose tags match this value")
	cmd.Flags().StringVar(&topic, "topic", "", "Only lessons whose topics match this value")
	cmd.Flags().StringVar(&file, "file", "", "Only lessons that apply to a matching file path")
	cmd.Flags().StringVar(&importance, "importance", "", "Only lessons with this importance (high, medium, low)")
	cmd.Flags().StringVarP(&format, "format", "o", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of lessons to return (0 = all)")
	return cmd
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
