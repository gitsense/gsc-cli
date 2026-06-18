/**
 * Component: Pi CLI Root Command
 * Block-UUID: 8c6d3dc9-732f-4c40-b8d1-d2c1531c3034
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the top-level gsc pi command group for Pi-specific discovery tools.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package pi

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/cli/pi/sessions"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "pi",
		Short:        "Query and sync Pi-specific local data",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return unknownCommandError(cmd, args[0])
			}
			return cmd.Help()
		},
	}
	cmd.AddCommand(sessions.NewCmd())
	return cmd
}

func unknownCommandError(cmd *cobra.Command, arg string) error {
	return fmt.Errorf("unknown command %q for %q", arg, cmd.CommandPath())
}
