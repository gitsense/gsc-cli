/**
 * Component: Claude Code Execution Manager
 * Block-UUID: a4b8d3cb-4731-48ce-af95-7e298fece72a
 * Parent-UUID: 8d3623a2-f00f-40e1-82d6-efeb9c33ad91
 * Version: 1.2.0
 * Description: Switched to '--output-format json' for reliable parsing, added a pre-flight check for the 'claude' binary, and improved ancestry logic to handle new chats (parentID=0).
 * Language: Go
 * Created-at: 2026-03-22T05:40:33.097Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), Gemini 3 Flash (v1.0.3), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0)
 */


package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	exec_internal "github.com/gitsense/gsc-cli/internal/exec"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ExecuteChat is the main entry point for executing a Claude Code chat session.
func ExecuteChat(chatUUID string, parentID int64, userMessage string) error {
	startTime := time.Now()

	// 1. Pre-flight Check: Ensure 'claude' binary is in PATH
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH. Please install Claude Code CLI first")
	}

	// 2. Resolve GSC_HOME
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// 3. Open Databases
	chatDBPath := settings.GetChatDatabasePath(gscHome)
	chatDB, err := db.OpenDB(chatDBPath)
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer db.CloseDB(chatDB)

	metricsDB, err := OpenMetricsDB()
	if err != nil {
		return fmt.Errorf("failed to open metrics database: %w", err)
	}
	defer metricsDB.Close()

	// 4. Retrieve Chat ID from UUID
	chat, err := db.FindChatByUUID(chatDB, chatUUID)
	if err != nil {
		return fmt.Errorf("failed to find chat: %w", err)
	}
	if chat == nil {
		return fmt.Errorf("chat not found")
	}

	// 5. Retrieve Messages and Filter by Ancestry
	allMessages, err := db.GetMessagesRecursive(chatDB, chat.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve messages: %w", err)
	}

	// Filter messages to only include ancestors of the parentID (Fork-safe)
	contextMessages, err := getAncestors(allMessages, parentID)
	if err != nil {
		return fmt.Errorf("failed to filter message ancestry: %w", err)
	}

	// 6. Setup Chat Directory
	chatDir := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeChatsDirRelPath, chatUUID)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %w", err)
	}

	// 7. Reconstruct File-Based State
	archiveSettings := Settings{
		ChunkSize: settings.DefaultClaudeChunkSize,
		MaxFiles:  settings.DefaultClaudeMaxFiles,
	}
	
	_, err = SyncArchive(chatDir, contextMessages, archiveSettings)
	if err != nil {
		return fmt.Errorf("failed to sync archive: %w", err)
	}

	// 8. Write User Message
	userMsgPath := filepath.Join(chatDir, "user-message.txt")
	if err := os.WriteFile(userMsgPath, []byte(userMessage), 0644); err != nil {
		return fmt.Errorf("failed to write user message: %w", err)
	}

	// 9. Prepare CLAUDE.md
	// Merge project CLAUDE.md (if exists) with our protocol template
	if err := prepareClaudeMD(chatDir, gscHome); err != nil {
		logger.Warning("Failed to prepare CLAUDE.md", "error", err)
	}

	// 10. Prepare System Prompt
	systemPromptPath := filepath.Join(chatDir, "messages", "system-prompt.md")
	defaultPrompt := "You are a helpful coding assistant." // Fallback

	// Try to load the bootstrapped coding-assistant.md template
	templatePath := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeTemplatesDirRelPath, "coding-assistant.md")
	if data, err := os.ReadFile(templatePath); err == nil {
		defaultPrompt = string(data)
		logger.Debug("Loaded coding-assistant.md template")
	} else {
		logger.Debug("coding-assistant.md not found, using default prompt", "error", err)
	}

	if _, err := os.Stat(systemPromptPath); os.IsNotExist(err) {
		if err := os.WriteFile(systemPromptPath, []byte(defaultPrompt), 0644); err != nil {
			return fmt.Errorf("failed to write system prompt: %w", err)
		}
	}

	// 11. Execute Claude Code CLI
	// We use '--output-format json' for reliable parsing of the final result and metrics.
	
	// 10.5. Build File List for Bulk Read Strategy
	// We explicitly list all context files in the prompt to ensure Claude reads them in the first turn.
	msgDir := filepath.Join(chatDir, "messages")
	var contextFiles []string
	
	entries, err := os.ReadDir(msgDir)
	if err != nil {
		return fmt.Errorf("failed to read messages directory: %w", err)
	}
	
	for _, entry := range entries {
		name := entry.Name()
		// Include active, archives, and context files. Exclude system-prompt.md (loaded via flag).
		if name == "messages-active.json" || strings.HasPrefix(name, "messages-archive-") || strings.HasPrefix(name, "context-") {
			contextFiles = append(contextFiles, filepath.Join("messages", name))
		}
	}
	
	sort.Strings(contextFiles)
	filesListStr := strings.Join(contextFiles, ", ")
	
	prompt := fmt.Sprintf("Read the user message in user-message.txt. IMPORTANT: First, read all context files: [%s]. Follow the protocol in CLAUDE.md.", filesListStr)
	
	flags := []string{
		"-p", fmt.Sprintf("%q", prompt),
		"--append-system-prompt-file", "./messages/system-prompt.md",
		"--allowedTools", "Read",
		"--output-format", "json",
	}

	// Create executor
	executor := exec_internal.NewExecutor("claude "+strings.Join(flags, " "), exec_internal.ExecFlags{}, chatDir, nil)

	// Run
	result, err := executor.Run()
	if err != nil {
		logger.Error("Claude CLI execution failed", "error", err)
	}

	// 12. Parse Metrics from Output
	// With '--output-format json', the entire output is a single JSON object.
	var finalResponse ClaudeResponse
	if err := json.Unmarshal([]byte(result.Output), &finalResponse); err != nil {
		logger.Error("Failed to parse Claude JSON response", "error", err)
	}

	// 13. Save Metrics
	duration := time.Since(startTime)
	if finalResponse.SessionID != "" {
		if err := InsertCompletion(
			metricsDB,
			chatUUID,
			0, // We don't have the new message ID yet (Node.js creates it)
			finalResponse.SessionID,
			"claude-code", // Model placeholder
			finalResponse.Usage,
			finalResponse.Cost,
			int(duration.Milliseconds()),
			result.Output,
			result.ExitCode,
		); err != nil {
			logger.Error("Failed to save completion metrics", "error", err)
		}

		if err := UpsertSession(
			metricsDB,
			finalResponse.SessionID,
			chatUUID,
			finalResponse.Usage,
			finalResponse.Cost,
		); err != nil {
			logger.Error("Failed to upsert session metrics", "error", err)
		}
	}

	return nil
}

