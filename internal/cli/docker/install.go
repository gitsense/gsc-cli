/**
 * Component: Docker CLI Install
 * Block-UUID: 44efbe87-ea3e-4c6e-8fe7-34658039b94d
 * Parent-UUID: 917b0dd3-707a-4031-9515-97ee541c8c45
 * Version: 1.2.0
 * Description: Implements the 'gsc docker install' command to verify Docker, create the isolated directory structure, and pull the latest image.
 * Language: Go
 * Created-at: 2026-03-20T22:11:29.926Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0)
 */


package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/AlecAivazis/survey/v2"

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

		// 3. Path Resolution
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		// Define the isolated Docker data directory
		dockerDataDir := filepath.Join(gscHome, settings.DockerDataDirRelPath)

		// Define the isolated Docker repos sandbox directory
		dockerReposDir := filepath.Join(gscHome, settings.DockerReposDirRelPath)

		// 4. Installation Summary & Confirmation
		image := settings.DefaultImageName
		fmt.Println("\nGitSense Chat Docker Installation Summary")
		fmt.Println("------------------------------------------")
		fmt.Println("This command will configure your local environment for GitSense Chat.")
		fmt.Println("")
		fmt.Println("Actions to be performed:")
		fmt.Printf("  1. Pull Docker Image: %s\n", image)
		fmt.Printf("  2. Create Data Directory: %s\n", dockerDataDir)
		fmt.Printf("  3. Create Repos Directory: %s\n", dockerReposDir)
		fmt.Println("")

		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with installation?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return fmt.Errorf("failed to prompt for confirmation: %w", err)
		}
		if !confirm {
			fmt.Println("Installation cancelled.")
			return nil
		}

		// 5. Directory Initialization
		logger.Info("Initializing directory structure...")
		if err := os.MkdirAll(dockerDataDir, 0755); err != nil {
			return fmt.Errorf("failed to create Docker data directory at %s: %w", dockerDataDir, err)
		}

		if err := os.MkdirAll(dockerReposDir, 0755); err != nil {
			return fmt.Errorf("failed to create Docker repos directory at %s: %w", dockerReposDir, err)
		}
		logger.Success("Docker directory structure initialized", "data", dockerDataDir, "repos", dockerReposDir)

		// 6. Image Acquisition
		logger.Info("Pulling latest Docker image...", "image", image)
		
		pullCmd := exec.Command("docker", "pull", image)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}

		// 7. Success Message
		fmt.Println("\nGitSense Chat Docker installation complete!")
		fmt.Println("")
		fmt.Println("Configuration:")
		fmt.Printf("  Image: %s\n", image)
		fmt.Printf("  Data:  %s\n", dockerDataDir)
		fmt.Printf("  Repos: %s\n", dockerReposDir)
		fmt.Println("")
		fmt.Println("Next Steps:")
		fmt.Println("  1. Start the application:")
		fmt.Println("     gsc docker start")
		fmt.Println("")
		fmt.Println("  2. (Optional) Point to your existing code:")
		fmt.Println("     gsc docker configure --repos-dir ~/path/to/your/projects")

		return nil
	},
}

func init() {
	DockerCmd.AddCommand(installCmd)
}
