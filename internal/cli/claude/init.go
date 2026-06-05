/**
 * Component: Claude Code Init Command
 * Block-UUID: e4a1ecfc-ccd9-40a0-acf3-35b07e307ae8
 * Parent-UUID: 4b202df9-431f-4c3c-a2a2-f47deda7d838
 * Version: 1.0.8
 * Description: Updated to use the exported TemplateFS from pkg/settings to resolve embed path restrictions.
 * Language: Go
 * Created-at: 2026-03-24T14:17:58.340Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), Gemini 3 Flash (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), GLM-4.7 (v1.0.7), GLM-4.7 (v1.0.8)
 */


package claude

import (
	claudeint "github.com/gitsense/gsc-cli/internal/claude"
	"encoding/json"
	"fmt"
	"os/exec"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the Claude Code environment",
	Long: `Initializes the directory structure and configuration files required for the 
Claude Code CLI integration. This creates the necessary folders for templates, chat sessions, 
and the metrics database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. Resolve GSC_HOME
		gscHome, err := settings.GetGSCHome(false)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		// 2. Define Directory Paths
		claudeRoot := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath)
		templatesDir := filepath.Join(gscHome, settings.ClaudeTemplatesPath)
		chatsDir := filepath.Join(claudeRoot, settings.ClaudeChatsDirRelPath)

		// 3. Create Directories
		logger.Info("Creating Claude Code directory structure...")
		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			return fmt.Errorf("failed to create templates directory: %w", err)
		}
		if err := os.MkdirAll(chatsDir, 0755); err != nil {
			return fmt.Errorf("failed to create chats directory: %w", err)
		}

		// 4. Bootstrap Template
		templateDest := filepath.Join(templatesDir, "claude_template.md")
		if _, err := os.Stat(templateDest); os.IsNotExist(err) {
			logger.Info("Bootstrapping Claude API protocol template...")
			// Use the exported TemplateFS from pkg/settings
			// The path is relative to the 'templates/' root defined in settings.go
			data, err := settings.TemplateFS.ReadFile("templates/claude/claude_template.md")
			if err != nil {
				return fmt.Errorf("failed to read embedded template: %w", err)
			}
			if err := os.WriteFile(templateDest, data, 0644); err != nil {
				return fmt.Errorf("failed to write template file: %w", err)
			}
		} else {
			logger.Info("Template already exists, skipping.")
		}

		// 5. Create Default Settings File
		settingsPath := filepath.Join(claudeRoot, settings.ClaudeSettingsFileName)
		if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
			defaultSettings := claudeint.Settings{
				ChunkSize: settings.DefaultClaudeChunkSize,
				MaxFiles:  settings.DefaultClaudeMaxFiles,
				Model:     settings.DefaultClaudeModel,
			}
			data, err := json.MarshalIndent(defaultSettings, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal settings: %w", err)
			}
			if err := os.WriteFile(settingsPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write settings file: %w", err)
			}
			logger.Info("Created default settings", "path", settingsPath)
		} else {
			logger.Info("Settings file already exists", "path", settingsPath)
		}

		// 6. Check Claude Authentication Status
		if _, err := exec.LookPath("claude"); err == nil {
			cmd := exec.Command("claude", "auth", "status")
			if err := cmd.Run(); err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
					logger.Warning("You are not authenticated with Claude Code CLI. You will not be able to use your Claude Code subscription to chat complete with GitSense Chat. Please run 'claude' to authenticate.")
				}
			}
		} else {
			logger.Debug("Claude CLI not found in PATH, skipping auth check")
		}

		logger.Success("Claude Code environment initialized successfully", "path", claudeRoot)
		return nil
	},
}
