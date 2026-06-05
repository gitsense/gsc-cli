/*
 * Component: Manifest Init Command
 * Block-UUID: 75661573-7c4a-4586-98cc-85893551558a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for 'gsc manifest init'.
 * Language: Go
 * Created-at: 2026-02-02T05:30:00Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the .gitsense directory structure",
	Long: `Initialize the .gitsense directory structure in the current project root.
This creates the necessary folders and the manifest.json registry file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Initializing GitSense...")
		if err := manifest.InitializeGitSense(); err != nil {
			logger.Error(err.Error())
			return err
		}
		return nil
	},
}

func init() {
	// Register this command with the parent manifest command
	// Note: The parent 'manifest' command is defined in root.go
	// This init function runs automatically when the package is imported
}
