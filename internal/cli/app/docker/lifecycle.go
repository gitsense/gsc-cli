/**
 * Component: Docker CLI Lifecycle Commands
 * Block-UUID: 5d164004-a369-49d7-bbec-5672547d2763
 * Parent-UUID: ff2816cc-5cdb-4f9b-91f6-22e768b7ca64
 * Version: 1.10.0
 * Description: Enhanced status command to show master file existence status alongside Docker Link status. Added clear indicators for both master file and Docker link to improve visibility into configuration state.
 * Language: Go
 * Created-at: 2026-05-13T01:55:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0)
 */


package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
)

var (
	logsFollow bool
)

// stopCmd represents the docker stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		name := "gitsense-chat"
		if dctx != nil {
			name = dctx.ContainerName
		}

		// Check if container is running before attempting to stop
		running, err := docker_internal.IsContainerRunning(ctx, name)
		if err != nil || !running {
			// If we can't check status or it's not running, report it as not running.
			// This prevents the confusing "failed to remove" error from docker rm.
			return fmt.Errorf("container '%s' is not running", name)
		}

		if err := docker_internal.StopContainer(ctx, name); err != nil {
			return err
		}

		fmt.Printf("Container '%s' stopped.\n", name)
		return nil
	},
}

// restartCmd represents the docker restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		name := "gitsense-chat"
		if dctx != nil {
			name = dctx.ContainerName
		}

		fmt.Printf("Restarting container '%s'...\n", name)
		restartCmd := exec.CommandContext(ctx, "docker", "restart", name)
		restartCmd.Stdout = os.Stdout
		restartCmd.Stderr = os.Stderr

		if err := restartCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}

		fmt.Printf("Container '%s' restarted successfully.\n", name)
		return nil
	},
}

// statusCmd represents the docker status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		if dctx == nil {
			fmt.Println("No active Docker context found. CLI is in native mode.")
			return nil
		}

		running, err := docker_internal.IsContainerRunning(ctx, dctx.ContainerName)
		if err != nil {
			return err
		}

		status := "Stopped"
		if running {
			status = "Running"
		}

		// Print enhanced status
		fmt.Println("\n" + "━" + strings.Repeat("━", 58))
		fmt.Println("  GitSense Chat Docker Status")
		fmt.Println("━" + strings.Repeat("━", 58))
		
		fmt.Printf("  Status:          %s\n", status)
		fmt.Printf("  Container:       %s\n", dctx.ContainerName)
		fmt.Printf("  Port:            %s\n", dctx.Port)
		
		// Get uptime and restart count from docker inspect
		if running {
			uptime, restartCount, err := getContainerStats(ctx, dctx.ContainerName)
			if err == nil {
				fmt.Printf("  Uptime:          %s\n", formatUptime(uptime))
				fmt.Printf("  Restart Count:   %d\n", restartCount)
			}
		}
		
		fmt.Println("")
		
		// Configuration section
		fmt.Println("  Configuration:")
		if dctx.ConfigType != "" {
			fmt.Printf("    Type:           %s\n", dctx.ConfigType)
			if dctx.ConfigType == "master" && dctx.MasterEnvPath != "" {
				// Check master file status
				masterStatus := "✗ [MISSING]"
				if _, err := os.Stat(dctx.MasterEnvPath); err == nil {
					masterStatus = "✓ [PRESENT]"
				}
				fmt.Printf("    Master File:    %s %s\n", dctx.MasterEnvPath, masterStatus)
			}
		} else {
			fmt.Printf("    Type:           Not configured\n")
		}
		
		// .env file status
		persistentEnvPath := filepath.Join(dctx.DataHostPath, ".env")
		envStatus := "✗ [MISSING]"
		if _, err := os.Stat(persistentEnvPath); err == nil {
			envStatus = "✓ [PRESENT]"
		}
		fmt.Printf("    Docker Link:    %s %s\n", persistentEnvPath, envStatus)
		
		// Sync status if linked
		if dctx.EnvHostPath != "" {
			inSync, err := checkEnvSync(dctx.EnvHostPath, persistentEnvPath)
			if err != nil {
				fmt.Printf("    Sync Status:    Error checking sync\n")
			} else if inSync {
				fmt.Printf("    Sync Status:    Up to date\n")
			} else {
				fmt.Printf("    Sync Status:    Out of sync (run 'gsc app docker env update')\n")
			}
		}
		
		fmt.Println("")
		
		// Volumes section
		fmt.Println("  Volumes:")
		fmt.Printf("    Data:           %s\n", dctx.DataHostPath)
		fmt.Printf("    Repos:          %s\n", dctx.ReposHostPath)
		
		fmt.Println("")
		
		// Tip based on configuration type
		fmt.Println("  Note:")
		if dctx.ConfigType == "master" {
			fmt.Println("    This configuration is shared with native mode.")
			fmt.Println("    Edit the master file to update both environments.")
		} else if dctx.ConfigType == "separate" {
			fmt.Println("    This configuration is Docker-only.")
			fmt.Println("    Edit the .env file in the data directory.")
		} else if dctx.ConfigType == "existing" {
			fmt.Println("    This configuration links to an existing file.")
			fmt.Println("    Run 'gsc app docker env update' to sync changes.")
		} else {
			fmt.Println("    Run 'gsc app docker env init' to configure your environment.")
		}
		
		fmt.Println("━" + strings.Repeat("━", 58))
		
		return nil
	},
}

// logsCmd represents the docker logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View the logs of the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		name := "gitsense-chat"
		if dctx != nil {
			name = dctx.ContainerName
		}

		return docker_internal.GetLogs(ctx, name, logsFollow)
	},
}

// shellCmd represents the docker shell command
var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Open an interactive bash session inside the container",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		name := "gitsense-chat"
		if dctx != nil {
			name = dctx.ContainerName
		}

		return docker_internal.ExecCommand(ctx, name, []string{"/bin/bash"}, true, "")
	},
}

// disconnectCmd represents the docker disconnect command
var disconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect from the Docker environment and return to native mode",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		if err := docker_internal.DeleteContext(); err != nil {
			return err
		}
		fmt.Println("Disconnected from Docker environment. Returned to native mode.")
		return nil
	},
}

func init() {
	DockerCmd.AddCommand(stopCmd)
	DockerCmd.AddCommand(restartCmd)
	DockerCmd.AddCommand(statusCmd)
	DockerCmd.AddCommand(logsCmd)
	DockerCmd.AddCommand(shellCmd)

	DockerCmd.AddCommand(disconnectCmd)
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}

// getContainerStats retrieves uptime and restart count from docker inspect
func getContainerStats(ctx context.Context, containerName string) (time.Duration, int, error) {
	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{json .State}}", containerName)
	output, err := inspectCmd.Output()
	if err != nil {
		return 0, 0, err
	}

	var state struct {
		StartedAt    string `json:"StartedAt"`
		RestartCount int    `json:"RestartCount"`
	}

	if err := json.Unmarshal(output, &state); err != nil {
		return 0, 0, err
	}

	startedAt, err := time.Parse(time.RFC3339, state.StartedAt)
	if err != nil {
		return 0, 0, err
	}

	uptime := time.Since(startedAt)
	return uptime, state.RestartCount, nil
}

// formatUptime formats a duration into a human-readable string
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-float64(int(d.Minutes()))*60)
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%.0fh %.0fm", d.Hours(), d.Minutes()-float64(int(d.Hours()))*60)
	} else {
		return fmt.Sprintf("%.0fd %.0fh", d.Hours()/24, d.Hours()-float64(int(d.Hours()/24))*60)
	}
}
