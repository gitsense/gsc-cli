/**
 * Component: Docker CLI Env Manager
 * Block-UUID: 37320a07-fb9b-4b3f-b279-454b1d632ac6
 * Parent-UUID: 699d6099-8a52-463f-9a69-f4206cdfe81c
 * Version: 1.1.0
 * Description: Implements the 'gsc docker env' command suite, focusing on a Link/Update workflow for synchronizing host-side environment files with the container's persistent data volume.
 * Language: Go
 * Created-at: 2026-03-19T19:08:54.938Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
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
)

// envCmd represents the docker env command
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage and synchronize the Docker environment file (.env)",
	Long: `Displays the status of the linked environment file and provides commands
to link new source files or update the container from the existing link.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dctx, err := docker_internal.LoadContext()
		if err != nil {
			return err
		}
		if dctx == nil {
			fmt.Println("No active Docker context found. Run 'gsc docker start' first.")
			return nil
		}

		fmt.Println("\n🐳 GitSense Chat Environment Status")
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
			fmt.Println("Status: ✅ Up to date")
		} else {
			fmt.Println("Status: ⚠️  Out of sync (Host source has changed)")
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

		fmt.Printf("✅ Linked %s to container storage.\n", sourcePath)
		return promptRestart(dctx.ContainerName)
	},
}

var envUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Synchronize the container with the linked host-side .env file",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			fmt.Println("✅ Environment is already up to date.")
			return nil
		}

		// 2. Perform the update
		if err := copyFile(dctx.EnvHostPath, targetPath); err != nil {
			return fmt.Errorf("failed to update env file: %w", err)
		}

		fmt.Printf("✅ Updated container from %s\n", dctx.EnvHostPath)
		return promptRestart(dctx.ContainerName)
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
		fmt.Printf("🚀 Restarting container '%s'...\n", containerName)
		restartCmd := exec.CommandContext(ctx, "docker", "restart", containerName)
		restartCmd.Stdout = os.Stdout
		restartCmd.Stderr = os.Stderr
		if err := restartCmd.Run(); err != nil {
			return fmt.Errorf("failed to restart container: %w", err)
		}
		fmt.Println("✅ Container restarted successfully.")
	} else {
		fmt.Println("⚠️  Remember to restart manually with 'gsc docker restart' to apply changes.")
	}
	return nil
}

func init() {
	DockerCmd.AddCommand(envCmd)
	envCmd.AddCommand(envLinkCmd)
	envCmd.AddCommand(envUpdateCmd)
}
