/**
 * Component: Docker Orchestration Manager
 * Block-UUID: f5271e40-220b-46a4-9e90-5c6d0c45d7d2
 * Parent-UUID: 75f82a31-1c5a-415b-bcc6-92d99faccccc
 * Version: 1.0.1
 * Description: Provides low-level orchestration for Docker CLI operations, including container lifecycle management and command execution.
 * Language: Go
 * Created-at: 2026-03-19T02:10:50.622Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1)
 */


package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// StartContainer launches the GitSense Chat container with the specified context and options.
func StartContainer(ctx context.Context, dctx DockerContext, image string, envFile string, pull bool) error {
	// 1. Ensure Docker is installed
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker CLI not found. Please install Docker: https://docs.docker.com/get-docker/")
	}

	// 2. Pull image if requested
	if pull {
		logger.Info("Pulling latest image", "image", image)
		pullCmd := exec.CommandContext(ctx, "docker", "pull", image)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", image, err)
		}
	}

	// 3. Build Arguments
	args := []string{
		"run", "-d",
		"--name", dctx.ContainerName,
		"-p", fmt.Sprintf("%s:%s", dctx.Port, settings.DefaultAppPort),
		"--restart", "unless-stopped",
	}

	// Add Data Volume
	if dctx.DataHostPath != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s/data", dctx.DataHostPath, settings.DockerRootPrefix))
	} else {
		// Use named volume if no host path provided
		args = append(args, "-v", fmt.Sprintf("%s-data:%s/data", dctx.ContainerName, settings.DockerRootPrefix))
	}

	// Add Repos Volume (Optional)
	if dctx.ReposHostPath != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s/repos:ro", dctx.ReposHostPath, settings.DockerRootPrefix))
	}

	// Add Env File
	if envFile != "" {
		absEnv, _ := filepath.Abs(envFile)
		args = append(args, "-v", fmt.Sprintf("%s:%s/.env:ro", absEnv, settings.DockerRootPrefix))
	}

	args = append(args, image)

	// 4. Execute
	logger.Debug("Executing docker run", "args", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// StopContainer stops and removes the specified container.
func StopContainer(ctx context.Context, name string) error {
	logger.Info("Stopping container", "name", name)
	
	// Stop
	stopCmd := exec.CommandContext(ctx, "docker", "stop", name)
	_ = stopCmd.Run() // Ignore error if already stopped

	// Remove
	rmCmd := exec.CommandContext(ctx, "docker", "rm", name)
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", name, err)
	}

	return nil
}

// ExecCommand executes a command inside the running container.
func ExecCommand(ctx context.Context, name string, args []string, interactive bool, workdir string) error {
	execArgs := []string{"exec"}
	
	if interactive {
		execArgs = append(execArgs, "-it")
	}

	if workdir != "" {
		execArgs = append(execArgs, "--workdir", workdir)
	}

	execArgs = append(execArgs, name)
	execArgs = append(execArgs, args...)

	logger.Debug("Executing docker exec", "args", strings.Join(execArgs, " "))
	cmd := exec.CommandContext(ctx, "docker", execArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// IsContainerRunning checks if the specified container is currently in the 'running' state.
func IsContainerRunning(ctx context.Context, name string) (bool, error) {
	args := []string{"inspect", "-f", "{{.State.Running}}", name}
	cmd := exec.CommandContext(ctx, "docker", args...)
	
	out, err := cmd.Output()
	if err != nil {
		return false, nil // Container likely doesn't exist
	}

	return strings.TrimSpace(string(out)) == "true", nil
}

// GetLogs tails the logs of the specified container.
func GetLogs(ctx context.Context, name string, follow bool) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
