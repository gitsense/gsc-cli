//go:build !windows

/**
 * Component: Pi Sessions Sync Start Unix Daemonization
 * Block-UUID: [to-be-generated]
 * Parent-UUID: 2e36c783-d48f-407e-b5ae-e7ff9f674fa2
 * Version: 1.0.0
 * Description: Re-executes the current binary as a detached background process for Pi sessions sync on Unix systems. Uses _GSC_SYNC_CHILD=1 sentinel to break the re-exec cycle. Parent polls for PID file to confirm startup before exiting.
 * Language: Go
 * Created-at: 2026-06-20T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */



package sessions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	app "github.com/gitsense/gsc-cli/internal/app"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

func daemonizeSync(cmd *cobra.Command, sessionsDir, dbPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	args := []string{
		"pi", "sessions", "sync", "start",
		"--sessions-dir", sessionsDir,
		"--db", dbPath,
	}

	child := exec.Command(exe, args...)
	child.Env = append(os.Environ(), "_GSC_SYNC_CHILD=1")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	child.Stdin = nil
	child.Stdout = nil
	child.Stderr = nil

	if err := child.Start(); err != nil {
		return fmt.Errorf("failed to start sync daemon: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		_ = child.Process.Kill()
		return fmt.Errorf("resolve GSC_HOME: %w", err)
	}

	piDataDir := settings.GetPiGscDataDir(gscHome)
	pidPath := filepath.Join(piDataDir, settings.AppPIDFileName)

	// Poll for PID file confirmation
	deadline := time.Now().Add(15 * time.Second)
	pidWritten := false
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		if _, err := os.Stat(pidPath); err == nil {
			pidWritten = true
			break
		}
	}

	if !pidWritten {
		_ = child.Process.Kill()
		logPath := settings.GetPiSyncLogPath(gscHome)
		return fmt.Errorf("sync daemon failed to start within 15 seconds\nCheck logs at: %s", logPath)
	}

	_ = child.Process.Release()

	// Display startup message
	running, _, childPid, err := app.IsProcessRunning(piDataDir)
	if err != nil || !running {
		return fmt.Errorf("sync daemon started but process not found")
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Pi sessions sync daemon started\n")
	fmt.Fprintf(out, "  PID:           %d\n", childPid)
	fmt.Fprintf(out, "  Sessions Dir:  %s\n", sessionsDir)
	fmt.Fprintf(out, "  Database:      %s\n", dbPath)
	fmt.Fprintf(out, "  Log:           %s\n", settings.GetPiSyncLogPath(gscHome))
	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "Monitor with:  gsc pi sessions sync status\n")
	fmt.Fprintf(out, "Stop with:     gsc pi sessions sync stop\n")
	fmt.Fprintf(out, "\n")

	os.Exit(0)
	return nil
}
