/**
 * Component: Pi CLI Root Command
 * Block-UUID: d8b3f5a1-2e74-4c69-8f30-1a7d9c4e6b25
 * Parent-UUID: 9f3a1c47-6e2b-4d80-8a14-1b7c2e5f9d3a
 * Version: 1.5.0
 * Description: Defines the top-level gsc pi command group: the -r/--resume flag launches the interactive Pi session picker, -b/--brains shows session statistics, --hud picks a session then opens a tmux split (pi + HUD sidebar), and --hud-panel runs that sidebar.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), claude-opus-4-8 (v1.1.0, v1.2.0, v1.3.0), MiMo-v2.5-Pro (v1.4.0, v1.5.0)
 */


package pi

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/cli/pi/sessions"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var resume bool
	var brains bool
	var hud bool
	var hudPanel string
	var dbPath string

	cmd := &cobra.Command{
		Use:          "pi",
		Short:        "Query and sync Pi-specific local data",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// --hud-panel runs the sidebar; tmux launches it, not the user.
			if hudPanel != "" {
				return runHudPanel(dbPath, hudPanel)
			}
			// --hud picks a session, then opens a tmux split (pi + sidebar).
			if hud {
				return runHudPicker(dbPath, args)
			}
			// -r/--resume launches the interactive session picker.
			if resume {
				return runResumePicker(dbPath, args)
			}
			// -b/--brains shows session statistics.
			if brains {
				var sessionID string
				if len(args) > 0 {
					sessionID = args[0]
				}
				return runStats(cmd.OutOrStdout(), dbPath, sessionID, "human")
			}
			if len(args) > 0 {
				return unknownCommandError(cmd, args[0])
			}
			return cmd.Help()
		},
	}
	cmd.Flags().BoolVarP(&resume, "resume", "r", false, "Pick a Pi session to resume (interactive)")
	cmd.Flags().BoolVarP(&brains, "brains", "b", false, "Show session statistics (context, model, files, brains)")
	cmd.Flags().BoolVar(&hud, "hud", false, "Pick a Pi session and open it in a tmux split with a HUD sidebar")
	cmd.Flags().StringVar(&hudPanel, "hud-panel", "", "Run the HUD sidebar for the given session ID (used internally by --hud)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi/pi-sessions.sqlite3)")
	_ = cmd.Flags().MarkHidden("hud-panel")
	cmd.AddCommand(sessions.NewCmd())
	cmd.AddCommand(NewGuideCmd())
	return cmd
}

func unknownCommandError(cmd *cobra.Command, arg string) error {
	return fmt.Errorf("unknown command %q for %q", arg, cmd.CommandPath())
}
