/**
 * Component: Docker CLI Configure
 * Block-UUID: 6c3ae6dc-2266-4a55-a772-89f0b58bf988
 * Parent-UUID: e51fed74-bbd4-4e87-9b9b-54299bbb8e6e
 * Version: 1.2.0
 * Description: Implements the 'gsc docker configure' command to allow users to update their Docker context settings (like repos-dir) without restarting or using complex flags.
 * Language: Go
 * Created-at: 2026-03-21T04:12:34.494Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
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
		cmd.SilenceUsage = true // Suppress usage output on error

		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return fmt.Errorf("failed to load docker context: %w", err)
		}

		// If no context exists, create a default one to allow pre-start configuration
		if dctx == nil {
			gscHome, err := settings.GetGSCHome(false)
			if err != nil {
				return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
			}
			
			logger.Info("No active context found. Initializing default configuration...")
			dctx = &docker_internal.DockerContext{
				ContainerName:      settings.DefaultContainerName,
				ReposHostPath:      "", // Will be set by flags below if provided
				ReposContainerPath: filepath.Join(settings.DockerRootPrefix, "repos"),
				DataHostPath:       filepath.Join(gscHome, settings.DockerDataDirRelPath),
				EnvHostPath:        "",
				Port:               settings.DefaultAppPort,
			}
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

		fmt.Println("\nConfiguration updated successfully.\n")
		fmt.Println("Note: If the container is currently running, you must restart it for volume changes to take effect.")
		fmt.Println("   gsc docker restart")

		return nil
	},
}

func init() {
	DockerCmd.AddCommand(configureCmd)

	configureCmd.Flags().StringVarP(&configReposDir, "repos-dir", "r", "", "Update the host path to your Git repositories")
	configureCmd.Flags().StringVarP(&configDataDir, "data-dir", "d", "", "Update the host path for persistent data")
}
