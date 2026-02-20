/*
 * Component: Manifest Unpublish Command
 * Block-UUID: 30a95cd1-04e6-4faf-aac9-6116048f7f00
 * Parent-UUID: 334483c0-9856-4c0e-b453-813b4c0934ed
 * Version: 1.0.1
 * Description: Defines the 'gsc manifest unpublish' command, allowing users to remove published manifests from the GitSense Chat index. Suppresses usage output on error.
 * Language: Go
 * Created-at: 2026-02-19T18:29:28.554Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1)
 */


package manifest

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// unpublishCmd represents the unpublish command
var unpublishCmd = &cobra.Command{
	Use:   "unpublish [remote-id]",
	Short: "Remove a published manifest from the GitSense Chat app",
	Long: `Removes a manifest from the GitSense Chat index and deletes the associated 
file from storage. The UI is automatically regenerated to reflect the change.`,
	Args: cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Pre-flight check for GSC_HOME
		if _, err := settings.GetGSCHome(true); err != nil {
			return fmt.Errorf("environment error: %w", err)
		}

		remoteID := args[0]

		logger.Info("Unpublishing manifest...", "id", remoteID)

		// 2. Execute logic
		if err := manifest.Unpublish(remoteID); err != nil {
			return err
		}

		return nil
	},
}
