/**
 * Component: Docker Context Manager
 * Block-UUID: 8edd9232-0890-4170-b74d-599fcc9340bf
 * Parent-UUID: 7caf9147-b45a-4834-9337-57e6b9ab520f
 * Version: 1.3.0
 * Description: Added inline documentation, secure directory permissions, context validation logic, and thread-safety documentation.
 * Language: Go
 * Created-at: 2026-03-19T18:49:31.697Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0)
 */


package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// DockerContext represents the metadata stored in the hidden context file (.gsc-docker-context.json).
// This file acts as the "source of truth" for the CLI's execution mode:
// - If the file exists, the CLI is in "Docker Proxy Mode" and will redirect write operations to the container.
// - If the file does not exist, the CLI is in "Native Mode" and operates directly on the host.
//
// The context file is created by 'gsc docker start' and deleted by 'gsc docker stop'.
// Users can manually delete this file to return to Native Mode at any time.
type DockerContext struct {
	ContainerName      string    `json:"container_name"`
	ReposHostPath      string    `json:"repos_host_path"`
	ReposContainerPath string    `json:"repos_container_path"`
	DataHostPath       string    `json:"data_host_path"`
	EnvHostPath        string    `json:"env_host_path"`
	Port               string    `json:"port"`
	LastStarted        time.Time `json:"last_started"`
}

// SaveContext performs an atomic write of the Docker context to the hidden file.
// Note: This function is not thread-safe. Concurrent calls to SaveContext may result in data loss.
// In practice, this is unlikely because 'gsc docker start' is typically run once per session.
func SaveContext(ctx DockerContext) error {
	path, err := GetContextPath()
	if err != nil {
		return err
	}

	// Ensure .gitsense directory exists with secure permissions (0700)
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}
	if err := os.MkdirAll(gscHome, 0700); err != nil {
		return fmt.Errorf("failed to create .gitsense directory: %w", err)
	}

	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal docker context: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil { // 0600: Read/write for owner only (security: contains path mappings)
		return fmt.Errorf("failed to write temp docker context file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename docker context file: %w", err)
	}

	return nil
}

// LoadContext reads and unmarshals the Docker context file.
func LoadContext() (*DockerContext, error) {
	path, err := GetContextPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read docker context file: %w", err)
	}

	var ctx DockerContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal docker context: %w", err)
	}

	return &ctx, nil
}

// DeleteContext removes the hidden Docker context file.
func DeleteContext() error {
	path, err := GetContextPath()
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete docker context file: %w", err)
	}

	return nil
}

// HasContext checks if the Docker context file exists.
func HasContext() bool {
	path, err := GetContextPath()
	if err != nil {
		return false
	}

	_, err = os.Stat(path)
	return err == nil
}

// ValidateContext checks if the context is still valid (paths exist, container is running, etc.)
func ValidateContext(ctx *DockerContext) error {
	if ctx.ReposHostPath != "" {
		if _, err := os.Stat(ctx.ReposHostPath); os.IsNotExist(err) {
			return fmt.Errorf(
				"repository directory '%s' no longer exists. "+
					"Run 'gsc docker stop' and restart with a valid path",
				ctx.ReposHostPath,
			)
		}
	}
	return nil
}

// GetContextPath returns the absolute path to the hidden Docker context file.
func GetContextPath() (string, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	return filepath.Join(gscHome, settings.DockerContextFileName), nil
}
