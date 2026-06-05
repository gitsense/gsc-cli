/**
 * Component: Native App CLI Restart
 * Block-UUID: 772304e7-18ff-47bd-aa30-1b1e0a587788
 * Parent-UUID: 1ca0cbee-03eb-49b5-8b91-63488dff9854
 * Version: 1.7.0
 * Description: Implements the 'gsc app native restart' command to stop and then start the application. Updated to support GSC_HOME environment variable and native-config.json fallback for app-dir and data-dir resolution. Updated IsProcessRunning call to handle new signature with supervisor and child PIDs. Refactored to use shared DisplayStartupMessage function for consistent enhanced startup messaging.
 * Language: Go
 * Created-at: 2026-05-30T17:15:24.302Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.6.1), GLM-4.7 (v1.7.0)
 */


package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/native"
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

// restartCmd represents the native app restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the GitSense Chat application",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		// 1. Resolve Data Directory
		dataDir := restartDataDir
		if dataDir == "" {
			// Priority 1: GSC_HOME/data (if GSC_HOME is set)
			if gscHomeEnv := os.Getenv("GSC_HOME"); gscHomeEnv != "" {
				dataDir = filepath.Join(gscHomeEnv, "data")
			} else {
				// Priority 2: native-config.json
				gscHome, err := settings.GetGSCHome(false)
				if err != nil {
					return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
				}
				cfg, err := native.LoadConfig(gscHome)
				if err != nil {
					return fmt.Errorf("failed to load native config: %w", err)
				}
				if cfg != nil && cfg.DataDir != "" {
					dataDir = cfg.DataDir
				} else {
					// Priority 3: Default
					dataDir = filepath.Join(gscHome, settings.AppDataDirRelPath)
				}
			}
		}
		absDataDir, err := filepath.Abs(dataDir)
		if err != nil {
			return fmt.Errorf("failed to resolve data directory: %w", err)
		}
		dataDir = absDataDir

		// 2. Stop the process if running
		running, _, _, _ := app_internal.IsProcessRunning(dataDir)
		if running {
			logger.Info("Stopping GitSense Chat...", "data-dir", dataDir)
			if err := app_internal.StopProcess(dataDir); err != nil {
				return fmt.Errorf("failed to stop process: %w", err)
			}

			// Wait for it to stop (max 10 seconds)
			logger.Info("Waiting for process to terminate...")
			for i := 0; i < 10; i++ {
				time.Sleep(1 * time.Second)
				stillRunning, _, _, _ := app_internal.IsProcessRunning(dataDir)
				if !stillRunning {
					break
				}
			}
		}

		// 3. Resolve App Directory
		appDir := restartAppDir
		if appDir == "" {
			// Priority 1: GSC_HOME environment variable
			if gscHomeEnv := os.Getenv("GSC_HOME"); gscHomeEnv != "" {
				appDir = gscHomeEnv
			} else {
				// Priority 2: native-config.json
				gscHome, err := settings.GetGSCHome(false)
				if err != nil {
					return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
				}
				cfg, err := native.LoadConfig(gscHome)
				if err != nil {
					return fmt.Errorf("failed to load native config: %w", err)
				}
				if cfg != nil && cfg.AppDir != "" {
					appDir = cfg.AppDir
				} else {
					// Priority 3: Error
					return fmt.Errorf("--app-dir is required (or set GSC_HOME environment variable)")
				}
			}
		}
		absAppDir, err := filepath.Abs(appDir)
		if err != nil {
			return fmt.Errorf("failed to resolve app directory: %w", err)
		}
		appDir = absAppDir

		// Check if .env file exists and warn if missing
		envPath := filepath.Join(dataDir, ".env")
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			fmt.Println("") // Add blank line before warning
			fmt.Println(strings.Repeat("━", 60))
			fmt.Println("  ⚠️  WARNING: .env file not found")
			fmt.Println(strings.Repeat("━", 60))
			fmt.Printf("  Location: %s\n\n", envPath)
			fmt.Println("  The application may not function correctly without proper configuration.\n")
			fmt.Println("  To fix this:")
			fmt.Println("    1. Copy the example file:")
			fmt.Printf("       cp %s %s\n", filepath.Join(appDir, ".env.example"), envPath)
			fmt.Println("")
			fmt.Println("    2. Edit the file and add your API keys:")
			fmt.Printf("       nano %s\n", envPath)
			fmt.Println("")
			fmt.Println("  For LLM model and provider management, use:")
			fmt.Println("    gsc app native admin llm")
			fmt.Println("")
			fmt.Println("  Note: If you create or modify the .env file while the")
			fmt.Println("        application is running, you will need to restart")
			fmt.Println("        the application for the changes to take effect:")
			fmt.Println("        gsc app native restart")
			fmt.Println(strings.Repeat("━", 60))
			fmt.Println("")
		}

		// 4. Start the process
		opts := app_internal.SupervisorOptions{
			AppDir:      appDir,
			DataDir:     dataDir,
			EnvFile:     restartEnvFile,
			Port:        restartPort,
			Foreground:  restartForeground,
			MaxRetries:  restartMaxRetries,
		}

		logger.Info("Restarting GitSense Chat...", "mode", map[bool]string{true: "foreground", false: "daemon"}[restartForeground])
		if err := app_internal.StartSupervisor(opts); err != nil {
			return err
		}

		// 5. Enhanced startup verification and messaging (foreground mode only)
		if restartForeground {
			return app_internal.DisplayStartupMessage(dataDir, restartPort, true)
		}

		return nil
	},
}

func init() {
	NativeCmd.AddCommand(restartCmd)

	restartCmd.Flags().StringVar(&restartAppDir, "app-dir", "", "Path to the Node.js application root (default: from GSC_HOME env var or native-config.json)")
	restartCmd.Flags().StringVar(&restartDataDir, "data-dir", "", "Path to the persistent data directory (default: from GSC_HOME/data or native-config.json)")
	restartCmd.Flags().StringVar(&restartEnvFile, "env-file", "", "Path to the .env file to load")
	restartCmd.Flags().StringVar(&restartPort, "port", settings.DefaultAppPort, "The port the application should listen on")
	restartCmd.Flags().BoolVar(&restartForeground, "foreground", false, "Run the application in the foreground")
	restartCmd.Flags().IntVar(&restartMaxRetries, "max-retries", settings.AppMaxRetries, "Maximum number of restart attempts on crash")
}
