/**
 * Component: Workspace Info Logic
 * Block-UUID: 80793b68-5814-44f7-b37b-dbd9a507d88c
 * Parent-UUID: 6ac72f9c-1ee8-445c-8865-06e0175b762d
 * Version: 1.5.0
 * Description: Logic to gather and format workspace information for the 'gsc info' command, including active profiles and available databases. Added 'gsc config active' to Quick Actions for consistency with the new command name. Refactored all logger calls to use structured Key-Value pairs instead of format strings.
 * Language: Go
 * Created-at: 2026-02-05T05:24:58.365Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
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
		logger.Warning("Failed to load config", "error", err)
	} else if config.ActiveProfile != "" {
		profile, err := LoadProfile(config.ActiveProfile)
		if err != nil {
			logger.Warning("Failed to load active profile", "profile", config.ActiveProfile, "error", err)
		} else {
			info.ActiveProfile = profile
		}
	}

	// 3. List Available Databases
	dbs, err := ListDatabases(ctx)
	if err != nil {
		logger.Warning("Failed to list databases", "error", err)
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

	// Use the extracted header function
	// We need a QueryConfig to pass to it, so we construct a minimal one or load it
	config, err := GetEffectiveConfig()
	if err != nil {
		// Fallback if config fails to load
		sb.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
		sb.WriteString("║                    GitSense Workspace Info                     ║\n")
		sb.WriteString("╚════════════════════════════════════════════════════════════════╝\n")
		sb.WriteString("\n")
		sb.WriteString("Active Profile:  (unknown)\n")
	} else {
		sb.WriteString(FormatWorkspaceHeader(config))
	}

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
	sb.WriteString("  • Show active:     gsc config active\n")
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

// FormatWorkspaceHeader returns the prominent workspace header box.
// This is reused by query and rg commands to show context.
func FormatWorkspaceHeader(config *QueryConfig) string {
	var sb strings.Builder

	sb.WriteString("╔════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    GitSense Workspace Info                     ║\n")
	sb.WriteString("╚════════════════════════════════════════════════════════════════╝\n")
	sb.WriteString("\n")

	if config.ActiveProfile != "" {
		sb.WriteString(fmt.Sprintf("Active Profile:  %s\n", config.ActiveProfile))
		sb.WriteString(fmt.Sprintf("├─ Database:    %s\n", config.Global.DefaultDatabase))
		sb.WriteString(fmt.Sprintf("├─ Field:       %s\n", config.Query.DefaultField))
		sb.WriteString(fmt.Sprintf("├─ Format:      %s\n", config.Query.DefaultFormat))

		// Display Scope Configuration
		if config.Global.Scope != nil {
			sb.WriteString(fmt.Sprintf("├─ Scope Inc:   %s\n", strings.Join(config.Global.Scope.Include, ", ")))
			sb.WriteString(fmt.Sprintf("└─ Scope Exc:   %s\n", strings.Join(config.Global.Scope.Exclude, ", ")))
		} else {
			sb.WriteString("└─ Scope:       (default - all tracked files)\n")
		}
	} else {
		sb.WriteString("Active Profile:  (none)\n")
		sb.WriteString("  Run 'gsc config use <name>' to activate a profile.\n")
		sb.WriteString("  Run 'gsc config context list' to list available profiles.\n")
	}
	sb.WriteString("\n")

	return sb.String()
}
