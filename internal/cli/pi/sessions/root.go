/**
 * Component: Pi Sessions CLI Root Command
 * Block-UUID: b9bc1d3a-b6fa-42a7-9c0b-795c2862ab2f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the gsc pi sessions command group for phase-one sync and query operations.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "sessions",
		Short:        "Sync and query Pi session JSONL",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(syncCmd())
	cmd.AddCommand(queryCmd())
	cmd.AddCommand(listCmd())
	cmd.AddCommand(showCmd())
	cmd.AddCommand(verifyCmd())
	return cmd
}
