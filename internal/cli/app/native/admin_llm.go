/**
 * Component: Native App CLI Admin LLM
 * Block-UUID: bc9fc9c6-8939-4bf7-b867-83a0e1361787
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc app native admin llm' command as a wrapper around gsc-admin-llm, providing convenient LLM model and provider management.
 * Language: Go
 * Created-at: 2026-05-12T15:40:00.000Z
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

var (
	adminLLMDataDir string
)

// adminLLMCmd represents the admin llm command
var adminLLMCmd = &cobra.Command{
	Use:   "llm [command] [args...]",
	Short: "Manage LLM models and providers",
	Long: `Manages LLM models and providers in chat-config.json. This is a convenient
wrapper around gsc-admin-llm that automatically resolves paths and configuration.

Once GSC_HOME is set, you can manage your LLM settings from any location.`,
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

		// 2. Verify gsc-admin-llm exists
		llmScript := filepath.Join(appDir, "bin", "gsc-admin-llm")
		if _, err := os.Stat(llmScript); os.IsNotExist(err) {
			return fmt.Errorf("gsc-admin-llm not found at %s\nIs the app installed? Run 'gsc app native install' first.", llmScript)
		}

		// 3. Execute gsc-admin-llm with all arguments
		// We need to pass through all arguments including flags
		// The Node.js script will handle its own argument parsing
		nodeArgs := []string{llmScript}
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
			return fmt.Errorf("gsc-admin-llm exited with error")
		}

		return nil
	},
}

func init() {
	adminCmd.AddCommand(adminLLMCmd)
}
