/*
 * Component: Profile Manager
 * Block-UUID: 2a14ba86-c102-46cb-9e09-b2c32a8d9044
 * Parent-UUID: 1ea81347-2c64-48eb-99a1-040d5eaba503
 * Version: 1.4.0
 * Description: Logic to manage Context Profiles, including listing, creating, deleting, activating, and deactivating profiles. Updated log messages in DeactivateProfile to use 'clear' terminology for consistency with the new CLI command. Refactored all logger calls to use structured Key-Value pairs instead of format strings.
 * Language: Go
 * Created-at: 2026-02-03T02:05:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
 */


package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

// ListProfiles returns a list of all available profiles in the .gitsense/profiles directory.
func ListProfiles() ([]Profile, error) {
	profilesDir, err := resolveProfilesDir()
	if err != nil {
		return nil, err
	}

	// Check if directory exists
	if _, err := os.Stat(profilesDir); os.IsNotExist(err) {
		return []Profile{}, nil
	}

	// Read directory entries
	entries, err := os.ReadDir(profilesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read profiles directory: %w", err)
	}

	var profiles []Profile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Check for .json extension
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Extract profile name (remove .json)
		name := entry.Name()[:len(entry.Name())-5]

		// Load profile
		profile, err := LoadProfile(name)
		if err != nil {
			logger.Warning("Failed to load profile", "name", name, "error", err)
			continue
		}

		profiles = append(profiles, *profile)
	}

	// Sort profiles alphabetically by name
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

// CreateProfile creates a new profile with the specified name, description, aliases, and settings.
func CreateProfile(name string, description string, aliases []string, settings ProfileSettings) error {
	// Validate name
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Validate aliases uniqueness
	for _, alias := range aliases {
		if err := ValidateAliasUniqueness(alias, ""); err != nil {
			return err
		}
	}

	// Check if profile already exists
	_, err := LoadProfile(name)
	if err == nil {
		return fmt.Errorf("profile '%s' already exists", name)
	}

	// Create profile struct
	profile := Profile{
		Name:        name,
		Description: description,
		Aliases:     aliases,
		Settings:    settings,
	}

	// Save profile
	return SaveProfile(&profile)
}

// DeleteProfile deletes a profile by name.
// If the profile is currently active, it will be deactivated.
func DeleteProfile(name string) error {
	profilePath, err := resolveProfilePath(name)
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		return fmt.Errorf("profile '%s' not found", name)
	}

	// Delete the file
	if err := os.Remove(profilePath); err != nil {
		return fmt.Errorf("failed to delete profile file: %w", err)
	}

	// Check if it was the active profile and deactivate if necessary
	config, err := LoadConfig()
	if err != nil {
		logger.Warning("Failed to load config to check active profile", "error", err)
	} else {
		if config.ActiveProfile == name {
			config.ActiveProfile = ""
			if err := SaveConfig(config); err != nil {
				logger.Warning("Failed to deactivate profile in config", "error", err)
			} else {
				logger.Info("Deactivated profile as it was deleted", "name", name)
			}
		}
	}

	logger.Success("Profile deleted successfully", "name", name)
	return nil
}

// ShowProfile returns the details of a specific profile.
func ShowProfile(name string) (*Profile, error) {
	return LoadProfile(name)
}

// SetActiveProfile sets the active profile in the configuration.
func SetActiveProfile(name string) error {
	// Resolve profile (supports aliases)
	profile, err := ResolveProfile(name)
	if err != nil {
		return fmt.Errorf("failed to resolve profile '%s': %w", name, err)
	}

	// Load config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set active profile
	config.ActiveProfile = profile.Name

	// Save config
	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Success("Active profile set", "name", profile.Name)
	return nil
}

// DeactivateProfile clears the active profile in the configuration.
func DeactivateProfile() error {
	// Load config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if there is even an active profile
	if config.ActiveProfile == "" {
		logger.Info("No active profile to clear")
		return nil
	}

	// Clear active profile
	config.ActiveProfile = ""

	// Save config
	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Success("Active profile cleared")
	return nil
}

// GetActiveProfileName returns the name of the currently active profile.
func GetActiveProfileName() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	return config.ActiveProfile, nil
}

// ValidateAliasUniqueness checks if an alias is already used by another profile.
// excludeProfileName is used to skip the check when updating a profile (so it can keep its own alias).
func ValidateAliasUniqueness(alias string, excludeProfileName string) error {
	if alias == "" {
		return nil // Empty alias is valid (though useless)
	}

	profiles, err := ListProfiles()
	if err != nil {
		return fmt.Errorf("failed to list profiles for validation: %w", err)
	}

	for _, profile := range profiles {
		// Skip the profile we are currently updating
		if profile.Name == excludeProfileName {
			continue
		}

		// Check if alias matches
		for _, existingAlias := range profile.Aliases {
			if strings.EqualFold(existingAlias, alias) {
				return fmt.Errorf("alias '%s' is already used by profile '%s'", alias, profile.Name)
			}
		}
	}

	return nil
}

// ResolveProfile attempts to find a profile by name or alias.
// It first tries an exact name match, then scans all profiles for an alias match.
func ResolveProfile(input string) (*Profile, error) {
	if input == "" {
		return nil, fmt.Errorf("profile name or alias cannot be empty")
	}

	// 1. Try Exact Name Match
	profile, err := LoadProfile(input)
	if err == nil {
		return profile, nil
	}

	// 2. Try Alias Match
	profiles, err := ListProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles for alias resolution: %w", err)
	}

	for _, p := range profiles {
		for _, alias := range p.Aliases {
			if strings.EqualFold(alias, input) {
				return &p, nil
			}
		}
	}

	return nil, fmt.Errorf("profile '%s' not found", input)
}

// resolveProfilesDir constructs the absolute path to the profiles directory.
func resolveProfilesDir() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	profilesDir := filepath.Join(root, settings.GitSenseDir, ProfilesDirName)
	return profilesDir, nil
}
