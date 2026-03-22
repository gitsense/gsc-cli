/**
 * Component: Claude Code Init Command
 * Block-UUID: b25aae1a-d1b6-4e2d-8108-16918fff52b1
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc claude init' command to bootstrap the Claude Code environment, including directory structure, templates, and the metrics database.
 * Language: Go
 * Created-at: 2026-03-22T03:39:12.456Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package claude

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

//go:embed ../../pkg/settings/templates/claude/claude_template.md
var templateFS embed.FS

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
		metricsDBPath := filepath.Join(claudeRoot, settings.ClaudeMetricsDBName)

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
			data, err := templateFS.ReadFile("claude_template.md")
			if err != nil {
				return fmt.Errorf("failed to read embedded template: %w", err)
			}
			if err := os.WriteFile(templateDest, data, 0644); err != nil {
				return fmt.Errorf("failed to write template file: %w", err)
			}
		} else {
			logger.Info("Template already exists, skipping.")
		}

		// 5. Initialize Metrics Database
		// Note: Schema initialization will be handled by the metrics package upon first open
		if _, err := os.Stat(metricsDBPath); os.IsNotExist(err) {
			logger.Info("Initializing metrics database...")
			// Create an empty file to reserve the path
			file, err := os.Create(metricsDBPath)
			if err != nil {
				return fmt.Errorf("failed to create metrics database file: %w", err)
			}
			file.Close()
		} else {
			logger.Info("Metrics database already exists, skipping.")
		}

		logger.Success("Claude Code environment initialized successfully", "path", claudeRoot)
		return nil
	},
}
