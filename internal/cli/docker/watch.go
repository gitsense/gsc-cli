/**
 * Component: Docker CLI Watch
 * Block-UUID: 19eed7cd-658d-4d58-a37b-5634707b4837
 * Parent-UUID: 51e38bd2-a3e2-42c5-a2d8-c7880136ead5
 * Version: 1.1.0
 * Description: Implements the 'gsc docker watch' command, which tails container logs to listen for and execute host-side launch signals.
 * Language: Go
 * Created-at: 2026-03-21T04:14:12.064Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package docker

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
)

// watchCmd represents the docker watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch container logs for host-side launch signals",
	Long: `Starts a background listener that monitors the GitSense Chat container logs.
When the containerized app requests a terminal or editor launch, this command
translates the path and executes the native command on your host machine.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()

		// 1. Load Docker Context
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return fmt.Errorf("failed to load docker context: %w", err)
		}

		if dctx == nil {
			return fmt.Errorf("no active Docker context found. Run 'gsc docker start' first")
		}

		// 2. Verify Container is Running
		running, err := docker_internal.IsContainerRunning(ctx, dctx.ContainerName)
		if err != nil {
			return fmt.Errorf("failed to check container status: %w", err)
		}

		if !running {
			return fmt.Errorf("container '%s' is not running. Run 'gsc docker start' first", dctx.ContainerName)
		}

		// 3. Start the Watcher
		// This is a blocking call that tails the logs
		return docker_internal.WatchLogs(ctx, *dctx)
	},
}

func init() {
	DockerCmd.AddCommand(watchCmd)
}
