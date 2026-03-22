/**
 * Component: Claude Code Execution Manager
 * Block-UUID: 38d1f519-4f95-472c-aac4-7a335dd89226
 * Parent-UUID: 0f4993b2-5a87-4d64-b97c-82108af0616f
 * Version: 1.0.3
 * Description: Fixed GetGSCHome calls to handle the error return value and removed unused variable.
 * Language: Go
 * Created-at: 2026-03-22T04:31:43.119Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), Gemini 3 Flash (v1.0.3)
 */


package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/exec"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ExecuteChat is the main entry point for executing a Claude Code chat session.
func ExecuteChat(chatUUID string, parentID int64, userMessage string) error {
	startTime := time.Now()

	// 1. Resolve GSC_HOME
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// 2. Open Databases
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

	// 3. Retrieve Chat ID from UUID
	chat, err := db.FindChatByUUID(chatDB, chatUUID)
	if err != nil {
		return fmt.Errorf("failed to find chat: %w", err)
	}
	if chat == nil {
		return fmt.Errorf("chat not found")
	}

	// 4. Retrieve Messages and Filter by Ancestry
	allMessages, err := db.GetMessagesRecursive(chatDB, chat.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve messages: %w", err)
	}

	// Filter messages to only include ancestors of the parentID (Fork-safe)
	contextMessages, err := getAncestors(allMessages, parentID)
	if err != nil {
		return fmt.Errorf("failed to filter message ancestry: %w", err)
	}

	// 5. Setup Chat Directory
	chatDir := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeChatsDirRelPath, chatUUID)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %w", err)
	}

	// 6. Reconstruct File-Based State
	archiveSettings := Settings{
		ChunkSize: settings.DefaultClaudeChunkSize,
		MaxFiles:  settings.DefaultClaudeMaxFiles,
	}
	
	_, err = SyncArchive(chatDir, contextMessages, archiveSettings)
	if err != nil {
		return fmt.Errorf("failed to sync archive: %w", err)
	}

	// 7. Write User Message
	userMsgPath := filepath.Join(chatDir, "user-message.txt")
	if err := os.WriteFile(userMsgPath, []byte(userMessage), 0644); err != nil {
		return fmt.Errorf("failed to write user message: %w", err)
	}

	// 8. Prepare CLAUDE.md
	// Merge project CLAUDE.md (if exists) with our protocol template
	if err := prepareClaudeMD(chatDir, gscHome); err != nil {
		logger.Warning("Failed to prepare CLAUDE.md", "error", err)
	}

	// 9. Prepare System Prompt
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

	// 10. Execute Claude Code CLI
	// We change directory to chatDir so Claude sees it as the project root.
	// We use the executor to run the command.
	
	// Construct the prompt
	prompt := "Read the user message from user-message.txt and respond based on the conversation history in the messages/ directory."
	
	// Construct flags
	flags := []string{
		"-p", prompt,
		"--append-system-prompt-file", "./messages/system-prompt.md",
		"--allowedTools", "Read",
		"--output-format", "stream-json",
	}

	// Create executor
	// We set Workdir to chatDir
	executor := exec.NewExecutor("claude "+strings.Join(flags, " "), exec.ExecFlags{}, chatDir, nil)

	// Run
	// The executor will stream stdout to os.Stdout (which is piped to Node.js)
	// and stderr to os.Stderr.
	result, err := executor.Run()
	if err != nil {
		logger.Error("Claude CLI execution failed", "error", err)
		// We still want to save metrics if possible, but the output might be incomplete.
	}

	// 11. Parse Metrics from Output
	// The output is a stream of JSON objects. The last one usually contains the summary.
	// We need to scan the output buffer to find the final JSON object.
	var finalResponse ClaudeResponse
	scanner := bufio.NewScanner(bytes.NewReader([]byte(result.Output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "{") {
			var resp ClaudeResponse
			if err := json.Unmarshal([]byte(line), &resp); err == nil {
				if resp.SessionID != "" {
					finalResponse = resp
				}
			}
		}
	}

	// 12. Save Metrics
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
	// Use git to find the true project root, not just the current working directory.
	projectRoot, err := git.FindProjectRoot()
	if err != nil {
		// Fallback to CWD if git fails (e.g., not in a git repo)
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
// This ensures we only include messages that are actually in the history path,
// handling forks correctly.
func getAncestors(allMessages []db.Message, targetID int64) ([]db.Message, error) {
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
		// Prepend to maintain chronological order
		ancestors = append([]db.Message{msg}, ancestors...)
		currentID = msg.ParentID
	}

	return ancestors, nil
}
