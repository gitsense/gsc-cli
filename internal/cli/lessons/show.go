/**
 * Component: Lessons Show Command
 * Block-UUID: b857e8de-e352-481e-8cfb-dc88c668cea3
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons show to display a single committed lesson by its lsn_<uuid-v7> identifier.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package lessons

import (
	"fmt"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func showCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "show <lesson-id>",
		Short:        "Show a committed lesson",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			record, err := lessonspkg.FindRecord(args[0])
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("lesson not found: %s", args[0])
			}
			fmt.Print(lessonspkg.RenderRecord(*record))
			return nil
		},
	}
	return cmd
}
