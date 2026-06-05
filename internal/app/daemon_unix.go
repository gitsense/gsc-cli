//go:build !windows

/**
 * Component: Daemon Unix
 * Block-UUID: 7683f117-f67d-4d31-9cab-0a8c5edb9ed5
 * Parent-UUID: f39fe3b1-15ee-4c5a-95fc-3df6d69984ba
 * Version: 1.4.0
 * Description: Re-executes the current binary as a detached background process on Unix systems. The _GSC_DAEMON_CHILD=1 sentinel breaks the re-exec cycle in the child. The parent polls for the PID file to confirm startup before returning. Updated to use shared DisplayStartupMessage function for consistent enhanced startup messaging.
 * Language: Go
 * Created-at: 2026-05-12T14:43:03.342Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
 */


package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func Daemonize(opts SupervisorOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	args := []string{
		"app", "native", "start",
		"--app-dir", opts.AppDir,
		"--data-dir", opts.DataDir,
		"--port", opts.Port,
		"--max-retries", strconv.Itoa(opts.MaxRetries),
	}
	if opts.EnvFile != "" {
		args = append(args, "--env-file", opts.EnvFile)
	}

	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "_GSC_DAEMON_CHILD=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	var pid int
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		running, _, childPid, _ := IsProcessRunning(opts.DataDir)
		if running {
			pid = childPid
			break
		}
	}

	if pid == 0 {
		_ = cmd.Process.Kill()
		logFile := filepath.Join(opts.DataDir, "logs", "app.log")
		return fmt.Errorf("daemon failed to start within 15 seconds\nCheck logs at: %s", logFile)
	}

	_ = cmd.Process.Release()
	
	// Display enhanced startup message using shared function
	if err := DisplayStartupMessage(opts.DataDir, opts.Port, false); err != nil {
		// If startup verification failed, exit with error code
		os.Exit(1)
	}
	
	// Exit successfully after displaying the message
	os.Exit(0)
	return nil
}
