/**
 * Component: Docker CLI Env Manager
 * Block-UUID: 684a671a-a339-400b-a4e1-217d7e03447d
 * Parent-UUID: 5a718fb2-1a44-44ac-930e-12307c310677
 * Version: 1.6.0
 * Description: Added interactive prompt for environment configuration choice. Users can now choose between master .env file, separate Docker-only .env file, or linking an existing file. Added --interactive flag to explicitly trigger the prompt.
 * Language: Go
 * Created-at: 2026-05-13T01:45:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
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
			fmt.Println("No active Docker context found. Run 'gsc app docker start' first.")
			return nil
		}

		fmt.Println("\nGitSense Chat Environment Status")
		fmt.Println("-----------------------------------")

		if dctx.EnvHostPath == "" {
			fmt.Println("Status: No host-side environment file linked.")
			fmt.Println("Action: Run 'gsc app docker env link <path>' to establish a link.")
			return nil
		}

		fmt.Printf("Linked Source: %s\n", dctx.EnvHostPath)
		fmt.Printf("Active Target: %s\n", filepath.Join(dctx.DataHostPath, ".env"))

		// Show configuration type if available
		if dctx.ConfigType != "" {
			fmt.Printf("Configuration Type: %s\n", dctx.ConfigType)
			if dctx.ConfigType == "master" && dctx.MasterEnvPath != "" {
				fmt.Printf("Master File: %s\n", dctx.MasterEnvPath)
			}
		}

		// Check sync status
		inSync, err := checkEnvSync(dctx.EnvHostPath, filepath.Join(dctx.DataHostPath, ".env"))
		if err != nil {
			fmt.Printf("Status: Error checking sync (%v)\n", err)
		} else if inSync {
			fmt.Println("Status: Up to date")
		} else {
			fmt.Println("Status: Out of sync (Host source has changed)")
			fmt.Println("Action: Run 'gsc app docker env update' to pull changes.")
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
		dctx.ConfigType = "existing" // Mark as existing file link
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
			return fmt.Errorf("no environment file is currently linked. Run 'gsc app docker env link <path>' first")
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

var (
	envInitMaster      bool
	envInitMasterPath  string
	envInitInteractive bool
)

var envInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a master .env file and link it to Docker",
	Long: `Creates a master .env file from the container's .env.example template and links it to the Docker data directory. 
This provides a single source of truth for API keys.

Use --master to create a master file (default behavior).
Use --master-path to specify a custom location for the master file.
Use --interactive to show a prompt for configuration choice.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Paths
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

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

		// 2. Determine if we should show interactive prompt
		// Show prompt if: --interactive flag is set OR no flags were changed
		shouldPrompt := envInitInteractive || (!cmd.Flags().Changed("master") && !cmd.Flags().Changed("master-path"))

		var configChoice string

		if shouldPrompt {
			// Show interactive prompt
			fmt.Println("\nGitSense Chat Environment Initialization")
			fmt.Println("------------------------------------------")
			fmt.Println("How would you like to manage your .env file?")
			fmt.Println("")

			prompt := &survey.Select{
				Message: "Choose a configuration option:",
				Options: []string{
					"Create a master .env file (recommended for sharing with native)",
					"Create a separate .env file for Docker only",
					"Use an existing .env file",
				},
				Default: "Create a master .env file (recommended for sharing with native)",
			}

			if err := survey.AskOne(prompt, &configChoice); err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
		} else {
			// Use flag-based logic
			if envInitMaster {
				configChoice = "Create a master .env file (recommended for sharing with native)"
			} else {
				configChoice = "Create a separate .env file for Docker only"
			}
		}

		// 3. Handle each configuration choice
		switch configChoice {
		case "Create a master .env file (recommended for sharing with native)":
			return handleMasterConfig(gscHome, dataDir, persistentEnvPath, dctx)
		case "Create a separate .env file for Docker only":
			return handleSeparateConfig(dataDir, persistentEnvPath, dctx)
		case "Use an existing .env file":
			return handleExistingConfig(dataDir, persistentEnvPath, dctx)
		default:
			return fmt.Errorf("invalid configuration choice: %s", configChoice)
		}
	},
}

// handleMasterConfig creates a master .env file and links it to Docker
func handleMasterConfig(gscHome, dataDir, persistentEnvPath string, dctx *docker_internal.DockerContext) error {
	// Resolve master file path
	masterEnvPath := envInitMasterPath
	if masterEnvPath == "" {
		masterEnvPath = filepath.Join(gscHome, ".env")
	} else {
		absPath, err := filepath.Abs(masterEnvPath)
		if err != nil {
			return fmt.Errorf("failed to resolve master env path: %w", err)
		}
		masterEnvPath = absPath
	}

	// Check if Master File already exists
	if _, err := os.Stat(masterEnvPath); err == nil {
		return fmt.Errorf("master .env file already exists at %s.\nPlease edit it directly or use 'gsc app docker env link' to use a different file.", masterEnvPath)
	}

	// Extract .env.example from Docker Image
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

	// Create Symlink in Data Directory
	if err := createSymlink(masterEnvPath, persistentEnvPath); err != nil {
		return err
	}

	// Update Docker Context
	if dctx == nil {
		dctx = createDefaultContext(dataDir)
	}
	
	dctx.EnvHostPath = masterEnvPath
	dctx.ConfigType = "master"
	dctx.MasterEnvPath = masterEnvPath
	
	if err := docker_internal.SaveContext(*dctx); err != nil {
		return fmt.Errorf("failed to update docker context: %w", err)
	}

	// Print success message
	printConfigSummary("Master .env file", masterEnvPath, persistentEnvPath)
	return nil
}

// handleSeparateConfig creates a separate .env file for Docker only
func handleSeparateConfig(dataDir, persistentEnvPath string, dctx *docker_internal.DockerContext) error {
	// Check if file already exists
	if _, err := os.Stat(persistentEnvPath); err == nil {
		return fmt.Errorf(".env file already exists at %s.\nPlease edit it directly.", persistentEnvPath)
	}

	// Extract .env.example from Docker Image
	image := settings.DefaultImageName
	fmt.Printf("Extracting .env.example from image '%s'...\n", image)

	extractCmd := exec.Command("docker", "run", "--rm", "--entrypoint", "cat", image, ".env.example")
	
	content, err := extractCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to extract .env.example from image: %w\nEnsure Docker is running and the image '%s' is available.", image, err)
	}

	// Write to persistent path
	if err := os.WriteFile(persistentEnvPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write .env file: %w", err)
	}
	fmt.Printf("Created Docker-only .env file: %s\n", persistentEnvPath)

	// Update Docker Context
	if dctx == nil {
		dctx = createDefaultContext(dataDir)
	}
	
	dctx.EnvHostPath = persistentEnvPath
	dctx.ConfigType = "separate"
	dctx.MasterEnvPath = ""
	
	if err := docker_internal.SaveContext(*dctx); err != nil {
		return fmt.Errorf("failed to update docker context: %w", err)
	}

	// Print success message
	printConfigSummary("Docker-only .env file", persistentEnvPath, persistentEnvPath)
	return nil
}

// handleExistingConfig links an existing .env file to Docker
func handleExistingConfig(dataDir, persistentEnvPath string, dctx *docker_internal.DockerContext) error {
	// Prompt for existing file path
	var sourcePath string
	prompt := &survey.Input{
		Message: "Enter the path to your existing .env file:",
		Help:    "This file will be linked to the Docker container. Changes to this file will require running 'gsc app docker env update'.",
	}

	if err := survey.AskOne(prompt, &sourcePath); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	// Resolve absolute path
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", absSourcePath)
	}

	// Create Symlink in Data Directory
	if err := createSymlink(absSourcePath, persistentEnvPath); err != nil {
		return err
	}

	// Update Docker Context
	if dctx == nil {
		dctx = createDefaultContext(dataDir)
	}
	
	dctx.EnvHostPath = absSourcePath
	dctx.ConfigType = "existing"
	dctx.MasterEnvPath = ""
	
	if err := docker_internal.SaveContext(*dctx); err != nil {
		return fmt.Errorf("failed to update docker context: %w", err)
	}

	// Print success message
	printConfigSummary("Existing .env file", absSourcePath, persistentEnvPath)
	return nil
}

// createSymlink creates a symlink from source to target
func createSymlink(source, target string) error {
	// Remove existing link/file if it exists
	if _, err := os.Lstat(target); err == nil {
		os.Remove(target)
	}

	// Create symlink
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}
	fmt.Printf("Linked %s -> %s\n", target, source)
	return nil
}

// createDefaultContext creates a default Docker context
func createDefaultContext(dataDir string) *docker_internal.DockerContext {
	return &docker_internal.DockerContext{
		ContainerName:      settings.DefaultContainerName,
		ReposHostPath:      "",
		ReposContainerPath: filepath.Join(settings.DockerRootPrefix, "repos"),
		DataHostPath:       dataDir,
		Port:               settings.DefaultAppPort,
	}
}

// printConfigSummary prints a formatted configuration summary
func printConfigSummary(configType, sourcePath, targetPath string) {
	fmt.Println("\n✓ Initialization complete!")
	fmt.Println("")
	fmt.Println("Configuration Summary:")
	fmt.Printf("  Type:           %s\n", configType)
	fmt.Printf("  Location:       %s\n", sourcePath)
	fmt.Printf("  Docker Link:    %s\n", targetPath)
	fmt.Println("")
	fmt.Println("Next Steps:")
	fmt.Printf("  1. Edit your API keys in: %s\n", sourcePath)
	fmt.Println("  2. Restart the container to apply changes:")
	fmt.Println("     gsc app docker restart")
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
		fmt.Println("Remember to restart manually with 'gsc app docker restart' to apply changes.")
	}
	return nil
}

func init() {
	DockerCmd.AddCommand(envCmd)
	envCmd.AddCommand(envLinkCmd)
	envCmd.AddCommand(envUpdateCmd)
	envCmd.AddCommand(envInitCmd)

	// Add flags to envInitCmd
	envInitCmd.Flags().BoolVar(&envInitMaster, "master", true, "Create a master .env file (default: true)")
	envInitCmd.Flags().StringVar(&envInitMasterPath, "master-path", "", "Custom path for the master .env file (default: ~/.gitsense/.env)")
	envInitCmd.Flags().BoolVar(&envInitInteractive, "interactive", false, "Show interactive prompt for configuration choice")
}
