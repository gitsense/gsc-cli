/*
 * Component: Docker CLI Configure
 * Block-UUID: 29a83bd9-0c4a-4cf8-8efa-4cf57fdb84bc
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc docker configure' command to allow users to update their Docker context settings (like repos-dir) without restarting or using complex flags.
 * Language: Go
 * Created-at: 2026-03-20T16:18:05.000Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	configReposDir string
	configDataDir  string
)

// configureCmd represents the docker configure command
var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Update the GitSense Chat Docker configuration",
	Long: `Allows you to update the host-side paths for repositories and data
stored in your Docker context. Changes will take effect the next time
the container is started or when commands are proxied.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return fmt.Errorf("failed to load docker context: %w", err)
		}

		if dctx == nil {
			return fmt.Errorf("no active Docker context found. Run 'gsc docker start' first to initialize the environment")
		}

		updated := false

		// 1. Update Repos Directory
		if configReposDir != "" {
			absPath, err := filepath.Abs(configReposDir)
			if err != nil {
				return fmt.Errorf("invalid repos directory path: %w", err)
			}
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				return fmt.Errorf("repos directory does not exist: %s", absPath)
			}
			dctx.ReposHostPath = absPath
			updated = true
			logger.Success("Updated repository umbrella directory", "path", absPath)
		}

		// 2. Update Data Directory
		if configDataDir != "" {
			absPath, err := filepath.Abs(configDataDir)
			if err != nil {
				return fmt.Errorf("invalid data directory path: %w", err)
			}
			dctx.DataHostPath = absPath
			updated = true
			logger.Success("Updated data directory", "path", absPath)
		}

		if !updated {
			fmt.Println("No configuration changes specified. Use --repos-dir or --data-dir to update settings.")
			return nil
		}

		// 3. Save Context
		if err := docker_internal.SaveContext(*dctx); err != nil {
			return fmt.Errorf("failed to save updated docker context: %w", err)
		}

		fmt.Println("\nConfiguration updated successfully.")
		fmt.Println("Note: If the container is currently running, you must restart it for volume changes to take effect:")
		fmt.Println("   gsc docker restart")

		return nil
	},
}

func init() {
	DockerCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVarP(&configReposDir, "repos-dir", "r", "", "Update the host path to your Git repositories")
	configureCmd.Flags().StringVarP(&configDataDir, "data-dir", "d", "", "Update the host path for persistent data")
}
