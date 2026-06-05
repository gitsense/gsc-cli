/*
 * Component: GitIgnore Update Command
 * Block-UUID: 2bbf32ff-ab05-49bc-ab51-c1848a00de6d
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command to regenerate the .gitsense/.gitignore file with all known patterns from various GitSense features.
 * Language: Go
 * Created-at: 2026-06-04T12:58:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package gitignore

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/gitignore"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// NewUpdateCommand creates the gitignore update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Regenerate the .gitsense/.gitignore file",
		Long: `Regenerate the .gitsense/.gitignore file with all known patterns from various GitSense features.
This ensures that state files, generated artifacts, and temporary files are not tracked by git.

The .gitignore file is programmatically generated and should not be edited manually.
If you need to add custom patterns, create a separate .gitignore file in the repository root.`,
		RunE: runUpdate,
	}

	return cmd
}

// runUpdate executes the gitignore update command
func runUpdate(cmd *cobra.Command, args []string) error {
	// Resolve project root
	projectRoot, err := git.FindProjectRoot()
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("GitSense Chat can only be used within a Git repository: %w", err)
	}

	gitsenseDir := fmt.Sprintf("%s/%s", projectRoot, settings.GitSenseDir)

	// Check if .gitsense directory exists
	if _, err := os.Stat(gitsenseDir); os.IsNotExist(err) {
		cmd.SilenceUsage = true
		return fmt.Errorf("GitSense workspace not found at %s. Please run 'gsc manifest init' first", gitsenseDir)
	}

	// Regenerate .gitignore
	logger.Info("Regenerating .gitsense/.gitignore...")
	if err := gitignore.Regenerate(gitsenseDir); err != nil {
		return fmt.Errorf("failed to regenerate .gitignore: %w", err)
	}

	logger.Success(".gitsense/.gitignore updated successfully")
	return nil
}
