/**
 * Component: Docker CLI Start
 * Block-UUID: 8ad793b8-1842-49d4-bfe7-40c57541d19a
 * Parent-UUID: 90c673bc-9546-4a04-8bf8-7128d2bf7c9f
 * Version: 1.3.1
 * Description: Fixed compilation error by correcting package alias usage from docker_internal to docker.
 * Language: Go
 * Created-at: 2026-03-19T02:33:45.688Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1)
 */


package docker

import (
	"context"
	"fmt"
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/internal/git"
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
			
			// Validate it is a Git repository
			if _, err := git.FindGitRootFrom(absReposDir); err != nil {
				return fmt.Errorf("repository directory '%s' does not appear to be a Git repository. Please provide a directory containing a .git folder", absReposDir)
			}
			fmt.Printf("✅ Git repository detected at: %s\n", absReposDir)
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
			// Validate Env File
			if _, err := os.Stat(envFile); os.IsNotExist(err) {
				return fmt.Errorf("env file not found: %s", envFile)
			}

			// Check for critical keys
			file, err := os.Open(envFile)
			if err != nil {
				return fmt.Errorf("failed to open env file: %w", err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			foundKeys := false
			for scanner.Scan() {
				line := scanner.Text()
				if strings.Contains(line, "_API_KEY=") {
					foundKeys = true
					break
				}
			}
			if !foundKeys {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: No API keys found in %s. The app may not function correctly.\n", envFile)
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

		// 5. Pre-flight Checks
		
		// Check Host Port Availability
		ln, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return fmt.Errorf("host port %s is already in use. Use --port to specify a different one", port)
		}
		ln.Close()

		// Check Container Name Collision
		running, err := docker.IsContainerRunning(ctx, containerName)
		if err == nil && running {
			return fmt.Errorf("container '%s' is already running. Stop it first with: gsc docker stop", containerName)
		}

		// Check if container exists but is stopped
		inspectCmd := exec.Command("docker", "inspect", containerName)
		if inspectCmd.Run() == nil {
			return fmt.Errorf("container '%s' already exists (but is stopped). Remove it with: docker rm %s", containerName, containerName)
		}

		// 6. Create Docker Context
		dctx := docker.DockerContext{
			ContainerName:      containerName,
			ReposHostPath:      reposDir,
			ReposContainerPath: filepath.Join(settings.DockerRootPrefix, "repos"),
			DataHostPath:       dataDir,
			Port:               port,
		}

		// 7. Save Context File
		if err := docker.SaveContext(dctx); err != nil {
			return fmt.Errorf("failed to save docker context: %w", err)
		}

		// 8. Print Mode Alert
		fmt.Println("🚀 GitSense Chat is starting in Docker...")
		fmt.Printf("✅ Docker context initialized at %s\n\n", settings.DockerContextFileName)
		fmt.Println("⚠️  MODE ALERT:")
		fmt.Println("   The CLI is now in 'Docker Proxy' mode. Commands like 'gsc contract create'")
		fmt.Println("   will be automatically sent to the container to ensure database integrity.")
		fmt.Println("")
		fmt.Println("   If you want to run GitSense Chat natively on this host again, you MUST")
		fmt.Println("   delete the context file:")
		fmt.Printf("   rm %s\n\n", settings.DockerContextFileName)

		// 9. Start Container
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
