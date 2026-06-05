/**
 * Component: Docker Orchestration Manager
 * Block-UUID: c1cd86ff-b0c8-48e4-88bf-6b69df789b4a
 * Parent-UUID: cd1c9ad2-a3b2-41af-9cfa-6a9fdcf93407
 * Version: 1.6.0
 * Description: Suppressed stdout output from docker stop and docker rm commands to prevent duplicate container name printing. Only stderr is shown to the user for error messages.
 * Language: Go
 * Created-at: 2026-03-20T22:29:46.899Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
 */


package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// StartContainer launches the GitSense Chat container with the specified context and options.
func StartContainer(ctx context.Context, dctx DockerContext, image string, pull bool) error {
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
	} else {
		// Check if image exists locally if not pulling
		inspectCmd := exec.CommandContext(ctx, "docker", "image", "inspect", image)
		if err := inspectCmd.Run(); err != nil {
			return fmt.Errorf("image %s not found locally. Use --pull to fetch it", image)
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
		args = append(args, "-v", fmt.Sprintf("%s:%s/repos", dctx.ReposHostPath, settings.DockerRootPrefix))
	}

	args = append(args, image)

	// 4. Execute
	logger.Debug("Executing docker run", "args", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "docker", args...)
	
	// Capture stdout to get container ID, but don't display it to user
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Optionally capture container ID for internal use (e.g., logging)
	containerID := strings.TrimSpace(stdoutBuf.String())
	if containerID != "" {
		logger.Debug("Container started", "id", containerID)
	}

	return nil
}

// StopContainer stops and removes the specified container.
func StopContainer(ctx context.Context, name string) error {
	logger.Info("Stopping container", "name", name)
	
	// Try graceful stop first
	stopCmd := exec.CommandContext(ctx, "docker", "stop", name)
	// Suppress stdout to avoid printing container name twice
	// Only show stderr for error messages
	stopCmd.Stdout = nil
	stopCmd.Stderr = os.Stderr
	
	if err := stopCmd.Run(); err != nil {
		logger.Warning("Graceful stop failed, attempting force remove", "error", err)
		// If graceful stop fails, try force remove
		rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", name)
		rmCmd.Stdout = nil
		rmCmd.Stderr = os.Stderr
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop and remove container '%s': %w", name, err)
		}
		return nil
	}
	
	// Remove the stopped container
	rmCmd := exec.CommandContext(ctx, "docker", "rm", name)
	// Suppress stdout to avoid printing container name twice
	// Only show stderr for error messages
	rmCmd.Stdout = nil
	rmCmd.Stderr = os.Stderr
	
	if err := rmCmd.Run(); err != nil {
		logger.Warning("Failed to remove stopped container", "error", err)
		// Don't return error - container is stopped, which is the important part
	}
	
	return nil
}

// IsContainerRunning checks if a container is currently running.
func IsContainerRunning(ctx context.Context, name string) (bool, error) {
	// Use docker inspect to check container state
	inspectCmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.State.Running}}", name)
	
	output, err := inspectCmd.CombinedOutput()
	if err != nil {
		// If container doesn't exist, return false
		if strings.Contains(string(output), "No such container") {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect container '%s': %w", name, err)
	}
	
	running := strings.TrimSpace(string(output)) == "true"
	return running, nil
}

// ExecCommand executes a command inside the container.
func ExecCommand(ctx context.Context, name string, args []string, interactive bool, workdir string) error {
	dockerArgs := []string{"exec"}
	
	if interactive {
		dockerArgs = append(dockerArgs, "-it")
	}
	
	if workdir != "" {
		dockerArgs = append(dockerArgs, "-w", workdir)
	}
	
	dockerArgs = append(dockerArgs, name)
	dockerArgs = append(dockerArgs, args...)
	
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	return cmd.Run()
}

// GetLogs retrieves logs from the container.
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
