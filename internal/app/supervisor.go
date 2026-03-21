/**
 * Component: App Process Supervisor
 * Block-UUID: 2cace8f2-8a80-479e-89bb-fc2fb2249cc0
 * Parent-UUID: d0614170-b46e-48f8-80a2-b107508a698e
 * Version: 1.1.0
 * Description: Implements the core process supervision logic, including the retry loop, signal forwarding, and lifecycle management for the Node.js application.
 * Language: Go
 * Created-at: 2026-03-20T23:46:36.653Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// SupervisorOptions defines the configuration for the process supervisor
type SupervisorOptions struct {
	AppDir     string
	DataDir    string
	EnvFile    string
	Port       string
	Foreground bool
	MaxRetries int
}

// StartSupervisor initializes the supervisor and begins the process lifecycle
func StartSupervisor(opts SupervisorOptions) error {
	if !opts.Foreground {
		// TODO: Implement daemonization logic for native background execution.
		// For now, we focus on the foreground supervisor required for Docker.
		return fmt.Errorf("daemon mode is not yet implemented; please use --foreground")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal trapping for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Shutdown signal received", "signal", sig.String())
		cancel()
	}()

	return runSupervisorLoop(ctx, opts)
}

// runSupervisorLoop manages the retry logic and process spawning
func runSupervisorLoop(ctx context.Context, opts SupervisorOptions) error {
	retryCount := 0
	startTime := time.Now()

	// Initialize log writer once outside the loop to prevent file handle leaks
	outputWriter, err := GetOutputWriters(opts.DataDir, opts.Foreground)
	if err != nil {
		return fmt.Errorf("failed to initialize log writer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Check if we are within the retry window
			if time.Since(startTime) > settings.AppRetryWindow {
				// Reset window and count if we've been stable
				startTime = time.Now()
				retryCount = 0
			}

			if retryCount > opts.MaxRetries {
				return fmt.Errorf("max retries (%d) reached; application failed to remain stable", opts.MaxRetries)
			}

			err := spawnProcess(ctx, opts, outputWriter)
			
			// If context was cancelled, exit normally
			if ctx.Err() != nil {
				return nil
			}

			if err != nil {
				retryCount++
				logger.Error("Application crashed", "error", err, "retry", retryCount, "max", opts.MaxRetries)
				
				// Wait a moment before restarting to prevent tight-looping
				time.Sleep(2 * time.Second)
				continue
			}

			// If process exited cleanly (exit code 0), we don't necessarily restart 
			// unless it's intended to be a persistent service.
			logger.Info("Application exited cleanly")
			return nil
		}
	}
}

// spawnProcess handles the actual execution of the Node.js application
func spawnProcess(ctx context.Context, opts SupervisorOptions, output io.Writer) error {
	// 1. Prepare Command
	// We assume 'node' is in the PATH as per Dockerfile requirements
	cmd := exec.CommandContext(ctx, "node", "index.js")
	cmd.Dir = opts.AppDir

	// 2. Setup Environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GSC_HOME=%s", settings.DockerRootPrefix))
	cmd.Env = append(cmd.Env, fmt.Sprintf("DEVBOARD_PORT=%s", opts.Port))

	// 3. Setup Output
	cmd.Stdout = output
	cmd.Stderr = output

	// 4. Start Process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start node process: %w", err)
	}

	logger.Info("Application process spawned", "pid", cmd.Process.Pid, "dir", opts.AppDir)

	// 5. Write PID
	if err := WritePID(opts.DataDir, cmd.Process.Pid); err != nil {
		logger.Warning("Failed to write PID file", "error", err)
	}

	// 6. Wait for exit
	err := cmd.Wait()

	// 7. Cleanup PID
	if err := RemovePID(opts.DataDir); err != nil {
		logger.Warning("Failed to remove PID file", "error", err)
	}

	return err
}
