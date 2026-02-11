/**
 * Component: Workspace Info Logic
 * Block-UUID: 78f27e29-99b3-426a-84f9-f1c7b31c68fa
 * Parent-UUID: 94604752-7799-4aeb-a15b-72631c2e8294
 * Version: 1.8.0
 * Description: Logic to gather and format workspace information for the 'gsc info' command. Updated to hide profile and config features from the user interface. Removed 'Active Profile' and 'Available Profiles' sections from output. Added hint for database import. Updated 'FormatWorkspaceHeader' to remove profile context. Implementation logic for profiles is retained internally but not exposed to the user.
 * Language: Go
 * Created-at: 2026-02-05T05:24:58.365Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0)
 */


package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yourusername/gsc-cli/internal/git"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

// WorkspaceInfo represents the complete state of the GitSense Chat workspace.
// INTERNAL: Profile fields are retained for internal logic but hidden from UI.
type WorkspaceInfo struct {
	ActiveProfile    *Profile       `json:"active_profile,omitempty"`
	AvailableProfiles []Profile     `json:"available_profiles"`
	AvailableDBs     []DatabaseInfo `json:"available_databases"`
	ProjectRoot      string         `json:"project_root"`
	GitSenseDir      string         `json:"gitsense_dir"`
	RegistryPath     string         `json:"registry_path"`
}

// GetWorkspaceInfo gathers all relevant information about the current workspace.
// INTERNAL: This function still loads profiles internally to support hidden features,
// but the output formatters will suppress this information from the user.
func GetWorkspaceInfo(ctx context.Context) (*WorkspaceInfo, error) {
	info := &WorkspaceInfo{}

	// 1. Get Project Root
	root, err := git.FindProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find project root: %w", err)
	}
	info.ProjectRoot = root
	info.GitSenseDir = filepath.Join(root, settings.GitSenseDir)
	info.RegistryPath = filepath.Join(info.GitSenseDir, settings.RegistryFileName)

	// 2. Load Active Profile (Internal Use Only)
	config, err := LoadConfig()
	if err != nil {
		logger.Warning("Failed to load config", "error", err)
	} else if config.ActiveProfile != "" {
		profile, err := LoadProfile(config.ActiveProfile)
		if err != nil {
			logger.Warning("Failed to load active profile", "profile", config.ActiveProfile, "error", err)
		} else {
			info.ActiveProfile = profile
		}
	}

	// 3. List Available Profiles (Internal Use Only)
	profiles, err := ListProfiles()
	if err != nil {
		logger.Warning("Failed to list profiles", "error", err)
	} else {
		info.AvailableProfiles = profiles
	}

	// 4. List Available Databases
	dbs, err := ListDatabases(ctx)
	if err != nil {
		logger.Warning("Failed to list databases", "error", err)
	} else {
		info.AvailableDBs = dbs
	}

	return info, nil
}

// FormatWorkspaceInfo formats the workspace info for display.
func FormatWorkspaceInfo(info *WorkspaceInfo, format string, verbose bool, noColor bool) string {
	switch strings.ToLower(format) {
	case "json":
		return formatInfoJSON(info)

	case "table":
		return formatInfoTable(info, verbose, noColor)
	default:
		return formatInfoTable(info, verbose, noColor)
	}
}

// formatInfoJSON returns the workspace info as a JSON string.
// INTERNAL: JSON output still contains profile data for potential internal tooling use,
// but this endpoint is not advertised to users.
func formatInfoJSON(info *WorkspaceInfo) string {
	bytes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

// formatInfoTable returns the workspace info as a formatted text table.
// UPDATED: Removed 'Active Profile' and 'Available Profiles' sections to hide the feature.
func formatInfoTable(info *WorkspaceInfo, verbose bool, noColor bool) string {
	var sb strings.Builder

	// Use the extracted header function
	// We need a QueryConfig to pass to it, so we construct a minimal one or load it
	config, err := GetEffectiveConfig()
	if err != nil {
		// Fallback if config fails to load
		//sb.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
		//sb.WriteString("║                 GitSense Chat Workspace Info                   ║\n")
		//sb.WriteString("╚════════════════════════════════════════════════════════════════╝\n")
		sb.WriteString("\n")
	} else {
		sb.WriteString(FormatWorkspaceHeader(config, noColor))
	}

	// Available Databases Section
	sb.WriteString("\nAvailable Databases:\n")
	if len(info.AvailableDBs) == 0 {
		sb.WriteString("   (none)\n")
		sb.WriteString("\n")
		sb.WriteString("Hint: To add a database, use `gsc import <manifest>`\n")
	} else {
		for _, db := range info.AvailableDBs {
			marker := " "
			// INTERNAL: We still mark the active DB internally, but don't show the profile name
			if info.ActiveProfile != nil && info.ActiveProfile.Settings.Global.DefaultDatabase == db.DatabaseLabel {
				marker = "*"
			}

			sb.WriteString(fmt.Sprintf("├─ %s %-16s (%d files)", marker, db.DatabaseName, db.EntryCount))

			if verbose {
				sb.WriteString(fmt.Sprintf("\n│  Description: %s", db.Description))
				if len(db.Tags) > 0 {
					sb.WriteString(fmt.Sprintf("\n│  Tags:        [%s]", strings.Join(db.Tags, ", ")))
				}
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	// Verbose Details
	if verbose {
		sb.WriteString("\n")
		sb.WriteString("Project Information:\n")
		sb.WriteString(fmt.Sprintf("  Project Root:      %s\n", info.ProjectRoot))
		sb.WriteString(fmt.Sprintf("  GitSense Dir:      %s\n", info.GitSenseDir))
		sb.WriteString(fmt.Sprintf("  Registry:          %s\n", info.RegistryPath))
	}

	return sb.String()
}

// FormatWorkspaceHeader returns the prominent workspace header box.
// This is reused by query and rg commands to show context.
// UPDATED: Removed profile details to hide the feature from the user.
func FormatWorkspaceHeader(config *QueryConfig, noColor bool) string {
	var sb strings.Builder

	//sb.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
	//sb.WriteString("║                 GitSense Chat Workspace Info                   ║\n")
	//sb.WriteString("╚════════════════════════════════════════════════════════════════╝\n")
	//sb.WriteString("\n")
	// Profile information removed from display
	
	return sb.String()
}
