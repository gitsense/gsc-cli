/**
 * Component: Native App CLI Setup
 * Block-UUID: 6d7eecc7-45b3-4243-a14f-f97a5cf91ee6
 * Parent-UUID: 7f4fdc3c-a0b4-474f-bad2-9c71e15d267e
 * Version: 1.3.0
 * Description: Fixed variable scoping issues by directly assigning absolute paths to appDir and dataDir instead of using temporary abs variables.
 * Language: Go
 * Created-at: 2026-05-12T15:47:20.769Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1), GLM-4.7 (v1.2.2), Gemini 3 Flash (v1.2.3), GLM-4.7 (v1.3.0)
 */


package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/native"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

var (
	setupAppDir  string
	setupDataDir string
	setupForce   bool
	setupVerbose bool
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Reinitialize the GitSense Chat data directory",
	Long: `Runs gsc-admin-setup to reinitialize the data directory from the base-state.
This is useful for resetting the data directory to a clean state or recovering
from data corruption.

This command is hidden from help output as it's primarily an advanced/recovery tool.
The install command automatically handles initialization on fresh installs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Step 1: Resolve paths
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		// Load config to get defaults
		cfg, err := native.LoadConfig(gscHome)
		if err != nil {
			logger.Warning("Failed to load native config", "error", err)
		}
		if cfg == nil {
			cfg = &native.Config{}
		}

		// Resolve appDir: config → flag → default
		appDir := setupAppDir
		if appDir == "" {
			if cfg.AppDir != "" {
				appDir = cfg.AppDir
			} else {
				appDir = filepath.Join(gscHome, "active", "app")
			}
		} else {
			appDir, err = filepath.Abs(appDir)
			if err != nil {
				return fmt.Errorf("failed to resolve --app-dir: %w", err)
			}
		}

		// Resolve dataDir: config → flag → default
		dataDir := setupDataDir
		if dataDir == "" {
			if cfg.DataDir != "" {
				dataDir = cfg.DataDir
			} else {
				dataDir = filepath.Join(gscHome, settings.AppDataDirRelPath)
			}
		} else {
			dataDir, err = filepath.Abs(dataDir)
			if err != nil {
				return fmt.Errorf("failed to resolve --data-dir: %w", err)
			}
		}

		// Step 2: Verify gsc-admin-setup exists
		setupScript := filepath.Join(appDir, "bin", "gsc-admin-setup")
		if _, err := os.Stat(setupScript); os.IsNotExist(err) {
			return fmt.Errorf("gsc-admin-setup not found at %s\nIs the app installed? Run 'gsc app native install' first.", setupScript)
		}

		// Step 3: Build args
		nodeArgs := []string{setupScript, "--data-dir", dataDir}
		if setupForce {
			nodeArgs = append(nodeArgs, "--force")
		}
		if setupVerbose {
			nodeArgs = append(nodeArgs, "--verbose")
		}

		// Step 4: Run gsc-admin-setup
		logger.Info("Running gsc-admin-setup...", "data-dir", dataDir)
		nodeCmd := exec.Command("node", nodeArgs...)
		nodeCmd.Dir = appDir // Critical: ensures __dirname resolves correctly
		nodeCmd.Stdout = os.Stdout
		nodeCmd.Stderr = os.Stderr

		if err := nodeCmd.Run(); err != nil {
			return fmt.Errorf("gsc-admin-setup failed: %w", err)
		}

		logger.Success("Data directory reinitialized", "path", dataDir)
		return nil
	},
}

func init() {
	NativeCmd.AddCommand(setupCmd)

	setupCmd.Hidden = true // Hide from help output (advanced/recovery tool)

	setupCmd.Flags().StringVar(&setupAppDir, "app-dir", "", "Override the app directory (default: from native-config.json or $GSC_HOME/active/app)")
	setupCmd.Flags().StringVar(&setupDataDir, "data-dir", "", "Override the data directory (default: from native-config.json or $GSC_HOME/app/data)")
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "Force reinitialization (passed to gsc-admin-setup)")
	setupCmd.Flags().BoolVar(&setupVerbose, "verbose", false, "Enable verbose logging (passed to gsc-admin-setup)")
}
