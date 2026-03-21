/**
 * Component: Docker CLI Admin
 * Block-UUID: f170bd65-1aed-4f15-8b61-da1e14c5e310
 * Parent-UUID: 3c9ed71b-f43f-4a4f-91f5-35238b41fd95
 * Version: 2.1.0
 * Description: Implements the 'gsc docker admin' command suite for managing environment files, configuration, and LLM settings within the Docker environment.
 * Language: Go
 * Created-at: 2026-03-21T04:15:27.843Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v2.0.0), GLM-4.7 (v2.1.0)
 */


package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// adminCmd represents the docker admin command
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage the GitSense Chat Docker environment",
	Long: `Provides commands to manage environment variables, configuration files,
and LLM settings for the running GitSense Chat container.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Check if container is running
		ctx := context.Background()
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			logger.Fatal("Failed to load docker context", "error", err)
		}
		if dctx == nil {
			logger.Fatal("No active Docker context found. Run 'gsc docker start' first")
		}

		running, err := docker_internal.IsContainerRunning(ctx, dctx.ContainerName)
		if err != nil {
			logger.Fatal("Failed to check container status", "error", err)
		}
		if !running {
			logger.Fatal("Container is not running. Start it with 'gsc docker start'")
		}
	},
}

// --- LLM Command (Proxy) ---

var llmCmd = &cobra.Command{
	Use:   "llm [args...]",
	Short: "Manage LLM models and providers (proxied to container)",
	DisableFlagParsing: true, // Pass all flags to the subcommand
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cmd.SilenceUsage = true // Suppress usage output on error

		dctx, _ := docker_internal.LoadContext()
		
		// Construct the command to run inside the container
		// gsc-admin llm <args...>
		containerArgs := append([]string{"gsc-admin", "llm"}, args...)
		
		return docker_internal.ExecCommand(ctx, dctx.ContainerName, containerArgs, true, "")
	},
}

// --- DOCTOR Command ---

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics on the Docker environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true // Suppress usage output on error

		ctx := context.Background()
		dctx, _ := docker_internal.LoadContext()

		fmt.Println("🩺 GitSense Chat Docker Diagnostics")
		fmt.Println("-----------------------------------")

		// 1. Check Container Status
		running, _ := docker_internal.IsContainerRunning(ctx, dctx.ContainerName)
		status := "Stopped"
		if running {
			status = "Running"
		}
		fmt.Printf("Container Status: %s\n", status)

		// 2. Check .env file
		envPath := dctx.EnvHostPath
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			fmt.Printf(".env file missing: %s\n", envPath)
		} else {
			fmt.Printf(".env file found: %s\n", envPath)
		}

		// 3. Check chat-config.json
		configPath := dctx.DataHostPath + "/chat-config.json"
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("chat-config.json missing: %s\n", configPath)
		} else {
			fmt.Printf("chat-config.json found: %s\n", configPath)
		}

		// 4. Check gsc-admin in container
		checkCmd := exec.CommandContext(ctx, "docker", "exec", dctx.ContainerName, "which", "gsc-admin")
		if err := checkCmd.Run(); err != nil {
			fmt.Println("gsc-admin not found in container")
		} else {
			fmt.Println("gsc-admin found in container")
		}

		return nil
	},
}

// --- Helper ---

func promptRestartIfNeeded() error {
	// forceRestart flag was removed, so we always prompt
	confirm := false
	prompt := &survey.Confirm{
		Message: "Changes applied. Restart the container now?",
		Default: true,
	}
	survey.AskOne(prompt, &confirm)

	if confirm {
		ctx := context.Background()
		dctx, _ := docker_internal.LoadContext()
		fmt.Printf("Restarting container '%s'...\n", dctx.ContainerName)
		restartCmd := exec.CommandContext(ctx, "docker", "restart", dctx.ContainerName)
		restartCmd.Stdout = os.Stdout
		restartCmd.Stderr = os.Stderr
		if err := restartCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}
		fmt.Println("Container restarted successfully.")
	} else {
		fmt.Println("Remember to restart manually with 'gsc docker restart'")
	}
	return nil
}

func init() {
	DockerCmd.AddCommand(adminCmd)

	// Register LLM subcommand
	adminCmd.AddCommand(llmCmd)

	// Register Doctor subcommand
	adminCmd.AddCommand(doctorCmd)
}
