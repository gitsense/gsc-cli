/**
 * Component: Docker Proxy Engine
 * Block-UUID: 3de2804b-8fce-4ac4-9061-d1aaa1728b25
 * Parent-UUID: 1b969df3-aaa0-49db-9cd8-a0320434eee5
 * Version: 1.3.0
 * Description: Implemented environment variable forwarding, symlink fallback for Windows, case-insensitive path checking, and added IsInContainer detection to prevent recursive proxy loops.
 * Language: Go
 * Created-at: 2026-03-21T03:52:14.527Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// IsInContainer checks if the current process is running inside the Docker container
// by verifying if GSC_HOME matches the Docker root prefix.
func IsInContainer() bool {
	return os.Getenv("GSC_HOME") == settings.DockerRootPrefix
}

// ProxyCommand intercepts a CLI command and redirects it to the Docker container
// if a valid Docker context is active.
func ProxyCommand(cmd *cobra.Command, args []string) (bool, error) {
	// Initialize context early as it is needed for the status check in the banner
	ctx := context.Background()

	// 1. Check for Docker Context
	dctx, err := LoadContext()
	if err != nil {
		return false, fmt.Errorf("failed to load docker context: %w", err)
	}

	// If no context file exists, we are in native mode.
	if dctx == nil {
		return false, nil
	}

	// 2. UX: Print the Docker Proxy Banner to stderr
	contextPath, err := GetContextPath()
	if err != nil {
		contextPath = "unknown"
	}

	running, err := IsContainerRunning(ctx, dctx.ContainerName)
	if err != nil {
		running = false
	}

	fmt.Fprintf(os.Stderr, "[gsc] Docker Proxy Mode Active\n")
	fmt.Fprintf(os.Stderr, "   Container: %s\n", dctx.ContainerName)
	fmt.Fprintf(os.Stderr, "   Status:    %s\n", map[bool]string{true: "Running", false: "Stopped"}[running])
	fmt.Fprintf(os.Stderr, "   Context:   %s\n\n", contextPath)

	if running {
		fmt.Fprintf(os.Stderr, "To disconnect: Stop the container ('gsc docker stop'), then delete the context file.\n")
	} else {
		fmt.Fprintf(os.Stderr, "To disconnect: Delete the context file at the path above.\n")
	}
	fmt.Fprintf(os.Stderr, "\n")

	// 3. Verify Container is Running
	// Note: running and err are already populated from the banner section above.
	if err != nil {
		return false, fmt.Errorf("failed to check container status: %w", err)
	}

	if !running {
		return false, fmt.Errorf("docker context found but container '%s' is not running. Run 'gsc docker start' or delete the context file to use native mode", dctx.ContainerName)
	}

	// Check if gsc binary exists in container
	if err := ExecCommand(ctx, dctx.ContainerName, []string{"which", "gsc"}, false, ""); err != nil {
		return false, fmt.Errorf("gsc binary not found in container '%s'. The container may not be properly initialized. Try: gsc docker stop && gsc docker start --pull", dctx.ContainerName)
	}

	logger.Debug("Docker proxy active", "container", dctx.ContainerName)

	// 4. Path Translation (Host -> Container)
	hostCwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("failed to get current directory: %w", err)
	}

	containerWorkdir, err := TranslatePathToContainer(hostCwd, dctx)
	if err != nil {
		return false, fmt.Errorf("path translation failed: %w. To run natively, delete the context file: rm %s", err, settings.DockerContextFileName)
	}
	logger.Debug("Path translated", "host", hostCwd, "container", containerWorkdir)

	// 5. Build Proxy Arguments
	// We reconstruct the full command line to pass to the container's gsc binary.
	proxyArgs := []string{"gsc"}
	proxyArgs = append(proxyArgs, cmd.CommandPath()[4:]) // Strip 'gsc ' prefix from CommandPath
	proxyArgs = append(proxyArgs, args...)

	// 6. Forward Environment Variables
	// Capture GSC_ prefixed environment variables to pass to the container
	var envArgs []string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GSC_") {
			envArgs = append(envArgs, "-e", env)
		}
	}
	// Prepend environment variables to the command arguments
	proxyArgs = append(envArgs, proxyArgs...)

	// 7. Execute via Docker Exec
	// We use interactive mode to preserve TTY for prompts (like contract creation)
	err = ExecCommand(ctx, dctx.ContainerName, proxyArgs, true, containerWorkdir)
	if err != nil {
		// The command failed inside the container. We return the error so the 
		// host process can exit with the appropriate code.
		return true, err
	}

	return true, nil
}

// TranslatePathToContainer converts a host absolute path to its corresponding 
// path inside the Docker container based on the context mapping.
func TranslatePathToContainer(hostPath string, dctx *DockerContext) (string, error) {
	absHostPath, err := filepath.Abs(hostPath)
	if err != nil {
		return "", err
	}
	
	// Resolve symlinks to ensure accurate mapping
	// Fallback to absolute path if symlink resolution fails (e.g., on Windows without permissions)
	absHostPath, err = filepath.EvalSymlinks(absHostPath)
	if err != nil {
		logger.Debug("Failed to resolve symlinks, using absolute path as-is", "error", err)
		absHostPath, _ = filepath.Abs(hostPath)
	}

	// If no repos mount was provided, we cannot translate paths for contracts.
	if dctx.ReposHostPath == "" {
		return "", fmt.Errorf("no repository directory was mapped during 'gsc docker start'")
	}

	absReposHostPath, _ := filepath.Abs(dctx.ReposHostPath)

	// Check if the host path is within the mapped repos directory
	// Use case-insensitive check for Windows/macOS compatibility
	hostPathLower := strings.ToLower(absHostPath)
	reposHostPathLower := strings.ToLower(absReposHostPath)

	if !strings.HasPrefix(hostPathLower, reposHostPathLower) {
		return "", fmt.Errorf("current directory (%s) is not inside the repository root defined in your Docker context (%s)", absHostPath, absReposHostPath)
	}

	// Calculate relative path using the original casing to preserve it in the container
	rel, err := filepath.Rel(absReposHostPath, absHostPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("failed to calculate relative path from %s to %s", absReposHostPath, absHostPath)
	}

	// Construct the container path: /gsc-docker-app/repos + relative offset
	containerPath := filepath.Join(dctx.ReposContainerPath, rel)
	
	// Ensure we use forward slashes for the Linux container
	return filepath.ToSlash(containerPath), nil
}

// TranslatePathToHost converts a container absolute path back to its 
// corresponding path on the host machine.
func TranslatePathToHost(containerPath string, dctx *DockerContext) (string, error) {
	// Check if the path starts with the magic prefix
	if !strings.HasPrefix(containerPath, dctx.ReposContainerPath) {
		return containerPath, nil // Not a mapped path
	}

	// Calculate relative offset from the container repo root
	rel := strings.TrimPrefix(containerPath, dctx.ReposContainerPath)
	rel = strings.TrimPrefix(rel, "/")

	// Construct host path: ReposHostPath + relative offset
	hostPath := filepath.Join(dctx.ReposHostPath, rel)
	
	return hostPath, nil
}

// IsProxyableCommand determines if a command should be considered for proxying.
// We exclude docker management commands to avoid infinite loops.
func IsProxyableCommand(cmd *cobra.Command) bool {
	excluded := []string{"docker", "version", "help"}
	
	current := cmd
	for current != nil {
		for _, name := range excluded {
			if current.Name() == name {
				return false
			}
		}
		current = current.Parent()
	}
	
	return true
}
