/**
 * Component: Pi CLI Root Command
 * Block-UUID: cf0bd40a-cd97-4d6b-9575-63610bc597f7
 * Parent-UUID: 8c6d3dc9-732f-4c40-b8d1-d2c1531c3034
 * Version: 1.1.0
 * Description: Defines the top-level gsc pi command group and adds -r/--resume and -q flags that launch the interactive Pi session picker.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v1.1.0)
 */


package pi

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/cli/pi/sessions"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var resume bool
	var query string
	var dbPath string

	cmd := &cobra.Command{
		Use:          "pi",
		Short:        "Query and sync Pi-specific local data",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// -r/--resume or -q launch the interactive session picker.
			if resume || query != "" {
				return runResumePicker(dbPath, query, args)
			}
			if len(args) > 0 {
				return unknownCommandError(cmd, args[0])
			}
			return cmd.Help()
		},
	}
	cmd.Flags().BoolVarP(&resume, "resume", "r", false, "Pick a Pi session to resume (interactive)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "Pick a session whose content matches the query, then resume")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi-sessions.sqlite3)")
	cmd.AddCommand(sessions.NewCmd())
	return cmd
}

func unknownCommandError(cmd *cobra.Command, arg string) error {
	return fmt.Errorf("unknown command %q for %q", arg, cmd.CommandPath())
}
