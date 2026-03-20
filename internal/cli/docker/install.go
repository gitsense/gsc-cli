/**
 * Component: Docker CLI Install
 * Block-UUID: 917b0dd3-707a-4031-9515-97ee541c8c45
 * Parent-UUID: 5bee63d9-f259-406b-803d-7eaac2ece383
 * Version: 1.1.0
 * Description: Implements the 'gsc docker install' command to verify Docker, create the isolated directory structure, and pull the latest image.
 * Language: Go
 * Created-at: 2026-03-20T16:08:48.581Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// installCmd represents the docker install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and configure the GitSense Chat Docker environment",
	Long: `Verifies the Docker installation, creates the necessary directory structure
for isolated data storage, and pulls the latest GitSense Chat Docker image.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Prerequisite Check: Docker CLI
		logger.Info("Verifying Docker installation...")
		if _, err := exec.LookPath("docker"); err != nil {
			return fmt.Errorf("docker CLI not found. Please install Docker: https://docs.docker.com/get-docker/")
		}

		// 2. Prerequisite Check: Docker Daemon
		logger.Info("Checking Docker daemon status...")
		daemonCheck := exec.Command("docker", "info")
		if err := daemonCheck.Run(); err != nil {
			return fmt.Errorf("docker daemon is not running. Please start Docker Desktop or your Docker service")
		}

		// 3. Directory Initialization
		logger.Info("Initializing directory structure...")
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		// Create the isolated Docker data directory
		dockerDataDir := filepath.Join(gscHome, settings.DockerDataDirRelPath)
		if err := os.MkdirAll(dockerDataDir, 0755); err != nil {
			return fmt.Errorf("failed to create Docker data directory at %s: %w", dockerDataDir, err)
		}

		// Create the isolated Docker repos sandbox directory
		dockerReposDir := filepath.Join(gscHome, settings.DockerReposDirRelPath)
		if err := os.MkdirAll(dockerReposDir, 0755); err != nil {
			return fmt.Errorf("failed to create Docker repos directory at %s: %w", dockerReposDir, err)
		}
		logger.Success("Docker directory structure initialized", "data", dockerDataDir, "repos", dockerReposDir)

		// 4. Image Acquisition
		image := settings.DefaultImageName
		logger.Info("Pulling latest Docker image...", "image", image)
		
		pullCmd := exec.Command("docker", "pull", image)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}

		// 5. Success Message
		fmt.Println("\nGitSense Chat Docker installation complete!")
		fmt.Printf("Image: %s\n", image)
		fmt.Printf("Data Directory: %s\n", dockerDataDir)
		fmt.Printf("Repos Sandbox:  %s\n", dockerReposDir)
		fmt.Println("\nTo start the application, run:")
		fmt.Println("  gsc docker start")
		fmt.Println("\n(Note: Use 'gsc docker configure --repos-dir' to use your own code directory.)")

		return nil
	},
}

func init() {
	DockerCmd.AddCommand(installCmd)
}
