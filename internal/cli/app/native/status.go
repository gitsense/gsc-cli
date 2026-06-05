/**
 * Component: Native App CLI Status
 * Block-UUID: ce6acb0f-a744-43cf-83b1-2f3a37d2c7c3
 * Parent-UUID: a08c3059-5173-4d4a-b49d-05b6f721adcd
 * Version: 1.5.1
 * Description: Implements the 'gsc app native status' command to display the current state of the application, including process information, crash history, uptime, and configuration details. Updated to only display the port when the application is running, as port information is only relevant for active connections.
 * Language: Go
 * Created-at: 2026-05-30T17:39:32.646Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1)
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
	statusDataDir string
)

// statusCmd represents the native app status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the GitSense Chat native application",
	Long: `Displays the current status of the GitSense Chat native application,
including process information, crash history, uptime, and configuration details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Data Directory
		dataDir := statusDataDir
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

		// 2. Check if process is running
		running, supervisorPid, childPid, err := app_internal.IsProcessRunning(dataDir)
		if err != nil {
			return fmt.Errorf("failed to check process status: %w", err)
		}

		// 3. Load health status
		health, err := app_internal.LoadHealthStatus(dataDir)
		if err != nil {
			logger.Warning("Failed to load health status", "error", err)
		}

		// 4. Display status
		fmt.Println("\n" + "━" + strings.Repeat("━", 58))
		fmt.Println("  GitSense Chat Native Status")
		fmt.Println("━" + strings.Repeat("━", 58))
		
		status := "Stopped"
		if running {
			status = "Running"
		}
		fmt.Printf("  Status:         %s\n", status)
		
		if running {
			fmt.Printf("  PID:            %d\n", childPid)
			fmt.Printf("  Supervisor PID: %d\n", supervisorPid)
		}
		
		if health != nil {
			if running {
				// Update uptime before displaying
				app_internal.UpdateUptime(health)
				fmt.Printf("  Uptime:         %s\n", app_internal.FormatUptime(time.Duration(health.UptimeSeconds)*time.Second))
				
				// Display port from health status (actual running port)
				if health.Port != "" {
					fmt.Printf("  Port:           %s\n", health.Port)
				}
			}
			
			if health.CrashCount > 0 {
				fmt.Printf("  Crashes:        %d\n", health.CrashCount)
				if health.LastCrashAt != nil {
					timeSinceCrash := time.Since(*health.LastCrashAt)
					fmt.Printf("  Last Crash:     %s (%s ago)\n",
						health.LastCrashAt.Format("2006-01-02 15:04:05 MST"),
						app_internal.FormatUptime(timeSinceCrash))
				}
			}
			
			if health.RestartCount > 0 {
				fmt.Printf("  Restarts:       %d\n", health.RestartCount)
			}
		}
		
		fmt.Printf("  Data Dir:       %s\n", dataDir)
		
		// Check .env file status
		envPath := filepath.Join(dataDir, ".env")
		envStatus := "✓ [PRESENT]"
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			envStatus = "✗ [MISSING]"
		}
		fmt.Printf("  Env File:       %s %s\n", envPath, envStatus)
		
		fmt.Printf("  Log File:       %s\n", filepath.Join(dataDir, "logs", "app.log"))
		
		// 5. Try to load config for additional info (App Dir and Version)
		gscHome, _ := settings.GetGSCHome(false)
		if cfg, err := native.LoadConfig(gscHome); err == nil && cfg != nil {
			fmt.Printf("  App Dir:        %s\n", cfg.AppDir)
			fmt.Printf("  Version:        %s\n", cfg.Version)
		}
		
		fmt.Println("━" + strings.Repeat("━", 58))
		
		return nil
	},
}

func init() {
	NativeCmd.AddCommand(statusCmd)

	statusCmd.Flags().StringVar(&statusDataDir, "data-dir", "", "Override the data directory (default: from GSC_HOME/data or native-config.json)")
}
