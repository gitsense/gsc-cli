/**
 * Component: Native Deployment Config Manager
 * Block-UUID: 80453de7-fe7a-4ccd-9cf3-ea86cb11f9e6
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Manages the persistence and retrieval of the native-config.json file, which tracks the state of the native Node.js installation.
 * Language: Go
 * Created-at: 2026-05-11T14:05:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ConfigFileName = "native-config.json"

// Config holds the persistent state of a native installation.
type Config struct {
	Version     string `json:"version"`
	AppDir      string `json:"app_dir"`
	DataDir     string `json:"data_dir"`
	Port        string `json:"port"`
	InstalledAt string `json:"installed_at"`
}

// GetConfigPath returns the absolute path to the native-config.json file.
func GetConfigPath(gscHome string) string {
	return filepath.Join(gscHome, ConfigFileName)
}

// LoadConfig reads the configuration from GSC_HOME.
// Returns nil, nil if the file does not exist.
func LoadConfig(gscHome string) (*Config, error) {
	path := GetConfigPath(gscHome)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read native config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse native config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig writes the configuration to GSC_HOME.
func SaveConfig(gscHome string, cfg Config) error {
	path := GetConfigPath(gscHome)
	
	// Ensure directory exists (GSC_HOME might not exist on very first run)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
