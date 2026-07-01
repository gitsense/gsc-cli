/**
 * Component: Pi Sessions Sync Command
 * Block-UUID: c18409b8-dda6-4426-8b61-03eb43d1a1ce
 * Parent-UUID: N/A
 * Version: 1.2.0
 * Description: Defines shared sync flags and registers continuous sync lifecycle commands (start/stop/status). One-shot sync removed in favor of start/stop lifecycle.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0, v1.1.0), MiMo-v2.5-Pro (v1.2.0)
 */

package sessions

import (
	"fmt"
	"path/filepath"

	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	return syncCmdWithDependencies(defaultSyncStartDependencies())
}

func syncCmdWithDependencies(startDependencies syncStartDependencies) *cobra.Command {
	config := &syncConfig{}

	cmd := &cobra.Command{
		Use:          "sync",
		Short:        "Manage continuous Pi session sync lifecycle (start/stop/status)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().StringVar(&config.sessionsDir, "sessions-dir", "", "Root directory containing Pi session JSONL files")
	cmd.PersistentFlags().StringVar(&config.dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi/pi-sessions.sqlite3)")
	cmd.AddCommand(syncStartCmd(config, startDependencies))
	cmd.AddCommand(syncStatusCmd(config))
	cmd.AddCommand(syncStopCmd(config))
	return cmd
}

type syncConfig struct {
	sessionsDir string
	dbPath      string
}

func resolvePiSessionsDBPath(value string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", err
	}
	return settings.GetPiSessionsDatabasePath(gscHome), nil
}
