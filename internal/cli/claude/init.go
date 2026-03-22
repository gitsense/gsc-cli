/**
 * Component: Claude Code Init Command
 * Block-UUID: d4d07cca-02aa-4296-884f-29e5e2f8f0a9
 * Parent-UUID: 33f74e8e-151f-4b47-9f75-f78af9d9ec18
 * Version: 1.0.3
 * Description: Updated to use the exported TemplateFS from pkg/settings to resolve embed path restrictions.
 * Language: Go
 * Created-at: 2026-03-22T04:33:08.400Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), Gemini 3 Flash (v1.0.3)
 */


package claude

import (
	"fmt"
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
		gscHome, err := settings.GetGSCHome(true)
		if err != nil {
			return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
		}

		// 2. Define Directory Paths
		claudeRoot := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath)
		templatesDir := filepath.Join(claudeRoot, settings.ClaudeTemplatesDirRelPath)
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

		logger.Success("Claude Code environment initialized successfully", "path", claudeRoot)
		return nil
	},
}
