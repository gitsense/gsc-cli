/*
 * Component: Manifest Unpublish Command
 * Block-UUID: 8c4b0826-97f2-4b6b-bdd1-0395893f0c6c
 * Parent-UUID: 30a95cd1-04e6-4faf-aac9-6116048f7f00
 * Version: 1.0.2
 * Description: Defines the 'gsc manifest unpublish' command, allowing users to remove published manifests from the GitSense Chat index. Suppresses usage output on error.
 * Language: Go
 * Created-at: 2026-02-19T18:29:28.554Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2)
 */


package manifest

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	unpubOwner string
	unpubRepo  string
	unpubForce bool
)

// unpublishCmd represents the unpublish command
var unpublishCmd = &cobra.Command{
	Use:   "unpublish [remote-id]",
	Short: "Remove published manifests from the GitSense Chat app",
	Long: `Removes manifests from the GitSense Chat index and deletes associated files from storage.

Three deletion scopes are supported:
  gsc manifest unpublish <uuid>                          - Delete a single manifest by UUID
  gsc manifest unpublish --owner <owner>                 - Delete all manifests for an owner (all repos)
  gsc manifest unpublish --owner <owner> --repo <repo>   - Delete all manifests for a specific repo`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if manifestCode != "" {
			return fmt.Errorf("the --code flag is not yet supported for manifest commands. It will be available in a future release")
		}

		if _, err := settings.GetGSCHome(false); err != nil {
			return fmt.Errorf("environment error: %w", err)
		}

		if unpubOwner != "" {
			if unpubRepo != "" {
				if !unpubForce {
					if err := confirmOwnerRepoDelete(unpubOwner, unpubRepo); err != nil {
						return err
					}
				}
				return manifest.DeleteByOwnerRepo(unpubOwner, unpubRepo)
			}
			if !unpubForce {
				if err := confirmOwnerDelete(unpubOwner); err != nil {
					return err
				}
			}
			return manifest.DeleteByOwner(unpubOwner)
		}

		if len(args) == 0 {
			return fmt.Errorf("requires either --owner flag or a manifest UUID argument")
		}

		remoteID := args[0]
		logger.Info("Unpublishing manifest...", "id", remoteID)

		return manifest.Unpublish(remoteID)
	},
}

func confirmOwnerRepoDelete(owner, repo string) error {
	total, groups, err := manifest.SummarizeOwnerRepoDelete(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to load manifest summary: %w", err)
	}
	if total == 0 {
		return fmt.Errorf("no published manifests found for %s/%s", owner, repo)
	}

	fmt.Printf("This will permanently unpublish %d manifest(s) for %s/%s:\n", total, owner, repo)
	for _, g := range groups {
		fmt.Printf("  - %s (%d version(s))\n", g.Name, g.Count)
	}
	fmt.Println()

	return promptConfirm()
}

func confirmOwnerDelete(owner string) error {
	total, repos, err := manifest.SummarizeOwnerDelete(owner)
	if err != nil {
		return fmt.Errorf("failed to load manifest summary: %w", err)
	}
	if total == 0 {
		return fmt.Errorf("no published manifests found for owner %s", owner)
	}

	fmt.Printf("This will permanently unpublish %d manifest(s) across %d repo(s) for %s:\n", total, len(repos), owner)
	for _, r := range repos {
		fmt.Printf("  - %s/%s (%d manifest(s))\n", owner, r.Repo, r.Count)
	}
	fmt.Println()

	return promptConfirm()
}

func promptConfirm() error {
	fmt.Print("Proceed? [y/N]: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if answer != "y" && answer != "yes" {
		return fmt.Errorf("aborted")
	}
	return nil
}

func init() {
	unpublishCmd.Flags().StringVar(&unpubOwner, "owner", "", "Repository owner (required for scope delete)")
	unpublishCmd.Flags().StringVar(&unpubRepo, "repo", "", "Repository name (optional, combined with --owner)")
	unpublishCmd.Flags().BoolVar(&unpubForce, "force", false, "Skip confirmation prompt")
}
