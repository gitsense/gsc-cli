/*
 * Component: App CLI Stop
 * Block-UUID: 234dc2a4-caa4-4aa5-9cc4-d13a0f3e81d1
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc app stop' command to gracefully terminate the running application.
 * Language: Go
 * Created-at: 2026-03-20T23:55:33.666Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package app

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/pkg/logger"
	app_internal "github.com/gitsense/gsc-cli/internal/app"
)

var (
	stopDataDir string
)

// stopCmd represents the app stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running GitSense Chat application",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Resolve Data Directory
		dataDir := stopDataDir
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

		// 2. Stop the process
		logger.Info("Stopping GitSense Chat...", "data-dir", dataDir)
		if err := app_internal.StopProcess(dataDir); err != nil {
			return err
		}

		logger.Success("Application stop signal sent successfully")
		return nil
	},
}

func init() {
	AppCmd.AddCommand(stopCmd)

	stopCmd.Flags().StringVar(&stopDataDir, "data-dir", "", "Path to the persistent data directory")
}
