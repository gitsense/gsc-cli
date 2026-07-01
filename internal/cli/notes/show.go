/**
 * Component: Notes Show Command
 * Block-UUID: b0c1d2e3-f4a5-6789-bcde-901234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc notes show to display a single note in detail.
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

func showCmd() *cobra.Command {
	var (
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:          "show <id>",
		Short:        "Show a note in detail",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			// Load records from scope
			records, err := notespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}

			// Resolve from scoped records without losing source provenance.
			record, err := notespkg.ResolveSourcedRecordFromRecords(args[0], records)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("note not found in %s scope: %s", scope, args[0])
			}

			switch format {
			case "json":
				output := struct {
					Source gitsensescope.Source `json:"source"`
					Note   notespkg.Note        `json:"note"`
				}{
					Source: record.Source,
					Note:   record.Note,
				}
				data, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			default:
				fmt.Printf("Source: %s\n\n", record.Source)
				fmt.Print(notespkg.RenderNoteDetail(record.Note))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")
	return cmd
}
