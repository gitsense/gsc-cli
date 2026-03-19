/**
 * Component: Docker CLI Start
 * Block-UUID: 2a8fa566-69a5-4db4-b8b3-ce45dff61e2c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc docker start' command, handling flags, context file creation, and container initialization.
 * Language: Go
 * Created-at: 2026-03-19T01:54:15.123Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	startReposDir string
	startDataDir  string
	startPort     string
	startEnvFile  string
	startName     string
	startImage    string
	startPull     bool
)

// startCmd represents the docker start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the GitSense Chat container",
	Long: `Start the GitSense Chat container. This command initializes the Docker context,
mounts the specified repository directory, and launches the application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// 1. Resolve Repos Directory
		reposDir := startReposDir
		if reposDir == "" {
			reposDir = os.Getenv("GSC_REPOS_DIR")
		}

		// Warning if no repos dir provided
		if reposDir == "" {
			fmt.Println("⚠️  WARNING: No repository directory specified.")
			fmt.Println("   GitSense Chat will start, but the following features will be DISABLED:")
			fmt.Println("     - Creating Traceability Contracts")
			fmt.Println("     - Local file analysis and indexing")
			fmt.Println("     - Saving AI-generated code to your host machine")
			fmt.Println("")
			fmt.Printf("   To enable these features, restart with: gsc docker start --repos-dir /path/to/repos\n\n")
		} else {
			// Validate path exists
			absReposDir, err := filepath.Abs(reposDir)
			if err != nil {
				return fmt.Errorf("failed to resolve repos directory: %w", err)
			}
			if _, err := os.Stat(absReposDir); os.IsNotExist(err) {
				return fmt.Errorf("repository directory does not exist: %s", absReposDir)
			}
			reposDir = absReposDir
		}

		// 2. Resolve Data Directory
		dataDir := startDataDir
		if dataDir == "" {
			// Default to named volume if not specified
			dataDir = ""
		} else {
			absDataDir, err := filepath.Abs(dataDir)
			if err != nil {
				return fmt.Errorf("failed to resolve data directory: %w", err)
			}
			dataDir = absDataDir
		}

		// 3. Resolve Env File
		envFile := startEnvFile
		if envFile != "" {
			if _, err := os.Stat(envFile); os.IsNotExist(err) {
				return fmt.Errorf("env file not found: %s", envFile)
			}
		}

		// 4. Resolve Container Name and Image
		containerName := startName
		if containerName == "" {
			containerName = settings.DefaultContainerName
		}

		image := startImage
		if image == "" {
			image = settings.DefaultImageName
		}

		port := startPort
		if port == "" {
			port = settings.DefaultAppPort
		}

		// 5. Create Docker Context
		dctx := docker.DockerContext{
			ContainerName:      containerName,
			ReposHostPath:      reposDir,
			ReposContainerPath: filepath.Join(settings.DockerRootPrefix, "repos"),
			DataHostPath:       dataDir,
			Port:               port,
		}

		// 6. Save Context File
		if err := docker.SaveContext(dctx); err != nil {
			return fmt.Errorf("failed to save docker context: %w", err)
		}

		// 7. Print Mode Alert
		fmt.Println("🚀 GitSense Chat is starting in Docker...")
		fmt.Printf("✅ Docker context initialized at %s\n\n", settings.DockerContextFileName)
		fmt.Println("⚠️  MODE ALERT:")
		fmt.Println("   The CLI is now in 'Docker Proxy' mode. Commands like 'gsc contract create'")
		fmt.Println("   will be automatically sent to the container to ensure database integrity.")
		fmt.Println("")
		fmt.Println("   If you want to run GitSense Chat natively on this host again, you MUST")
		fmt.Println("   delete the context file:")
		fmt.Printf("   rm %s\n\n", settings.DockerContextFileName)

		// 8. Start Container
		if err := docker.StartContainer(ctx, dctx, image, envFile, startPull); err != nil {
			// Cleanup context if start fails
			_ = docker.DeleteContext()
			return err
		}

		fmt.Printf("✅ Container '%s' started successfully on port %s\n", containerName, port)
		fmt.Printf("   Access it at: http://localhost:%s\n", port)
		fmt.Printf("   View logs with: gsc docker logs\n")

		return nil
	},
}

func init() {
	DockerCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&startReposDir, "repos-dir", "r", "", "Host path to your Git repositories (optional)")
	startCmd.Flags().StringVarP(&startDataDir, "data-dir", "d", "", "Host path for persistent data (optional, uses named volume if empty)")
	startCmd.Flags().StringVarP(&startPort, "port", "p", settings.DefaultAppPort, "Host port to map to the container")
	startCmd.Flags().StringVarP(&startEnvFile, "env-file", "e", ".env", "Path to the .env file containing API keys")
	startCmd.Flags().StringVarP(&startName, "name", "n", settings.DefaultContainerName, "Custom name for the container")
	startCmd.Flags().StringVarP(&startImage, "image", "i", settings.DefaultImageName, "The Docker image to pull and run")
	startCmd.Flags().BoolVar(&startPull, "pull", false, "Force pull the latest image before starting")
}
