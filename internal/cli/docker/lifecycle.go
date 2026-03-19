/**
 * Component: Docker CLI Lifecycle Commands
 * Block-UUID: 561354ee-e14f-44eb-9dd5-1d67b0c37899
 * Parent-UUID: 5e7afe59-be6a-4bcb-8c18-069a0ab3f300
 * Version: 1.2.0
 * Description: Implements standard Docker lifecycle commands (stop, status, logs, shell, admin) for the gsc CLI.
 * Language: Go
 * Created-at: 2026-03-19T19:08:25.452Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0)
 */


package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
)

var (
	logsFollow bool
)

// stopCmd represents the docker stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop and remove the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		name := "gitsense-chat"
		if dctx != nil {
			name = dctx.ContainerName
		}

		if err := docker_internal.StopContainer(ctx, name); err != nil {
			return err
		}

		// Cleanup context file to return to native mode
		if err := docker_internal.DeleteContext(); err != nil {
			return err
		}

		fmt.Printf("✅ Container '%s' stopped and removed. CLI returned to native mode.\n", name)
		return nil
	},
}

// restartCmd represents the docker restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		name := "gitsense-chat"
		if dctx != nil {
			name = dctx.ContainerName
		}

		fmt.Printf("🚀 Restarting container '%s'...\n", name)
		restartCmd := exec.CommandContext(ctx, "docker", "restart", name)
		restartCmd.Stdout = os.Stdout
		restartCmd.Stderr = os.Stderr

		if err := restartCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}

		fmt.Printf("✅ Container '%s' restarted successfully.\n", name)
		return nil
	},
}

// statusCmd represents the docker status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Printf("GitSense Chat Docker Status:\n")
		fmt.Printf("  Container: %s\n", dctx.ContainerName)
		fmt.Printf("  Status:    %s\n", status)
		fmt.Printf("  Port:      %s\n", dctx.Port)
		fmt.Printf("  Repos:     %s\n", dctx.ReposHostPath)
		fmt.Printf("  Data:      %s\n", dctx.DataHostPath)
		
		return nil
	},
}

// logsCmd represents the docker logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View the logs of the GitSense Chat container",
	RunE: func(cmd *cobra.Command, args []string) error {
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

func init() {
	DockerCmd.AddCommand(stopCmd)
	DockerCmd.AddCommand(restartCmd)
	DockerCmd.AddCommand(statusCmd)
	DockerCmd.AddCommand(logsCmd)
	DockerCmd.AddCommand(shellCmd)

	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}
