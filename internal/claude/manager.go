/**
 * Component: Claude Code Execution Manager
 * Block-UUID: e21f0d9f-22e0-43d7-af5a-82aa7d7eed8e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Orchestrates the Claude Code CLI execution, including database retrieval, state reconstruction, CLI invocation, and metrics collection.
 * Language: Go
 * Created-at: 2026-03-22T03:49:12.345Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package claude

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/exec"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ExecuteChat is the main entry point for executing a Claude Code chat session.
func ExecuteChat(chatUUID string, parentID int64, userMessage string) error {
	startTime := time.Now()

	// 1. Open Databases
	chatDBPath := settings.GetChatDatabasePath(settings.GetGSCHome(false)) // Resolve path
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

	// 2. Retrieve Chat ID from UUID
	chat, err := db.FindChatByUUID(chatDB, chatUUID)
	if err != nil {
		return fmt.Errorf("failed to find chat: %w", err)
	}
	if chat == nil {
		return fmt.Errorf("chat not found")
	}

	// 3. Retrieve Messages
	// We need all messages up to the parentID to reconstruct the context.
	// For simplicity in this iteration, we fetch the full tree and filter.
	// A more optimized query would be "GetMessagesRecursiveUpTo(chatID, parentID)".
	allMessages, err := db.GetMessagesRecursive(chatDB, chat.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve messages: %w", err)
	}

	// Filter messages to only include those up to the parentID
	var contextMessages []db.Message
	for _, msg := range allMessages {
		if msg.ID <= parentID {
			contextMessages = append(contextMessages, msg)
		}
	}

	// 4. Setup Chat Directory
	gscHome, _ := settings.GetGSCHome(false)
	chatDir := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeChatsDirRelPath, chatUUID)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %w", err)
	}

	// 5. Reconstruct File-Based State
	archiveSettings := Settings{
		ChunkSize: settings.DefaultClaudeChunkSize,
		MaxFiles:  settings.DefaultClaudeMaxFiles,
	}
	
	archiveFiles, err := SyncArchive(chatDir, contextMessages, archiveSettings)
	if err != nil {
		return fmt.Errorf("failed to sync archive: %w", err)
	}

	// 6. Write User Message
	userMsgPath := filepath.Join(chatDir, "user-message.txt")
	if err := os.WriteFile(userMsgPath, []byte(userMessage), 0644); err != nil {
		return fmt.Errorf("failed to write user message: %w", err)
	}

	// 7. Prepare CLAUDE.md
	// Merge project CLAUDE.md (if exists) with our protocol template
	if err := prepareClaudeMD(chatDir); err != nil {
		logger.Warning("Failed to prepare CLAUDE.md", "error", err)
	}

	// 8. Prepare System Prompt
	// For now, we use a minimal default or the template if it exists.
	// In a future iteration, this would be the cleaned-up coding-assistant.md.
	systemPromptPath := filepath.Join(chatDir, "messages", "system-prompt.md")
	defaultPrompt := "You are a helpful coding assistant."
	if _, err := os.Stat(systemPromptPath); os.IsNotExist(err) {
		if err := os.WriteFile(systemPromptPath, []byte(defaultPrompt), 0644); err != nil {
			return fmt.Errorf("failed to write system prompt: %w", err)
		}
	}

	// 9. Execute Claude Code CLI
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

	// 10. Parse Metrics from Output
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

	// 11. Save Metrics
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
func prepareClaudeMD(chatDir string) error {
	// 1. Read Project CLAUDE.md (if exists)
	// We assume the project root is the parent of GSC_HOME or current working directory.
	// For simplicity, we check the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectClaudeMD := filepath.Join(cwd, "CLAUDE.md")
	
	var projectContent string
	if data, err := os.ReadFile(projectClaudeMD); err == nil {
		projectContent = string(data)
	}

	// 2. Read GitSense Protocol Template
	templatePath := filepath.Join(settings.GetGSCHome(false), settings.ClaudeCodeDirRelPath, settings.ClaudeTemplatesDirRelPath, "claude_template.md")
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
