/**
 * Component: Pi Sessions Sync Stop Command
 * Block-UUID: [to-be-generated]
 * Parent-UUID: c18409b8-dda6-4426-8b61-03eb43d1a1ce
 * Version: 1.0.0
 * Description: Stops the running Pi sessions sync watcher by sending SIGTERM and cleaning up the PID file.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package sessions

import (
	"encoding/json"
	"fmt"
	"time"

	app "github.com/gitsense/gsc-cli/internal/app"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

type syncStopResult struct {
	Status           string `json:"status"`
	ShutdownMethod   string `json:"shutdown_method"`
	ShutdownDuration string `json:"shutdown_duration"`
	Error            string `json:"error,omitempty"`
}

func syncStopCmd(config *syncConfig) *cobra.Command {
	var format string
	var force bool

	cmd := &cobra.Command{
		Use:          "stop",
		Short:        "Stop the running Pi sessions sync watcher",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncStop(cmd, config, format, force)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	cmd.Flags().BoolVar(&force, "force", false, "Force kill if graceful shutdown fails")
	return cmd
}

func runSyncStop(cmd *cobra.Command, config *syncConfig, format string, force bool) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("resolve GSC_HOME: %w", err)
	}

	piDataDir := settings.GetPiGscDataDir(gscHome)

	// Check if process is running
	running, supervisorPid, _, err := app.IsProcessRunning(piDataDir)
	if err != nil {
		return fmt.Errorf("check process status: %w", err)
	}

	if !running {
		result := syncStopResult{
			Status:         "not_running",
			ShutdownMethod: "none",
		}
		return writeSyncStopResult(cmd, result, format)
	}

	// Attempt graceful shutdown
	stopStartTime := time.Now()
	if err := app.StopProcess(piDataDir); err != nil {
		return fmt.Errorf("stop process: %w", err)
	}

	// Wait for process to exit
	maxWaitTime := 10 * time.Second
	checkInterval := 100 * time.Millisecond
	elapsed := time.Duration(0)
	processExited := false

	for elapsed < maxWaitTime {
		running, _, _, err := app.IsProcessRunning(piDataDir)
		if err != nil || !running {
			processExited = true
			break
		}
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	shutdownDuration := time.Since(stopStartTime)

	result := syncStopResult{
		ShutdownDuration: formatDuration(shutdownDuration),
	}

	if processExited {
		result.Status = "stopped"
		result.ShutdownMethod = "Graceful (SIGTERM)"
	} else {
		result.Status = "timeout"
		result.ShutdownMethod = "Timeout"
		result.Error = fmt.Sprintf("Process %d did not exit within %v", supervisorPid, maxWaitTime)

		if force {
			// Force kill not implemented yet - would need to send SIGKILL
			result.Error += " (force kill not implemented)"
		}
	}

	return writeSyncStopResult(cmd, result, format)
}

func writeSyncStopResult(cmd *cobra.Command, result syncStopResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "human", "":
		out := cmd.OutOrStdout()

		if result.Status == "not_running" {
			fmt.Fprintf(out, "Pi sessions sync watcher is not running\n")
			return nil
		}

		fmt.Fprintf(out, "Pi sessions sync watcher stopped\n")
		fmt.Fprintf(out, "  Status:      %s\n", result.Status)
		fmt.Fprintf(out, "  Shutdown:    %s\n", result.ShutdownMethod)
		fmt.Fprintf(out, "  Duration:    %s\n", result.ShutdownDuration)

		if result.Error != "" {
			fmt.Fprintf(out, "  Warning:     %s\n", result.Error)
		}

		fmt.Fprintf(out, "\n")
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
