/**
 * Component: Docker Context Manager
 * Block-UUID: ede1cdd8-e56d-4067-aaa9-47e90bc43470
 * Parent-UUID: a0a22256-45ef-4e6d-b338-7a37a491af24
 * Version: 1.1.0
 * Description: Manages the lifecycle of the hidden Docker context file used to track path mappings and proxy state.
 * Language: Go
 * Created-at: 2026-03-19T02:35:43.668Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
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

// DockerContext represents the metadata stored in the hidden context file.
type DockerContext struct {
	ContainerName      string    `json:"container_name"`
	ReposHostPath      string    `json:"repos_host_path"`
	ReposContainerPath string    `json:"repos_container_path"`
	DataHostPath       string    `json:"data_host_path"`
	Port               string    `json:"port"`
	LastStarted        time.Time `json:"last_started"`
}

// SaveContext performs an atomic write of the Docker context to the hidden file.
func SaveContext(ctx DockerContext) error {
	path, err := GetContextPath()
	if err != nil {
		return err
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

// GetContextPath returns the absolute path to the hidden Docker context file.
func GetContextPath() (string, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	return filepath.Join(gscHome, settings.DockerContextFileName), nil
}
