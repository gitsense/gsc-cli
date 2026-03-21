/**
 * Component: App CLI Start
 * Block-UUID: a04e7939-fadb-4dc5-9130-d0c0de2684a0
 * Parent-UUID: b5effeb2-7d7d-4c99-b980-b3dbc999aa8d
 * Version: 1.1.0
 * Description: Implements the 'gsc app start' command, handling path resolution and invoking the process supervisor.
 * Language: Go
 * Created-at: 2026-03-21T04:16:03.510Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/pkg/logger"
	app_internal "github.com/gitsense/gsc-cli/internal/app"
)

var (
	startAppDir      string
	startDataDir     string
	startEnvFile     string
	startPort        string
	startForeground  bool
	startMaxRetries  int
)

// startCmd represents the app start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the native GitSense Chat application",
	Long: `Starts the GitSense Chat Node.js application. By default, it runs 
in the background as a daemon. Use --foreground for Docker environments.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Resolve App Directory (where index.js lives)
		cmd.SilenceUsage = true // Suppress usage output on error

		appDir := startAppDir
		if appDir == "" {
			return fmt.Errorf("--app-dir is required to specify the Node.js application root")
		}
		absAppDir, err := filepath.Abs(appDir)
		if err != nil {
			return fmt.Errorf("failed to resolve app directory: %w", err)
		}
		appDir = absAppDir

		// Verify index.js exists
		if _, err := os.Stat(filepath.Join(appDir, "index.js")); os.IsNotExist(err) {
			return fmt.Errorf("invalid app directory: index.js not found in %s", appDir)
		}

		// 2. Resolve Data Directory
		dataDir := startDataDir
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

		// Ensure data directory exists
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// 3. Resolve Env File
		envFile := startEnvFile
		if envFile != "" {
			absEnv, err := filepath.Abs(envFile)
			if err != nil {
				return fmt.Errorf("failed to resolve env file path: %w", err)
			}
			envFile = absEnv
		}

		// 4. Initialize Supervisor Options
		opts := app_internal.SupervisorOptions{
			AppDir:      appDir,
			DataDir:     dataDir,
			EnvFile:     envFile,
			Port:        startPort,
			Foreground:  startForeground,
			MaxRetries:  startMaxRetries,
		}

		logger.Info("Starting GitSense Chat...", "mode", map[bool]string{true: "foreground", false: "daemon"}[startForeground])
		return app_internal.StartSupervisor(opts)
	},
}

func init() {
	AppCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&startAppDir, "app-dir", "", "Path to the Node.js application root")
	startCmd.Flags().StringVar(&startDataDir, "data-dir", "", "Path to the persistent data directory")
	startCmd.Flags().StringVar(&startEnvFile, "env-file", "", "Path to the .env file to load")
	startCmd.Flags().StringVar(&startPort, "port", settings.DefaultAppPort, "The port the application should listen on")
	startCmd.Flags().BoolVar(&startForeground, "foreground", false, "Run the application in the foreground (required for Docker)")
	startCmd.Flags().IntVar(&startMaxRetries, "max-retries", settings.AppMaxRetries, "Maximum number of restart attempts on crash")
}
