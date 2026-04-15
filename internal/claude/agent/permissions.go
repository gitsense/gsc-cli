/**
 * Component: Agent Permissions Configuration
 * Block-UUID: 7403c8f5-d7a0-4c7b-b16c-61cb1eb5d26b
 * Parent-UUID: dab89f10-c685-4293-9419-3ec02e253aa6
 * Version: 1.1.0
 * Description: Generic permissions configuration for agent sessions including restricted command execution and file access controls.
 * Language: Go
 * Created-at: 2026-03-27T16:12:50.000Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteAgentPermissions creates a .claude/settings.json file with restricted permissions
// for agent sessions. This ensures Claude can only execute specific commands and read files.
func WriteAgentPermissions(turnDir string) error {
	// Create .claude directory
	claudeDir := filepath.Join(turnDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	// Define permissions - allow only gsc commands and Read tool
	permissions := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{
				"Bash(gsc:*)",
				"Bash(sort)",
				"Bash(head)",
				"Bash(tail)",
				"Read(*)",
			},
			"defaultMode": "default",
		},
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(permissions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal permissions: %w", err)
	}

	// Write to settings.json
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	return nil
}
