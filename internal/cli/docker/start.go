/**
 * Component: Docker CLI Start
 * Block-UUID: c8708340-07f2-44bb-a38f-eb8a1cd64462
 * Parent-UUID: c4dd0d95-828d-4cdb-9bbd-c7a2250bea78
 * Version: 1.13.0
 * Description: Fixed compilation error by correcting package alias usage from docker_internal to docker.
 * Language: Go
 * Created-at: 2026-03-21T15:01:36.811Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), Gemini 3 Flash (v1.8.0), Gemini 3 Flash (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.10.1), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0)
 */


package docker

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/logger"
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
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()

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

		// 1. Resolve Repos Directory
		reposDir := startReposDir
		if reposDir == "" {
			reposDir = os.Getenv("GSC_REPOS_DIR")
		}

		// If still empty, check for an existing context
		if reposDir == "" {
			if dctx, _ := docker.LoadContext(); dctx != nil {
				reposDir = dctx.ReposHostPath
			}
		}

		// If still empty, use the default sibling sandbox directory
		isDefaultRepos := false
		if reposDir == "" {
			gscHome, _ := settings.GetGSCHome(false)
			reposDir = filepath.Join(gscHome, settings.DockerReposDirRelPath)
			isDefaultRepos = true
		}

		// Validate path exists
		absReposDir, err := filepath.Abs(reposDir)
		if err != nil {
			return fmt.Errorf("failed to resolve repos directory: %w", err)
		}
		
		// Ensure the directory exists (especially for the default)
		if err := os.MkdirAll(absReposDir, 0755); err != nil {
			return fmt.Errorf("failed to create repos directory: %w", err)
		}
		
		// Note: --repos-dir is intended to be an umbrella directory for projects/workspaces.
		reposDir = absReposDir

		// 3. Resolve and Consolidate Environment File
		persistentEnvPath := filepath.Join(dataDir, ".env")
		sourceEnvPath := ""
		isLinked := false

		// Priority 1: Explicit --env-file flag
		if startEnvFile != "" {
			absSource, err := filepath.Abs(startEnvFile)
			if err != nil {
				return fmt.Errorf("failed to resolve source env file path: %w", err)
			}
			sourceEnvPath = absSource
		} else {
			// Priority 2: Check Docker Context for existing link
			if dctx, _ := docker.LoadContext(); dctx != nil && dctx.EnvHostPath != "" {
				sourceEnvPath = dctx.EnvHostPath
				isLinked = true
			}
		}

		// Handle Source File Logic
		if sourceEnvPath != "" {
			if _, err := os.Stat(sourceEnvPath); os.IsNotExist(err) {
				// Source file is missing
				if isLinked {
					logger.Warning("Linked environment file missing on host", "path", sourceEnvPath)
					fmt.Println("   The container will use the existing cached .env file if available.")
				} else {
					return fmt.Errorf("specified env file not found: %s", sourceEnvPath)
				}
			} else {
				// Source file exists
				if _, err := os.Stat(persistentEnvPath); os.IsNotExist(err) {
					// Persistent file missing -> Auto Restore
					logger.Info("Restoring environment file from link/source", "source", sourceEnvPath)
					if err := copyFile(sourceEnvPath, persistentEnvPath); err != nil {
						return fmt.Errorf("failed to restore env file: %w", err)
					}
					fmt.Printf("Environment file restored to: %s\n", persistentEnvPath)
				} else {
					// Both exist -> Check for Drift
					inSync, err := checkEnvSync(sourceEnvPath, persistentEnvPath)
					if err != nil {
						logger.Warning("Failed to check env file sync status", "error", err)
					} else if !inSync {
						logger.Warning("Environment file drift detected", "source", sourceEnvPath)
						fmt.Println("   The linked .env file on the host has changed.")
						fmt.Println("   The container is currently using an outdated version.")
						fmt.Println("   Run 'gsc docker env update' to sync changes.")
					}
				}
			}
		}

		// Final Status Check
		if _, err := os.Stat(persistentEnvPath); err != nil {
			if os.IsNotExist(err) {
			} else {
				return fmt.Errorf("failed to access persistent env file: %w", err)
			}
		} else {
			fmt.Printf("Using environment file: %s\n", persistentEnvPath)
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
			logger.Info("Found existing stopped container. Removing it...", "name", containerName)
			if err := docker.StopContainer(ctx, containerName); err != nil {
				return fmt.Errorf("failed to remove existing container '%s': %w", containerName, err)
			}
			logger.Success("Existing container removed", "name", containerName)
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
		fmt.Println("\nGitSense Chat is starting in Docker...")
		fmt.Println("------------------------------------------")
		fmt.Printf("  Container: %s\n", containerName)
		fmt.Printf("  Port:      %s\n", port)
		fmt.Printf("  Data Dir:  %s\n", dataDir)
		fmt.Printf("  Repos Dir: %s", reposDir)
		if isDefaultRepos {
			fmt.Print(" (Default Sandbox)")
		}
		fmt.Println("\n------------------------------------------")
		
		fmt.Println("\nTIP: To change your repository umbrella directory, run:")
		fmt.Println("   gsc docker configure --repos-dir /path/to/your/code")
		
		fmt.Println("\nDOCKER PROXY MODE ACTIVE:")
		fmt.Println("   Commands like 'gsc contract create' will be proxied to the container.")
		fmt.Printf("   Context file: %s\n", settings.DockerContextFileName)
		fmt.Println("")

		// 9. Start Container
		if err := docker.StartContainer(ctx, dctx, image, startPull); err != nil {
			// Cleanup context if start fails
			_ = docker.DeleteContext()
			return err
		}

		fmt.Printf("Container '%s' started successfully on port %s\n", containerName, port)
		fmt.Printf("   Access it at: http://localhost:%s\n", port)
		fmt.Printf("   View logs with: gsc docker logs\n")

		// Check for .env file existence for the final warning
		if _, err := os.Stat(persistentEnvPath); os.IsNotExist(err) {
			fmt.Println("\n⚠️  CRITICAL WARNING: No API Keys Found")
			fmt.Println("   The application started, but AI Chat features will be unavailable.")
			fmt.Println("")
			fmt.Println("   To fix this, choose one of the following options:")
			fmt.Println("")
			fmt.Println("   1. Link an existing file (e.g., ~/.env):")
			fmt.Println("      gsc docker env link <path-to-your-file>")
			fmt.Println("")
			fmt.Println("   2. Create a new master configuration file:")
			fmt.Println("      gsc docker env init")
			fmt.Println("      (This creates ~/.gitsense/.env and links it to Docker)")
		}

		return nil
	},
}

func init() {
	DockerCmd.AddCommand(startCmd)

	startCmd.Flags().StringVarP(&startReposDir, "repos-dir", "r", "", "Host path to your Git repositories (optional)")
	startCmd.Flags().StringVarP(&startDataDir, "data-dir", "d", "", "Host path for persistent data (optional, uses named volume if empty)")
	startCmd.Flags().StringVarP(&startPort, "port", "p", settings.DefaultAppPort, "Host port to map to the container")
	startCmd.Flags().StringVarP(&startEnvFile, "env-file", "e", "", "Path to the .env file containing API keys")
	startCmd.Flags().StringVarP(&startName, "name", "n", settings.DefaultContainerName, "Custom name for the container")
	startCmd.Flags().StringVarP(&startImage, "image", "i", settings.DefaultImageName, "The Docker image to pull and run")
	startCmd.Flags().BoolVar(&startPull, "pull", false, "Force pull the latest image before starting")
}
