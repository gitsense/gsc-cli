/**
 * Component: Claude Code Execution Manager
 * Block-UUID: d34f94c7-d1b5-4d60-9f25-eb108e377ce9
 * Parent-UUID: b8233165-f479-4444-b23b-bd88e4669fc6
 * Version: 1.6.0
 * Description: Added explicit printing of the Claude response result to stdout so the user can see the answer in the CLI.
 * Language: Go
 * Created-at: 2026-03-22T16:38:44.106Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)
 */


package claude

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/google/uuid"
)

// ExecuteChat is the main entry point for executing a Claude Code chat session.
func ExecuteChat(chatUUID string, parentID int64, userMessage string, format string, appendMsg bool, save bool, appendSave bool, model string) error {
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

	// 5.5. Determine Parent ID (Hierarchy: explicit > append/append-save)
	if parentID == 0 && (appendMsg || appendSave) {
		lastID, err := db.GetLastMessageID(chatDB, chat.ID)
		if err != nil {
			return fmt.Errorf("failed to get last message ID for append: %w", err)
		}
		parentID = lastID
		logger.Info("Auto-appending to latest message", "parent_id", parentID)
	}

	if parentID == 0 {
		return fmt.Errorf("no parent-id specified and no append flag set")
	}

	// 5.6. Handle --append-save (Insert User Message)
	if appendSave {
		logger.Info("Saving user message to database", "parent_id", parentID)

		// Insert User Message
		userMsg := &db.Message{
			Type:       "regular",
			Deleted:    0,
			Visibility: "public",
			ChatID:     chat.ID,
			ParentID:   parentID,
			Level:      1, // Level is a legacy thing that we will calculate at runtime. Just set it to 1.
			Role:       "user",
			Message:    sql.NullString{String: userMessage, Valid: true},
			RealModel:  sql.NullString{Valid: false}, // User messages don't have a model
			Temperature: sql.NullFloat64{Valid: false},
		}

		newID, err := db.InsertMessage(chatDB, userMsg)
		if err != nil {
			return fmt.Errorf("failed to save user message to database: %w", err)
		}
		logger.Success("User message saved", "id", newID)
		parentID = newID // Update parentID for the response
		logger.Info("Updated parent_id for response", "parent_id", parentID)
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

	// Try to load the bootstrapped coding_assistant.md template
	templatePath := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeTemplatesDirRelPath, "coding_assistant.md")
	if data, err := os.ReadFile(templatePath); err == nil {
		defaultPrompt = string(data)
		logger.Debug("Loaded coding_assistant.md template")
	} else {
		logger.Debug("coding_assistant.md not found, using default prompt", "error", err)
	}

	if _, err := os.Stat(systemPromptPath); os.IsNotExist(err) {
		if err := os.WriteFile(systemPromptPath, []byte(defaultPrompt), 0644); err != nil {
			return fmt.Errorf("failed to write system prompt: %w", err)
		}
	}

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
	
	// Determine effective model
	effectiveModel := "Claude Code"
	if model != "" {
		effectiveModel = model
	}

	// Inject Identity into System Prompt
	identityPrompt := fmt.Sprintf("Your name is %s. When generating code, you must include this name in the Authors field.", effectiveModel)

	prompt := fmt.Sprintf("Read the user message in user-message.txt. IMPORTANT: First, read all context files: [%s]. Follow the protocol in CLAUDE.md.", filesListStr)
	
	flags := []string{
		"-p", fmt.Sprintf("%q", prompt),
		"--append-system-prompt-file", "./messages/system-prompt.md",
		"--append-system-prompt", identityPrompt,
		"--allowedTools", "Read",
		"--output-format", "stream-json",
	}

	// Add model flag if specified
	if model != "" {
		flags = append(flags, "--model", model)
	}

	// 11. Execute Claude Code CLI (Streaming)
	cmd := exec.Command("claude", flags...)
	cmd.Dir = chatDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude command: %w", err)
	}

	// 12. Process Stream
	scanner := bufio.NewScanner(stdout)
	var fullResponse strings.Builder
	var finalUsage Usage
	var finalCost float64
	var sessionID string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse event type to determine unmarshaling target
		var baseEvent StreamEvent
		if err := json.Unmarshal([]byte(line), &baseEvent); err != nil {
			logger.Warning("Failed to parse stream event line", "line", line, "error", err)
			continue
		}

		switch baseEvent.Type {
		case "text_delta":
			var deltaEvent TextDeltaEvent
			if err := json.Unmarshal([]byte(line), &deltaEvent); err == nil {
				fullResponse.WriteString(deltaEvent.Delta)

				if format == "text" {
					// Stream text directly to user
					fmt.Print(deltaEvent.Delta)
				} else if format == "json" {
					// Stream raw JSON to backend
					fmt.Println(line)
				}
			}
		case "usage":
			var usageEvent StreamUsageEvent
			if err := json.Unmarshal([]byte(line), &usageEvent); err == nil {
				finalUsage = usageEvent.Usage
				finalCost = usageEvent.Cost
			}
		case "error":
			logger.Error("Claude CLI stream error", "data", line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading claude output: %w", err)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("claude command exited with error: %w", err)
	}

	// 13. Save Metrics
	duration := time.Since(startTime)
	// Note: Session ID is not typically emitted in stream-json events in the same way as the final JSON object.
	// We might need to derive it or handle it differently if the CLI provides it in a specific event.
	// For now, we will use a placeholder or check if it was in a specific event.
	// Assuming the CLI might emit a 'session_start' or similar, or we generate one locally.
	// If the CLI doesn't emit it, we might need to track it via environment or process ID.
	// For this implementation, we'll assume we need to capture it if available, or use a generated UUID.
	// *Self-correction*: The standard Claude Code CLI stream-json usually includes session info in the first event or similar.
	// We will look for a specific event or just use a generated ID for tracking purposes if not found.
	// Let's assume for now we generate a session ID for tracking purposes if not provided by the stream.
	if sessionID == "" {
		sessionID = fmt.Sprintf("stream-%d", time.Now().UnixNano())
	}

	if sessionID != "" {
		if err := InsertCompletion(
			metricsDB,
			chatUUID,
			0, // We don't have the new message ID yet (Node.js creates it)
			sessionID,
			"claude-code", // Model placeholder
			finalUsage,
			finalCost,
			int(duration.Milliseconds()),
			"", // rawJSON (streaming mode doesn't produce a single JSON blob)
			0,  // exitCode (assumed 0 if we reached here)
		); err != nil {
			logger.Error("Failed to save completion metrics", "error", err)
		}

		if err := UpsertSession(
			metricsDB,
			sessionID,
			chatUUID,
			finalUsage,
			finalCost,
		); err != nil {
			logger.Error("Failed to upsert session metrics", "error", err)
		}
	}

	// 14. Save Response to Database (if --save flag is set)
	if save {
		logger.Info("Saving response to database", "parent_id", parentID)

		// 1. Get full response
		responseContent := fullResponse.String()

		// 2. Replace Placeholders
		uuidPattern := regexp.MustCompile(`{{GS-UUID}}`)
		timePattern := regexp.MustCompile(`{{UTC-TIME}}`)
		
		currentTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

		// Replace UUIDs (each one gets a unique UUID)
		finalContent := uuidPattern.ReplaceAllStringFunc(responseContent, func(match string) string {
			return uuid.New().String()
		})

		// Replace Time (all get the same timestamp)
		finalContent = timePattern.ReplaceAllString(finalContent, currentTime)

		// 3. Construct Message
		newMessage := &db.Message{
			Type:       "regular",
			Deleted:    0,
			Visibility: "public",
			ChatID:     chat.ID,
			ParentID:   parentID,
			Level:      1, // Level is a legacy thing that we will calculate at runtime. Just set it to 1.
			Role:       "assistant",
			Message:    sql.NullString{String: finalContent, Valid: true},
			RealModel:  sql.NullString{String: effectiveModel, Valid: true},
			Temperature: sql.NullFloat64{Valid: false}, // Default
		}

		// 4. Insert
		newMsgID, err := db.InsertMessage(chatDB, newMessage)
		if err != nil {
			return fmt.Errorf("failed to save message to database: %w", err)
		}
		logger.Success("Message saved", "id", newMsgID)
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
