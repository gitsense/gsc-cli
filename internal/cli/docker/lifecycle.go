/**
 * Component: Docker CLI Lifecycle Commands
 * Block-UUID: f764cd78-4ca6-4a16-8e4e-ccc48471d690
 * Parent-UUID: 19b86155-ccda-4378-87c1-b587905b9f8d
 * Version: 1.6.0
 * Description: Implements standard Docker lifecycle commands (stop, status, logs, shell, admin) for the gsc CLI.
 * Language: Go
 * Created-at: 2026-03-21T04:11:47.473Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
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

		fmt.Printf("Container '%s' stopped. CLI remains in Docker mode.\n", name)
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
