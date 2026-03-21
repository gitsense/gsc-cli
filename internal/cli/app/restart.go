/**
 * Component: App CLI Restart
 * Block-UUID: a25fb846-b3e7-4327-957a-da2ed36c6072
 * Parent-UUID: 96d59355-8dfb-452d-aeb9-a1cd2159d44c
 * Version: 1.1.0
 * Description: Implements the 'gsc app restart' command to stop and then start the application.
 * Language: Go
 * Created-at: 2026-03-21T04:17:05.776Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package app

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/pkg/logger"
	app_internal "github.com/gitsense/gsc-cli/internal/app"
)

var (
	restartDataDir     string
	restartAppDir      string
	restartEnvFile     string
	restartPort        string
	restartForeground  bool
	restartMaxRetries  int
)

// restartCmd represents the app restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the GitSense Chat application",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		// 1. Resolve Data Directory
		dataDir := restartDataDir
		if dataDir == "" {
			gscHome, err := settings.GetGSCHome(false)
			if err != nil {
				return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
			}
			dataDir = filepath.Join(gscHome, settings.AppDataDirRelPath)
		}
		absDataDir, err := filepath.Abs(dataDir)
		if err != nil {
			return fmt.Errorf("failed to resolve data directory: %w", err)
		}
		dataDir = absDataDir

		// 2. Stop the process if running
		running, _, _ := app_internal.IsProcessRunning(dataDir)
		if running {
			logger.Info("Stopping GitSense Chat...", "data-dir", dataDir)
			if err := app_internal.StopProcess(dataDir); err != nil {
				return fmt.Errorf("failed to stop process: %w", err)
			}

			// Wait for it to stop (max 10 seconds)
			logger.Info("Waiting for process to terminate...")
			for i := 0; i < 10; i++ {
				time.Sleep(1 * time.Second)
				stillRunning, _, _ := app_internal.IsProcessRunning(dataDir)
				if !stillRunning {
					break
				}
			}
		}

		// 3. Start the process
		appDir := restartAppDir
		if appDir == "" {
			return fmt.Errorf("--app-dir is required to restart the application")
		}
		absAppDir, err := filepath.Abs(appDir)
		if err != nil {
			return fmt.Errorf("failed to resolve app directory: %w", err)
		}
		appDir = absAppDir

		opts := app_internal.SupervisorOptions{
			AppDir:      appDir,
			DataDir:     dataDir,
			EnvFile:     restartEnvFile,
			Port:        restartPort,
			Foreground:  restartForeground,
			MaxRetries:  restartMaxRetries,
		}

		logger.Info("Restarting GitSense Chat...", "mode", map[bool]string{true: "foreground", false: "daemon"}[restartForeground])
		return app_internal.StartSupervisor(opts)
	},
}

func init() {
	AppCmd.AddCommand(restartCmd)

	restartCmd.Flags().StringVar(&restartAppDir, "app-dir", "", "Path to the Node.js application root")
	restartCmd.Flags().StringVar(&restartDataDir, "data-dir", "", "Path to the persistent data directory")
	restartCmd.Flags().StringVar(&restartEnvFile, "env-file", "", "Path to the .env file to load")
	restartCmd.Flags().StringVar(&restartPort, "port", settings.DefaultAppPort, "The port the application should listen on")
	restartCmd.Flags().BoolVar(&restartForeground, "foreground", false, "Run the application in the foreground")
	restartCmd.Flags().IntVar(&restartMaxRetries, "max-retries", settings.AppMaxRetries, "Maximum number of restart attempts on crash")
}
