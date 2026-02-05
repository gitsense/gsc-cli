/**
 * Component: Query Configuration Manager
 * Block-UUID: 669a4b74-5db3-4c8a-99ff-1d1786758846
 * Parent-UUID: d2decfd1-5a1b-43f0-9068-95271f235f53
 * Version: 2.4.0
 * Description: Manages the .gitsense/config.json file and profile loading. Updated GlobalSettings to include Scope, updated mergeConfig to handle scope merging, and integrated .gitsense-map loading into GetEffectiveConfig to ensure project-level scope is considered. Refactored all logger calls to use structured Key-Value pairs instead of format strings. Updated to support professional CLI output: demoted routine Info and Success logs to Debug level to prevent duplicate output with the CLI layer and enable quiet-by-default behavior.
 * Language: Go
 * Created-at: 2026-02-02T18:48:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0)
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
	// ProfilesDirName is the name of the directory containing profile definitions
	ProfilesDirName = "profiles"
)

// QueryConfig represents the configuration stored in .gitsense/config.json.
// Updated to include the active profile and global settings.
type QueryConfig struct {
	ActiveProfile string                 `json:"active_profile"` // The name of the currently active profile
	Global        GlobalSettings          `json:"global"`         // Global settings (fallback if no profile is active)
	Query         QuerySettings           `json:"query"`          // Query command settings
	RG            RGSettings              `json:"rg"`             // Ripgrep command settings
	Aliases       map[string]QueryAlias   `json:"aliases"`        // Saved query aliases (Phase 5)
	History       []string                `json:"history"`        // Recent query history (Phase 5)
}

// LoadConfig loads the query configuration from .gitsense/config.json.
// If the file does not exist, it returns a new, empty configuration.
func LoadConfig() (*QueryConfig, error) {
	configPath, err := resolveConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		logger.Debug("Config file not found, creating new config", "path", configPath)
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

	logger.Debug("Config loaded successfully", "path", configPath)
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

	logger.Debug("Config saved successfully", "path", configPath)
	return nil
}

// GetEffectiveConfig loads the configuration and merges the active profile if one is set.
// It also integrates project-level scope from .gitsense-map if no profile scope is defined.
// This is the primary function that commands should use to get their settings.
// Precedence: Profile Settings > Global Settings > .gitsense-map.
func GetEffectiveConfig() (*QueryConfig, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	// If no active profile, check for .gitsense-map and return
	if config.ActiveProfile == "" {
		// If no profile scope, try to load project map
		if config.Global.Scope == nil {
			projectMap, err := LoadGitSenseMap()
			if err == nil && projectMap != nil {
				config.Global.Scope = projectMap
				logger.Debug("Applied .gitsense-map to global config")
			}
		}
		return config, nil
	}

	// Load the active profile
	profile, err := LoadProfile(config.ActiveProfile)
	if err != nil {
		logger.Warning("Failed to load active profile, using base config", "profile", config.ActiveProfile, "error", err)
		// Fallback to checking .gitsense-map if profile loading fails
		if config.Global.Scope == nil {
			projectMap, err := LoadGitSenseMap()
			if err == nil && projectMap != nil {
				config.Global.Scope = projectMap
			}
		}
		return config, nil
	}

	// Merge profile settings into the config
	merged := mergeConfig(config, profile)

	// If merged config has no scope, check .gitsense-map
	if merged.Global.Scope == nil {
		projectMap, err := LoadGitSenseMap()
		if err == nil && projectMap != nil {
			merged.Global.Scope = projectMap
			logger.Debug("Applied .gitsense-map to merged config")
		}
	}

	return merged, nil
}

// LoadProfile loads a specific profile JSON file from the .gitsense/profiles directory.
func LoadProfile(name string) (*Profile, error) {
	profilePath, err := resolveProfilePath(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to parse profile JSON: %w", err)
	}

	return &profile, nil
}

// SaveProfile saves a profile to the .gitsense/profiles directory.
func SaveProfile(profile *Profile) error {
	profilePath, err := resolveProfilePath(profile.Name)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(profilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(profilePath, data, 0644); err != nil {
		return err
	}

	logger.Debug("Profile saved successfully", "name", profile.Name)
	return nil
}

// mergeConfig merges profile settings into the base configuration.
// Profile settings take precedence over global settings.
func mergeConfig(base *QueryConfig, profile *Profile) *QueryConfig {
	merged := *base // Copy base

	// Merge Global Settings
	if profile.Settings.Global.DefaultDatabase != "" {
		merged.Global.DefaultDatabase = profile.Settings.Global.DefaultDatabase
	}
	// Merge Scope: Profile scope overrides Base scope
	if profile.Settings.Global.Scope != nil {
		merged.Global.Scope = profile.Settings.Global.Scope
	}

	// Merge Query Settings
	if profile.Settings.Query.DefaultField != "" {
		merged.Query.DefaultField = profile.Settings.Query.DefaultField
	}
	if profile.Settings.Query.DefaultFormat != "" {
		merged.Query.DefaultFormat = profile.Settings.Query.DefaultFormat
	}

	// Merge RG Settings
	if profile.Settings.RG.DefaultFormat != "" {
		merged.RG.DefaultFormat = profile.Settings.RG.DefaultFormat
	}
	if profile.Settings.RG.DefaultContext != 0 {
		merged.RG.DefaultContext = profile.Settings.RG.DefaultContext
	}

	return &merged
}

// NewQueryConfig creates a new, empty QueryConfig with default values.
func NewQueryConfig() *QueryConfig {
	return &QueryConfig{
		ActiveProfile: "",
		Global: GlobalSettings{
			DefaultDatabase: "",
			Scope:           nil, // No default scope
		},
		Query: QuerySettings{
			DefaultField:  "",
			DefaultFormat: "table",
		},
		RG: RGSettings{
			DefaultFormat:  "table",
			DefaultContext: 0,
		},
		Aliases: make(map[string]QueryAlias),
		History: []string{},
	}
}

// SetDefault sets a default value for a specific key in the configuration.
// NOTE: This function is deprecated in favor of profiles but kept for compatibility.
func SetDefault(key string, value string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	switch key {
	case "db":
		config.Global.DefaultDatabase = value
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
// NOTE: This function is deprecated in favor of profiles but kept for compatibility.
func ClearDefault(key string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	switch key {
	case "db":
		config.Global.DefaultDatabase = ""
	case "field":
		config.Query.DefaultField = ""
	case "format":
		config.Query.DefaultFormat = "table"
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

// resolveProfilePath constructs the absolute path to a profile JSON file.
func resolveProfilePath(name string) (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	profilePath := filepath.Join(root, settings.GitSenseDir, ProfilesDirName, name+".json")
	return profilePath, nil
}
