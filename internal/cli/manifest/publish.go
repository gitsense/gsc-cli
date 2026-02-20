/**
 * Component: Manifest Publish Command
 * Block-UUID: 3310da31-91b9-49c8-9c13-f01852b44165
 * Parent-UUID: bab7cf2f-a615-408f-9ea6-c105c8fd295b
 * Version: 1.0.2
 * Description: Defines the 'gsc manifest publish' command, allowing users to publish local manifests to a GitSense Chat installation. Suppresses usage output on error.
 * Language: Go
 * Created-at: 2026-02-20T00:40:50.147Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
 */


package manifest

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	pubOwner  string
	pubRepo   string
	pubBranch string
)

// publishCmd represents the publish command
var publishCmd = &cobra.Command{
	Use:   "publish [path-to-manifest.json]",
	Short: "Publish a manifest to the local GitSense Chat app",
	Long: `Publishes a manifest file to the GitSense Chat application defined by $GSC_HOME.
This command creates the necessary chat hierarchy (Root -> Owner -> Repo) and 
updates the index for user downloads.`,
	Args: cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Pre-flight check for GSC_HOME
		if manifestCode != "" {
			return fmt.Errorf("the --code flag is not yet supported for manifest commands. It will be available in a future release")
		}

		if _, err := settings.GetGSCHome(true); err != nil {
			return fmt.Errorf("environment error: %w", err)
		}

		manifestPath := args[0]

		logger.Info("Publishing manifest...", "path", manifestPath, "repo", pubOwner+"/"+pubRepo)

		// 2. Execute logic
		if err := manifest.Publish(manifestPath, pubOwner, pubRepo, pubBranch); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	publishCmd.Flags().StringVar(&pubOwner, "owner", "", "Repository owner (required)")
	publishCmd.Flags().StringVar(&pubRepo, "repo", "", "Repository name (required)")
	publishCmd.Flags().StringVar(&pubBranch, "branch", "", "Branch name (required)")

	publishCmd.MarkFlagRequired("owner")
	publishCmd.MarkFlagRequired("repo")
	publishCmd.MarkFlagRequired("branch")
}
