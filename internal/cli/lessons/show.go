/**
 * Component: Lessons Show Command
 * Block-UUID: b857e8de-e352-481e-8cfb-dc88c668cea3
 * Parent-UUID: N/A
 * Version: 1.2.0
 * Description: Accepts a full lesson ID or a unique short-ID prefix via ResolveRecord, with table/json output.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v1.1.0), claude-opus-4-8 (v1.2.0)
 */


package lessons

import (
	"encoding/json"
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func showCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:          "show <lesson-id>",
		Short:        "Show a committed lesson",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lessonspkg.ResolveRecord(args[0])
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("lesson not found: %s", args[0])
			}
			switch format {
			case "", "table":
				fmt.Print(lessonspkg.RenderRecord(*record))
				return nil
			case "json":
				data, err := json.MarshalIndent(record, "", "  ")
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
	return cmd
}