// prepareClaudeMD merges the project's CLAUDE.md with the GitSense protocol.
func prepareClaudeMD(chatDir string, gscHome string) error {
	// 1. Resolve Project Root and Read Content
	projectRoot, err := git.FindProjectRoot()
	if err != nil {
		logger.Warning("Failed to find git project root, falling back to CWD", "error", err)
		projectRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
	}

	projectClaudeMD := filepath.Join(projectRoot, "CLAUDE.md")
	
	var projectContent string
	if data, err := os.ReadFile(projectClaudeMD); err == nil {
		projectContent = string(data)
	}

	// 2. Read GitSense Protocol Template
	templatePath := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeTemplatesDirRelPath, "claude_template.md")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read protocol template: %w", err)
	}

	// 3. Merge
	var finalContent string
	if projectContent != "" {
		finalContent = projectContent + "\n\n--- GITSENSE PROTOCOL ---\n\n" + string(templateContent)
	} else {
		finalContent = string(templateContent)
	}

	// 4. Write to Chat Directory
	destPath := filepath.Join(chatDir, "CLAUDE.md")
	return os.WriteFile(destPath, []byte(finalContent), 0644)
}

// getAncestors retrieves the list of messages from the root up to the target ID.
func getAncestors(allMessages []db.Message, targetID int64) ([]db.Message, error) {
	if targetID == 0 {
		return []db.Message{}, nil
	}

	// Create a map for O(1) lookup
	msgMap := make(map[int64]db.Message)
	for _, m := range allMessages {
		msgMap[m.ID] = m
	}

	var ancestors []db.Message
	currentID := targetID

	for currentID != 0 {
		msg, ok := msgMap[currentID]
		if !ok {
			return nil, fmt.Errorf("message with ID %d not found in message map", currentID)
		}
		ancestors = append([]db.Message{msg}, ancestors...)
		currentID = msg.ParentID
	}

	return ancestors, nil
}
