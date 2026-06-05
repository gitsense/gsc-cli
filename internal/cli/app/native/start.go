/**
 * Component: Native App CLI Start
 * Block-UUID: 3c132f97-3f84-4bbf-a733-a033a14cf232
 * Parent-UUID: bbe3f1d8-ed82-4c9c-bebf-cf5bea11040f
 * Version: 1.11.0
 * Description: Implements the 'gsc app native start' command, handling path resolution and invoking the process supervisor. Updated to support GSC_HOME environment variable and native-config.json fallback for app-dir and data-dir resolution. Updated IsProcessRunning call to handle new signature with supervisor and child PIDs. Refactored to use shared DisplayStartupMessage function for consistent enhanced startup messaging.
 * Language: Go
 * Created-at: 2026-05-30T17:14:56.863Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), Gemini 2.5 Flash Lite (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.10.1), GLM-4.7 (v1.10.2), GLM-4.7 (v1.11.0)
 */


package native

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/native"
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

// startCmd represents the native app start command
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

		// Verify index.js exists
		if _, err := os.Stat(filepath.Join(appDir, "index.js")); os.IsNotExist(err) {
			return fmt.Errorf("invalid app directory: index.js not found in %s", appDir)
		}

		// 2. Resolve Data Directory
		dataDir := startDataDir
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

		// Ensure data directory exists
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		// Set GSC_HOME environment variable if not already set
		// This ensures the CLI and child process have consistent GSC_HOME
		if os.Getenv("GSC_HOME") == "" {
			os.Setenv("GSC_HOME", appDir)
		}

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

		// 4. Initialize Supervisor Options
		opts := app_internal.SupervisorOptions{
			AppDir:      appDir,
			DataDir:     dataDir,
			Port:        startPort,
			Foreground:  startForeground,
			MaxRetries:  startMaxRetries,
		}

		running, _, childPid, _ := app_internal.IsProcessRunning(dataDir)
		if running {
			return fmt.Errorf("GitSense Chat is already running (PID %d); stop it first with: gsc app native stop", childPid)
		}

		logger.Info("Starting GitSense Chat...", "mode", map[bool]string{true: "foreground", false: "daemon"}[startForeground])
		if err := app_internal.StartSupervisor(opts); err != nil {
			return err
		}

		// 5. Enhanced startup verification and messaging (foreground mode only)
		if startForeground {
			return app_internal.DisplayStartupMessage(dataDir, startPort, false)
		}

		return nil
	},
}

func init() {
	NativeCmd.AddCommand(startCmd)

	startCmd.Flags().StringVar(&startAppDir, "app-dir", "", "Path to the Node.js application root (default: from GSC_HOME env var or native-config.json)")
	startCmd.Flags().StringVar(&startDataDir, "data-dir", "", "Path to the persistent data directory (default: from GSC_HOME/data or native-config.json)")
	startCmd.Flags().StringVar(&startEnvFile, "env-file", "", "Path to the .env file to load")
	startCmd.Flags().StringVar(&startPort, "port", settings.DefaultAppPort, "The port the application should listen on")
	startCmd.Flags().BoolVar(&startForeground, "foreground", false, "Run the application in the foreground instead of as a background daemon")
	startCmd.Flags().IntVar(&startMaxRetries, "max-retries", settings.AppMaxRetries, "Maximum number of restart attempts on crash")
}
