/*
 * Component: Profile Manager
 * Block-UUID: 35e82d14-d1f4-4422-b6f3-334aa72dd4dd
 * Parent-UUID: 939abc56-3b4f-4812-80bd-d3fedfca04b9
 * Version: 1.1.0
 * Description: Logic to manage Context Profiles, including listing, creating, deleting, activating, and deactivating profiles.
 * Language: Go
 * Created-at: 2026-02-03T02:05:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
			logger.Warning("Failed to load profile %s: %v", name, err)
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

// CreateProfile creates a new profile with the specified name, description, and settings.
func CreateProfile(name string, description string, settings ProfileSettings) error {
	// Validate name
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
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
		logger.Warning("Failed to load config to check active profile: %v", err)
	} else {
		if config.ActiveProfile == name {
			config.ActiveProfile = ""
			if err := SaveConfig(config); err != nil {
				logger.Warning("Failed to deactivate profile in config: %v", err)
			} else {
				logger.Info("Deactivated profile '%s' as it was deleted", name)
			}
		}
	}

	logger.Success("Profile '%s' deleted successfully", name)
	return nil
}

// ShowProfile returns the details of a specific profile.
func ShowProfile(name string) (*Profile, error) {
	return LoadProfile(name)
}

// SetActiveProfile sets the active profile in the configuration.
func SetActiveProfile(name string) error {
	// Validate profile exists
	_, err := LoadProfile(name)
	if err != nil {
		return fmt.Errorf("profile '%s' not found: %w", name, err)
	}

	// Load config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set active profile
	config.ActiveProfile = name

	// Save config
	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Success("Active profile set to '%s'", name)
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
		logger.Info("No active profile to deactivate.")
		return nil
	}

	// Clear active profile
	config.ActiveProfile = ""

	// Save config
	if err := SaveConfig(config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Success("Active profile deactivated")
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

// resolveProfilesDir constructs the absolute path to the profiles directory.
func resolveProfilesDir() (string, error) {
	root, err := git.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("failed to find project root: %w", err)
	}

	profilesDir := filepath.Join(root, settings.GitSenseDir, ProfilesDirName)
	return profilesDir, nil
}
