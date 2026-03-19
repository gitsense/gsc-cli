/**
 * Component: Docker CLI Start
 * Block-UUID: dc58ec92-1cd0-446c-a56f-cc9204a99b2e
 * Parent-UUID: 04cd91e6-f493-4b9a-99e5-6d2731b91caa
 * Version: 1.7.0
 * Description: Fixed compilation error by correcting package alias usage from docker_internal to docker.
 * Language: Go
 * Created-at: 2026-03-19T19:00:56.897Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0)
 */


package docker

import (
	"io"
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
	startReposDir string // Host path to the umbrella directory containing Git repositories
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
			
			// Note: --repos-dir is intended to be an umbrella directory for projects/workspaces.
			// We do not validate if it is a git repo itself, as it may contain multiple repos.
			fmt.Printf("✅ Umbrella directory set: %s\n", absReposDir)
			reposDir = absReposDir
		}

		// 2. Resolve Data Directory
		dataDir := startDataDir
		if dataDir == "" {
			// Default to isolated Docker data directory
			gscHome, err := settings.GetGSCHome(false)
			if err != nil {
				return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
			}
			dataDir = filepath.Join(gscHome, settings.DockerDataDirRelPath)
		} else {
			absDataDir, err := filepath.Abs(dataDir)
			if err != nil {
				return fmt.Errorf("failed to resolve data directory: %w", err)
			}
			dataDir = absDataDir
		}

		// 3. Resolve and Consolidate Environment File
		persistentEnvPath := filepath.Join(dataDir, ".env")
		sourceEnvPath := ""
		if startEnvFile != "" {
			// User provided a source file, copy it to the persistent data directory
			absSource, err := filepath.Abs(startEnvFile)
			if err != nil {
				return fmt.Errorf("failed to resolve source env file path: %w", err)
			}
			sourceEnvPath = absSource

			if _, err := os.Stat(sourceEnvPath); os.IsNotExist(err) {
				return fmt.Errorf("env file not found: %s", sourceEnvPath)
			}
			
			src, err := os.Open(sourceEnvPath)
			if err != nil {
				return fmt.Errorf("failed to open source env file: %w", err)
			}
			defer src.Close()
			
			dst, err := os.Create(persistentEnvPath)
			if err != nil {
				return fmt.Errorf("failed to create persistent env file: %w", err)
			}
			defer dst.Close()
			
			if _, err := io.Copy(dst, src); err != nil {
				return fmt.Errorf("failed to copy env file: %w", err)
			}
			
			fmt.Printf("✅ Environment file copied to persistent storage: %s\n", persistentEnvPath)
		} else {
			// No source file provided, check if one exists in persistent storage
			if _, err := os.Stat(persistentEnvPath); err == nil {
				fmt.Printf("✅ Using existing environment file: %s\n", persistentEnvPath)
			} else {
				fmt.Println("⚠️  No environment file found. The app will start without API keys.")
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
			EnvHostPath:        sourceEnvPath, // Track the source path for Link & Update
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
		if err := docker.StartContainer(ctx, dctx, image, "", startPull); err != nil {
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
