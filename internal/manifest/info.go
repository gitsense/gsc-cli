/*
 * Component: Workspace Info Logic
 * Block-UUID: 3e691f33-8796-4c65-a4c7-b6d03a6e8c49
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Logic to gather and format workspace information for the 'gsc info' command, including active profiles and available databases.
 * Language: Go
 * Created-at: 2026-02-03T03:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
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

// WorkspaceInfo represents the complete state of the GitSense workspace.
type WorkspaceInfo struct {
	ActiveProfile   *Profile       `json:"active_profile,omitempty"`
	AvailableDBs    []DatabaseInfo `json:"available_databases"`
	ProjectRoot     string         `json:"project_root"`
	GitSenseDir     string         `json:"gitsense_dir"`
	RegistryPath    string         `json:"registry_path"`
}

// GetWorkspaceInfo gathers all relevant information about the current workspace.
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

	// 2. Load Active Profile
	config, err := LoadConfig()
	if err != nil {
		logger.Warning("Failed to load config: %v", err)
	} else if config.ActiveProfile != "" {
		profile, err := LoadProfile(config.ActiveProfile)
		if err != nil {
			logger.Warning("Failed to load active profile '%s': %v", config.ActiveProfile, err)
		} else {
			info.ActiveProfile = profile
		}
	}

	// 3. List Available Databases
	dbs, err := ListDatabases(ctx)
	if err != nil {
		logger.Warning("Failed to list databases: %v", err)
	} else {
		info.AvailableDBs = dbs
	}

	return info, nil
}

// FormatWorkspaceInfo formats the workspace info for display.
func FormatWorkspaceInfo(info *WorkspaceInfo, format string, verbose bool) string {
	switch strings.ToLower(format) {
	case "json":
		return formatInfoJSON(info)
	case "table":
		return formatInfoTable(info, verbose)
	default:
		return formatInfoTable(info, verbose)
	}
}

// formatInfoJSON returns the workspace info as a JSON string.
func formatInfoJSON(info *WorkspaceInfo) string {
	bytes, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting JSON: %v", err)
	}
	return string(bytes)
}

// formatInfoTable returns the workspace info as a formatted text table.
func formatInfoTable(info *WorkspaceInfo, verbose bool) string {
	var sb strings.Builder

	// Header
	sb.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    GitSense Workspace Info                     ║\n")
	sb.WriteString("╚════════════════════════════════════════════════════════════════╝\n")
	sb.WriteString("\n")

	// Active Profile Section
	if info.ActiveProfile != nil {
		sb.WriteString(fmt.Sprintf("Active Profile:  %s\n", info.ActiveProfile.Name))
		sb.WriteString(fmt.Sprintf("├─ Database:    %s\n", info.ActiveProfile.Settings.Global.DefaultDatabase))
		sb.WriteString(fmt.Sprintf("├─ Field:       %s\n", info.ActiveProfile.Settings.Query.DefaultField))
		sb.WriteString(fmt.Sprintf("└─ Format:      %s\n", info.ActiveProfile.Settings.Query.DefaultFormat))
	} else {
		sb.WriteString("Active Profile:  (none)\n")
		sb.WriteString("  Run 'gsc config use <name>' to activate a profile.\n")
	}
	sb.WriteString("\n")

	// Available Databases Section
	sb.WriteString("Available Databases:\n")
	if len(info.AvailableDBs) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, db := range info.AvailableDBs {
			marker := " "
			if info.ActiveProfile != nil && info.ActiveProfile.Settings.Global.DefaultDatabase == db.Name {
				marker = "*"
			}
			
			sb.WriteString(fmt.Sprintf("├─ %s %-16s (%d files)", marker, db.Name, db.EntryCount))
			
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

	// Quick Actions
	sb.WriteString("Quick Actions:\n")
	sb.WriteString("  • Switch context:  gsc config use <name>\n")
	sb.WriteString("  • List all:        gsc config context list\n")
	sb.WriteString("  • Query:           gsc query --value <val>\n")

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
