/**
 * Component: Native App CLI Admin Environment
 * Block-UUID: 2c66bb58-3599-4c00-b625-4028da93a4b0
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc app native admin env' command as a wrapper around gsc-admin-env, providing convenient environment variable management.
 * Language: Go
 * Created-at: 2026-05-31T02:40:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/native"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// adminEnvCmd represents the admin env command
var adminEnvCmd = &cobra.Command{
	Use:   "env [command] [args...]",
	Short: "Manage environment variables (.env file)",
	Long: `Manages environment variables in the .env file. This is a convenient
wrapper around gsc-admin-env that automatically resolves paths and configuration.

Once GSC_HOME is set, you can manage your environment variables from any location.`,
	DisableFlagParsing: true, // Pass all flags to the Node.js script
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve App Directory
		appDir := ""
		if gscHomeEnv := os.Getenv("GSC_HOME"); gscHomeEnv != "" {
			appDir = gscHomeEnv
		} else {
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
				return fmt.Errorf("app directory not found. Run 'gsc app native install' first or set GSC_HOME environment variable")
			}
		}
		absAppDir, err := filepath.Abs(appDir)
		if err != nil {
			return fmt.Errorf("failed to resolve app directory: %w", err)
		}
		appDir = absAppDir

		// 2. Verify gsc-admin-env exists
		envScript := filepath.Join(appDir, "bin", "gsc-admin-env.js")
		if _, err := os.Stat(envScript); os.IsNotExist(err) {
			return fmt.Errorf("gsc-admin-env not found at %s\nIs the app installed? Run 'gsc app native install' first.", envScript)
		}

		// 3. Execute gsc-admin-env with all arguments
		// We need to pass through all arguments including flags
		// The Node.js script will handle its own argument parsing
		nodeArgs := []string{envScript}
		nodeArgs = append(nodeArgs, args...)

		nodeCmd := exec.Command("node", nodeArgs...)
		nodeCmd.Dir = appDir
		nodeCmd.Stdout = os.Stdout
		nodeCmd.Stderr = os.Stderr
		nodeCmd.Stdin = os.Stdin

		// Set environment variables
		nodeCmd.Env = os.Environ()
		nodeCmd.Env = append(nodeCmd.Env, fmt.Sprintf("GSC_HOME=%s", appDir))

		if err := nodeCmd.Run(); err != nil {
			// If the command failed, return the error
			// The Node.js script will have already printed its own error messages
			return fmt.Errorf("gsc-admin-env exited with error")
		}

		return nil
	},
}

func init() {
	adminCmd.AddCommand(adminEnvCmd)
}
