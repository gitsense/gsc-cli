/**
 * Component: App Process Supervisor
 * Block-UUID: 253e7e23-4ad3-4d49-92d9-3e000fbd7563
 * Parent-UUID: dd39692d-8510-404b-ac78-e723ece21f02
 * Version: 2.1.0
 * Description: Implements the core process supervision logic, including the retry loop, signal forwarding, and lifecycle management for the Node.js application. Updated to track supervisor PID, manage health status, trigger log rotation on crashes, and pass port configuration to health status for accurate status reporting.
 * Language: Go
 * Created-at: 2026-05-12T01:20:17.325Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), Gemini 2.5 Flash Lite (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0)
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
		if os.Getenv("_GSC_DAEMON_CHILD") != "1" {
			return Daemonize(opts)
		}
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

	// Write supervisor PID immediately (before starting child)
	supervisorPid := os.Getpid()
	if err := WritePID(opts.DataDir, 0); err != nil {
		logger.Warning("Failed to write supervisor PID file", "error", err)
	}

	// Initialize health status with port configuration
	health := InitializeHealthStatus(opts.DataDir, supervisorPid, opts.Port)
	if err := SaveHealthStatus(opts.DataDir, health); err != nil {
		logger.Warning("Failed to initialize health status", "error", err)
	}

	return runSupervisorLoop(ctx, opts, health)
}

// runSupervisorLoop manages the retry logic and process spawning
func runSupervisorLoop(ctx context.Context, opts SupervisorOptions, health *HealthStatus) error {
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
			// Update health status before exiting
			health.Status = "stopped"
			health.ChildPID = 0
			_ = SaveHealthStatus(opts.DataDir, health)
			return nil
		default:
			// Check if we are within the retry window
			if time.Since(startTime) > settings.AppRetryWindow {
				// Reset window and count if we've been stable
				startTime = time.Now()
				retryCount = 0
			}

			if retryCount > opts.MaxRetries {
				health.Status = "stopped"
				health.ChildPID = 0
				_ = SaveHealthStatus(opts.DataDir, health)
				return fmt.Errorf("max retries (%d) reached; application failed to remain stable", opts.MaxRetries)
			}

			err := spawnProcess(ctx, opts, outputWriter, health)
			
			// If context was cancelled, exit normally
			if ctx.Err() != nil {
				return nil
			}

			if err != nil {
				// Trigger log rotation on crash
				if rotWriter, ok := outputWriter.(*RotatingLogWriter); ok {
					rotWriter.ScheduleRotation("crash")
				}

				// Update health status on crash
				now := time.Now().UTC()
				health.LastCrashAt = &now
				health.CrashCount++
				health.RestartCount++
				health.Status = "crashed"
				_ = SaveHealthStatus(opts.DataDir, health)
				
				retryCount++
				logger.Error("Application crashed", "error", err, "retry", retryCount, "max", opts.MaxRetries)
				
				// Wait a moment before restarting to prevent tight-looping
				time.Sleep(2 * time.Second)
				
				// Update status back to running before restart
				health.Status = "running"
				_ = SaveHealthStatus(opts.DataDir, health)
				continue
			}

			// If process exited cleanly (exit code 0), we don't necessarily restart 
			// unless it's intended to be a persistent service.
			health.Status = "stopped"
			health.ChildPID = 0
			_ = SaveHealthStatus(opts.DataDir, health)
			logger.Info("Application exited cleanly")
			return nil
		}
	}
}

// spawnProcess handles the actual execution of the Node.js application
func spawnProcess(ctx context.Context, opts SupervisorOptions, output io.Writer, health *HealthStatus) error {
	// 1. Prepare Command
	// We assume 'node' is in the PATH as per Dockerfile requirements
	cmd := exec.CommandContext(ctx, "node", "index.js")
	cmd.Dir = opts.AppDir

	// 2. Setup Environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GSC_HOME=%s", opts.AppDir))
	cmd.Env = append(cmd.Env, fmt.Sprintf("DEVBOARD_PORT=%s", opts.Port))

	// 3. Setup Output
	cmd.Stdout = output
	cmd.Stderr = output

	// 4. Start Process
	if err := cmd.Start(); err != nil {
		// In daemon mode, explicitly write errors to the log file
		if !opts.Foreground {
			fmt.Fprintf(output, "[ERROR] %s: Failed to start node process: %v\n", time.Now().Format(time.RFC3339), err)
		}
		return fmt.Errorf("failed to start node process: %w", err)
	}

	logger.Info("Application process spawned", "pid", cmd.Process.Pid, "dir", opts.AppDir)

	// 5. Update PID file with child PID
	if err := WritePID(opts.DataDir, cmd.Process.Pid); err != nil {
		// In daemon mode, explicitly write errors to the log file
		if !opts.Foreground {
			fmt.Fprintf(output, "[ERROR] %s: Failed to update PID file with child PID: %v\n", time.Now().Format(time.RFC3339), err)
		}
		logger.Warning("Failed to update PID file with child PID", "error", err)
	}

	// 6. Update health status with child PID
	health.ChildPID = cmd.Process.Pid
	_ = SaveHealthStatus(opts.DataDir, health)

	// 7. Wait for exit
	err := cmd.Wait()

	// 8. Cleanup PID
	if err := RemovePID(opts.DataDir); err != nil {
		// In daemon mode, explicitly write errors to the log file
		if !opts.Foreground {
			fmt.Fprintf(output, "[ERROR] %s: Failed to remove PID file: %v\n", time.Now().Format(time.RFC3339), err)
		}
		logger.Warning("Failed to remove PID file", "error", err)
	}

	return err
}
