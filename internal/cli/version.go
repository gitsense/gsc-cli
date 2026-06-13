/**
 * Component: Version CLI Command
 * Block-UUID: 4fa0762f-6760-4716-8575-00acc82630cf
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Adds the top-level version command for printing gsc build metadata.
 * Language: Go
 * Created-at: 2026-06-13T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package cli

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.GetVersion())
		},
	}
}
