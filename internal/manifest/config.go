/*
 * Component: Query Configuration Manager
 * Block-UUID: 50037455-9f00-423d-ad19-9ebc374cad70
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Manages the .gitsense/config.json file, handling loading, saving, and updating of query and ripgrep defaults.
 * Language: Go
 * Created-at: 2026-02-02T18:48:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

const (
	// ConfigFileName is the name of the configuration file
	ConfigFileName = "config.json"
)

// LoadConfig loads the query configuration from .gitsense/config.json.
// If the file does not exist, it returns a new, empty configuration.
func LoadConfig() (*QueryConfig, error) {
	configPath, err := resolveConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Info("Config file not found, creating new config", "path", configPath)
		return NewQueryConfig(), nil
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		logger.Error("Failed to read config file", "path", configPath, "error", err)
		return nil, err
	}

	// Parse JSON
	var config QueryConfig
	if err := json.Unmarshal(data, &config); err != nil {
		logger.Error("Failed to parse config JSON", "path", configPath, "error", err)
		return nil, err
	}

	logger.Info("Config loaded successfully", "path", configPath)
	return &config, nil
}

// SaveConfig saves the query configuration to .gitsense/config.json.
func SaveConfig(config *QueryConfig) error {
	configPath, err := resolveConfigPath()
	if err != nil {
		return fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Error("Failed to create config directory", "dir", dir, "error", err)
		return err
	}

	// Marshal JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal config JSON", "error", err)
		return err
	}

	// Write file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		logger.Error("Failed to write config file", "path", configPath, "error", err)
		return err
	}

	logger.Success("Config saved successfully", "path", configPath)
	return nil
}

// NewQueryConfig creates a new, empty QueryConfig with default values.
func NewQueryConfig() *QueryConfig {
	return &QueryConfig{
		Query: struct {
			DefaultDatabase string                 `json:"default_database"`
			DefaultField    string                 `json:"default_field"`
			DefaultFormat   string                 `json:"default_format"`
			Aliases         map[string]QueryAlias  `json:"aliases"`
			History         []string               `json:"history"`
		}{
			DefaultDatabase: "",
			DefaultField:    "",
			DefaultFormat:   "table",
			Aliases:         make(map[string]QueryAlias),
			History:         []string{},
		},
		RG: struct {
			DefaultDatabase string `json:"default_database"`
			DefaultFormat   string `json:"default_format"`
			DefaultContext  int    `json:"default_context"`
		}{
			DefaultDatabase: "",
			DefaultFormat:   "table",
			DefaultContext:  0,
		},
	}
}

// SetDefault sets a default value for a specific key in the configuration.
// Supported keys: "db", "field", "format".
func SetDefault(key string, value string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	switch key {
	case "db":
		config.Query.DefaultDatabase = value
		config.RG.DefaultDatabase = value // Sync with RG
	case "field":
		config.Query.DefaultField = value
	case "format":
		config.Query.DefaultFormat = value
	default:
		return fmt.Errorf("unknown default key: %s", key)
	}

	return SaveConfig(config)
}

// ClearDefault clears a default value for a specific key.
func ClearDefault(key string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	switch key {
	case "db":
		config.Query.DefaultDatabase = ""
		config.RG.DefaultDatabase = "" // Sync with RG
	case "field":
		config.Query.DefaultField = ""
	case "format":
		config.Query.DefaultFormat = "table" // Reset to default
	default:
		return fmt.Errorf("unknown default key: %s", key)
	}

	return SaveConfig(config)
}

// resolveConfigPath constructs the absolute path to the config.json file.
func resolveConfigPath() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	configPath := filepath.Join(root, settings.GitSenseDir, ConfigFileName)
	return configPath, nil
}
