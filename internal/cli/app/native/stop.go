/**
 * Component: Native App CLI Stop
 * Block-UUID: 8118299f-7636-4564-b53b-708b014c495f
 * Parent-UUID: f7ee8d64-193d-4d6f-aa4c-73a16a81e4bc
 * Version: 1.5.0
 * Description: Implements the 'gsc app native stop' command to gracefully terminate the running application. Updated to support GSC_HOME environment variable and native-config.json fallback for data-dir resolution. Updated IsProcessRunning call to handle new signature with supervisor and child PIDs.
 * Language: Go
 * Created-at: 2026-05-12T14:10:08.285Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package native

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/native"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/pkg/logger"
	app_internal "github.com/gitsense/gsc-cli/internal/app"
)

var (
	stopDataDir string
)

// stopCmd represents the native app stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running GitSense Chat application",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		// 1. Resolve Data Directory
		dataDir := stopDataDir
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

		// 2. Stop the process
		running, supervisorPid, childPid, _ := app_internal.IsProcessRunning(dataDir)
		if !running {
			return fmt.Errorf("no process found running for data directory: %s", dataDir)
		}
		
		logger.Info("Stopping GitSense Chat...", "supervisor-pid", supervisorPid, "child-pid", childPid, "data-dir", dataDir)
		
		if err := app_internal.StopProcess(dataDir); err != nil {
			return err
		}

		logger.Success("Application stop signal sent successfully", "supervisor-pid", supervisorPid)
		return nil
	},
}

func init() {
	NativeCmd.AddCommand(stopCmd)

	stopCmd.Flags().StringVar(&stopDataDir, "data-dir", "", "Path to the persistent data directory (default: from GSC_HOME/data or native-config.json)")
}
