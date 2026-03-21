/**
 * Component: Docker CLI Env Manager
 * Block-UUID: 8fc19419-4d5f-4cc2-b4d0-04331613684f
 * Parent-UUID: 96a84608-3e09-4349-8a3e-7cdc77f96266
 * Version: 1.3.0
 * Description: Implements the 'gsc docker env' command suite, focusing on a Link/Update workflow for synchronizing host-side environment files with the container's persistent data volume.
 * Language: Go
 * Created-at: 2026-03-21T15:02:41.618Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package docker

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"

	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// envCmd represents the docker env command
var envCmd = &cobra.Command{
	Use:   "env",
	Aliases: []string{"environment"},
	Short: "Manage and synchronize the Docker environment file (.env)",
	Long: `Displays the status of the linked environment file and provides commands
to link new source files or update the container from the existing link.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dctx, err := docker_internal.LoadContext()
		cmd.SilenceUsage = true // Suppress usage output on error

		if err != nil {
			return err
		}
		if dctx == nil {
			fmt.Println("No active Docker context found. Run 'gsc docker start' first.")
			return nil
		}

		fmt.Println("\nGitSense Chat Environment Status")
		fmt.Println("-----------------------------------")

		if dctx.EnvHostPath == "" {
			fmt.Println("Status: No host-side environment file linked.")
			fmt.Println("Action: Run 'gsc docker env link <path>' to establish a link.")
			return nil
		}

		fmt.Printf("Linked Source: %s\n", dctx.EnvHostPath)
		fmt.Printf("Active Target: %s\n", filepath.Join(dctx.DataHostPath, ".env"))

		// Check sync status
		inSync, err := checkEnvSync(dctx.EnvHostPath, filepath.Join(dctx.DataHostPath, ".env"))
		if err != nil {
			fmt.Printf("Status: Error checking sync (%v)\n", err)
		} else if inSync {
			fmt.Println("Status: Up to date")
		} else {
			fmt.Println("Status: Out of sync (Host source has changed)")
			fmt.Println("Action: Run 'gsc docker env update' to pull changes.")
		}
		fmt.Println("")

		return nil
	},
}

var envLinkCmd = &cobra.Command{
	Use:   "link <source_path>",
	Short: "Link a host-side .env file to the container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		sourcePath, _ := filepath.Abs(args[0])
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dctx.DataHostPath, ".env")

		// 1. Perform the copy
		if err := copyFile(sourcePath, targetPath); err != nil {
			return fmt.Errorf("failed to link env file: %w", err)
		}

		// 2. Update Context
		dctx.EnvHostPath = sourcePath
		if err := docker_internal.SaveContext(*dctx); err != nil {
			return fmt.Errorf("failed to update docker context: %w", err)
		}

		fmt.Printf("Linked %s to container storage.\n", sourcePath)
		return promptRestart(dctx.ContainerName)
	},
}

var envUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Synchronize the container with the linked host-side .env file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}

		if dctx.EnvHostPath == "" {
			return fmt.Errorf("no environment file is currently linked. Run 'gsc docker env link <path>' first")
		}

		targetPath := filepath.Join(dctx.DataHostPath, ".env")

		// 1. Check if update is needed
		inSync, err := checkEnvSync(dctx.EnvHostPath, targetPath)
		if err != nil {
			return err
		}

		if inSync {
			fmt.Println("Environment is already up to date.")
			return nil
		}

		// 2. Perform the update
		if err := copyFile(dctx.EnvHostPath, targetPath); err != nil {
			return fmt.Errorf("failed to update env file: %w", err)
		}

		fmt.Printf("Updated container from %s\n", dctx.EnvHostPath)
		return promptRestart(dctx.ContainerName)
	},
}

var envInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a master .env file and link it to Docker",
	Long: `Creates a master .env file at ~/.gitsense/.env from the container's 
.env.example template and links it to the Docker data directory. 
This provides a single source of truth for API keys.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Paths
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		masterEnvPath := filepath.Join(gscHome, ".env")

		// Resolve Data Directory (from context or default)
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return fmt.Errorf("failed to load docker context: %w", err)
		}

		dataDir := ""
		if dctx != nil {
			dataDir = dctx.DataHostPath
		} else {
			// Default data directory if no context exists
			dataDir = filepath.Join(gscHome, settings.DockerDataDirRelPath)
		}

		// Ensure data directory exists
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		persistentEnvPath := filepath.Join(dataDir, ".env")

		// 2. Check if Master File already exists
		if _, err := os.Stat(masterEnvPath); err == nil {
			return fmt.Errorf("master .env file already exists at %s.\nPlease edit it directly or use 'gsc docker env link' to use a different file.", masterEnvPath)
		}

		// 3. Extract .env.example from Docker Image
		image := settings.DefaultImageName
		fmt.Printf("Extracting .env.example from image '%s'...\n", image)

		// Command: docker run --rm --entrypoint cat <image> .env.example
		extractCmd := exec.Command("docker", "run", "--rm", "--entrypoint", "cat", image, ".env.example")
		
		// Read output
		content, err := extractCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to extract .env.example from image: %w\nEnsure Docker is running and the image '%s' is available.", image, err)
		}

		// Write to master file
		if err := os.WriteFile(masterEnvPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write master .env file: %w", err)
		}
		fmt.Printf("Created master .env file: %s\n", masterEnvPath)

		// 4. Create Symlink in Data Directory
		// Remove existing link/file if it exists
		if _, err := os.Lstat(persistentEnvPath); err == nil {
			os.Remove(persistentEnvPath)
		}

		// Create relative symlink if possible, or absolute
		// For simplicity and robustness across OS, absolute path is safer here
		if err := os.Symlink(masterEnvPath, persistentEnvPath); err != nil {
			return fmt.Errorf("failed to create symlink: %w", err)
		}
		fmt.Printf("Linked %s -> %s\n", persistentEnvPath, masterEnvPath)

		// 5. Update Docker Context
		if dctx == nil {
			// Create default context if it didn't exist
			dctx = &docker_internal.DockerContext{
				ContainerName:      settings.DefaultContainerName,
				ReposHostPath:      "",
				ReposContainerPath: filepath.Join(settings.DockerRootPrefix, "repos"),
				DataHostPath:       dataDir,
				Port:               settings.DefaultAppPort,
			}
		}
		
		dctx.EnvHostPath = masterEnvPath
		if err := docker_internal.SaveContext(*dctx); err != nil {
			return fmt.Errorf("failed to update docker context: %w", err)
		}

		fmt.Println("\n✓ Initialization complete!")
		fmt.Printf("1. Edit your API keys in: %s\n", masterEnvPath)
		fmt.Println("2. Restart the container to apply changes:")
		fmt.Println("   gsc docker restart")

		return nil
	},
}

// --- Helpers ---

func checkEnvSync(source, target string) (bool, error) {
	sHash, err := getFileHash(source)
	if err != nil {
		return false, err
	}
	tHash, err := getFileHash(target)
	if err != nil {
		// If target doesn't exist, they are not in sync
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(sHash, tHash), nil
}

func getFileHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func promptRestart(containerName string) error {
	confirm := false
	prompt := &survey.Confirm{
		Message: "Changes applied. Restart the container now to take effect?",
		Default: true,
	}
	survey.AskOne(prompt, &confirm)

	if confirm {
		ctx := context.Background()
		fmt.Printf("Restarting container '%s'...\n", containerName)
		restartCmd := exec.CommandContext(ctx, "docker", "restart", containerName)
		restartCmd.Stdout = os.Stdout
		restartCmd.Stderr = os.Stderr
		if err := restartCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}
		fmt.Println("Container restarted successfully.")
	} else {
		fmt.Println("Remember to restart manually with 'gsc docker restart' to apply changes.")
	}
	return nil
}

func init() {
	DockerCmd.AddCommand(envCmd)
	envCmd.AddCommand(envLinkCmd)
	envCmd.AddCommand(envUpdateCmd)
	envCmd.AddCommand(envInitCmd)
}
