/**
 * Component: Docker CLI Lifecycle Commands
 * Block-UUID: f3b42461-cf37-46ce-83b1-561b64256eb8
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements standard Docker lifecycle commands (stop, status, logs, shell, admin) for the gsc CLI.
 * Language: Go
 * Created-at: 2026-03-19T01:56:34.675Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package docker

import (
	"context"
	"fmt"
	"os"

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

// adminCmd represents the docker admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Run the gsc-admin tool inside the container",
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

		// Pass all arguments to gsc-admin
		adminArgs := append([]string{"gsc-admin"}, args...)
		return docker_internal.ExecCommand(ctx, name, adminArgs, true, "")
	},
}

func init() {
	DockerCmd.AddCommand(stopCmd)
	DockerCmd.AddCommand(statusCmd)
	DockerCmd.AddCommand(logsCmd)
	DockerCmd.AddCommand(shellCmd)
	DockerCmd.AddCommand(adminCmd)

	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}
