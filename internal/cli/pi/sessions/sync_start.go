/**
 * Component: Pi Sessions Sync Start Command
 * Block-UUID: 2e36c783-d48f-407e-b5ae-e7ff9f674fa2
 * Parent-UUID: c18409b8-dda6-4426-8b61-03eb43d1a1ce
 * Version: 1.2.0
 * Description: Connects the foreground sync start command to continuous reconciliation with graceful signal cancellation, PID file management, and detach mode support.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), MiMo-v2.5-Pro (v1.2.0)
 */


package sessions

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	app "github.com/gitsense/gsc-cli/internal/app"
	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

type syncStartDependencies struct {
	watch         func(context.Context, pisessions.WatchOptions) error
	notifyContext func(context.Context, ...os.Signal) (context.Context, context.CancelFunc)
	daemonize     func(cmd *cobra.Command, sessionsDir, dbPath string) error
}

func defaultSyncStartDependencies() syncStartDependencies {
	return syncStartDependencies{
		watch:         pisessions.Watch,
		notifyContext: signal.NotifyContext,
		daemonize:     daemonizeSync,
	}
}

func syncStartCmd(config *syncConfig, dependencies syncStartDependencies) *cobra.Command {
	var detached bool
	var debug bool
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Continuously sync Pi session JSONL in the foreground",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown argument %q for %q", args[0], cmd.CommandPath())
			}
			if detached {
				sessionsDir, err := resolvePiSessionsDir(config.sessionsDir)
				if err != nil {
					return err
				}
				resolvedDB, err := resolvePiSessionsDBPath(config.dbPath)
				if err != nil {
					return err
				}
				return dependencies.daemonize(cmd, sessionsDir, resolvedDB)
			}
			sessionsDir, err := resolvePiSessionsDir(config.sessionsDir)
			if err != nil {
				return err
			}
			resolvedDB, err := resolvePiSessionsDBPath(config.dbPath)
			if err != nil {
				return err
			}

			// Write PID file for status/stop commands
			gscHome, err := settings.GetGSCHome(false)
			if err != nil {
				return fmt.Errorf("resolve GSC_HOME: %w", err)
			}
			piDataDir := settings.GetPiGscDataDir(gscHome)
			if err := os.MkdirAll(piDataDir, 0755); err != nil {
				return fmt.Errorf("create Pi data directory: %w", err)
			}
			if err := app.WritePID(piDataDir, os.Getpid()); err != nil {
				return fmt.Errorf("write PID file: %w", err)
			}
			defer app.RemovePID(piDataDir)

			watchCtx, stop := dependencies.notifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			logPath := settings.GetPiSyncLogPath(gscHome)
			var debugLogPath string
			if debug {
				debugLogPath = settings.GetPiSyncDebugLogPath(gscHome)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Watching Pi sessions in %s\n", sessionsDir)
			return dependencies.watch(watchCtx, pisessions.WatchOptions{
				SessionsDir:  sessionsDir,
				DBPath:       resolvedDB,
				LogPath:      logPath,
				DebugLogPath: debugLogPath,
			})
		},
	}
	cmd.Flags().BoolVarP(&detached, "detach", "d", false, "Run sync in the background")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging to sync-debug.log")
	return cmd
}

func resolvePiSessionsDir(value string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".pi", "agent", "sessions"), nil
}
