/**
 * Component: Lessons Show Command
 * Block-UUID: b857e8de-e352-481e-8cfb-dc88c668cea3
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Added --scope flag for scoped lesson display with source provenance.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func showCmd() *cobra.Command {
	var (
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:          "show <lesson-id>",
		Short:        "Show a committed lesson",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			// Load records from scope
			records, err := lessonspkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}

			// Resolve from scoped records (preserves source, handles cross-source ambiguity)
			sourced, err := lessonspkg.ResolveSourcedRecordFromRecords(args[0], records)
			if err != nil {
				return err
			}
			if sourced == nil {
				return fmt.Errorf("lesson not found in %s scope: %s", scope, args[0])
			}

			switch format {
			case "", "table":
				fmt.Printf("Source: %s\n\n", sourced.Source)
				fmt.Print(lessonspkg.RenderRecord(sourced.Lesson))
				return nil
			case "json":
				output := struct {
					Source gitsensescope.Source `json:"source"`
					Lesson lessonspkg.Record    `json:"lesson"`
				}{
					Source: sourced.Source,
					Lesson: sourced.Lesson,
				}
				data, err := json.MarshalIndent(output, "", "  ")
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
	return cmd
}
