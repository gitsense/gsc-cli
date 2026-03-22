/**
 * Component: Claude Code Execution Manager
 * Block-UUID: 0d15c09a-cb35-4a63-a900-2af7b072d147
 * Parent-UUID: 14e6b532-169b-4670-8431-3f00628037a7
 * Version: 1.14.0
 * Description: Implemented raw stream logging to file and fixed text extraction from 'assistant' events by properly parsing the nested content structure instead of relying on string matching.
 * Language: Go
 * Created-at: 2026-03-22T20:47:44.355Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0)
 */


package claude

import (
	"bufio"
	"database/sql"
	"bytes"
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

// AssistantMessageEvent represents the full assistant message event containing text content
type AssistantMessageEvent struct {
	Type    string `json:"type"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

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
	// Moved to messages/ directory to align with protocol and prevent path hallucinations
	// Changed extension to .md to indicate it is part of the context documentation set
	userMsgPath := filepath.Join(chatDir, "messages", "user-message.md")
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
	templatePath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "coding_assistant.md")
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
		if name == "messages-active.json" {
			// Check if file has content before adding to context list to prevent hallucinations
			fullPath := filepath.Join(msgDir, name)
			data, err := os.ReadFile(fullPath)
			if err == nil {
				var window ActiveWindow
				if json.Unmarshal(data, &window) == nil && len(window.Messages) > 0 {
					contextFiles = append(contextFiles, filepath.Join("messages", name))
				}
			}
		} else if strings.HasPrefix(name, "messages-archive-") || strings.HasPrefix(name, "context-") {
			contextFiles = append(contextFiles, filepath.Join("messages", name))
		}
	}
	
	sort.Strings(contextFiles)
	
	// Construct prompt with explicit file paths
	prompt := "Read the user message in messages/user-message.md."
	if len(contextFiles) > 0 {
		filesListStr := strings.Join(contextFiles, ", ")
		prompt += fmt.Sprintf(" IMPORTANT: First, read all context files: [%s].", filesListStr)
	}
	prompt += " Follow the protocol in CLAUDE.md."

	// Determine effective model
	effectiveModel := "Claude Code"
	if model != "" {
		effectiveModel = model
	}

	// Inject Identity into System Prompt
	identityPrompt := fmt.Sprintf("Your name is %s. When generating code, you must include this name in the Authors field.", effectiveModel)

	// Append identity to the system prompt file to avoid CLI flag conflicts
	f, err := os.OpenFile(systemPromptPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open system prompt for appending: %w", err)
	}
	if _, err := f.WriteString(identityPrompt); err != nil {
		f.Close()
		return fmt.Errorf("failed to append identity to system prompt: %w", err)
	}
	f.Close()

	flags := []string{
		"-p", fmt.Sprintf("%q", prompt),
		"--append-system-prompt-file", "./messages/system-prompt.md",
		"--verbose",
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

	// Log the full command for debugging
	logger.Debug("Executing Claude CLI command", "command", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude command: %w", err)
	}

	// Capture stderr for debugging and history logging
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// 12. Process Stream
	scanner := bufio.NewScanner(stdout)
	var fullResponse strings.Builder
	var finalUsage Usage
	var finalCost float64
	var sessionID string
	currentTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	isFirstLine := true

	// Setup Raw Stream Logging
	logDir := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}
	logFileName := fmt.Sprintf("raw-stream-%s.ndjson", time.Now().Format("20060102-150405"))
	logFilePath := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create raw stream log file: %w", err)
	}
	defer logFile.Close()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Write raw line to log file
		if _, err := logFile.WriteString(line + "\n"); err != nil {
			logger.Warning("Failed to write to raw stream log", "error", err)
		}

		// Parse event type to determine unmarshaling target
		var baseEvent StreamEvent
		if err := json.Unmarshal([]byte(line), &baseEvent); err != nil {
			logger.Warning("Failed to parse stream event line", "line", line, "error", err)
			continue
		}

		// Handle Init Event (First Line)
		if isFirstLine && baseEvent.Type == "system" {
			var initEvent SystemInitEvent
			if err := json.Unmarshal([]byte(line), &initEvent); err == nil {
				effectiveModel = initEvent.Model
				sessionID = initEvent.SessionID
				
				// Emit Clean Stream Init Event
				initJSON, _ := json.Marshal(map[string]interface{}{
					"event":      "init",
					"model":      effectiveModel,
					"session_id": sessionID,
				})
				fmt.Println(string(initJSON))
			}
			isFirstLine = false
			continue
		}

		// Handle Text Delta (On-the-fly replacement)
		if baseEvent.Type == "text_delta" {
			var deltaEvent TextDeltaEvent
			if err := json.Unmarshal([]byte(line), &deltaEvent); err == nil {
				// Replace placeholders on the fly
				modifiedDelta := strings.ReplaceAll(deltaEvent.Delta, "{{MODEL-NAME}}", effectiveModel)
				modifiedDelta = strings.ReplaceAll(modifiedDelta, "{{UTC-TIME}}", currentTime)

				fullResponse.WriteString(modifiedDelta)

				if format == "text" {
					// Stream text directly to user
					fmt.Print(modifiedDelta)
				} else if format == "json" {
					// Stream Clean Stream JSON to backend
					cleanJSON, _ := json.Marshal(map[string]interface{}{
						"event": "text",
						"delta": modifiedDelta,
					})
					fmt.Println(string(cleanJSON))
				}
			}
			continue
		}

		// Handle Thinking / Tool Use (Status Events)
		if baseEvent.Type == "assistant" {
			// Parse the full assistant event to extract content
			var assistantEvent AssistantMessageEvent
			if err := json.Unmarshal([]byte(line), &assistantEvent); err == nil {
				for _, contentBlock := range assistantEvent.Message.Content {
					switch contentBlock.Type {
					case "thinking":
						statusJSON, _ := json.Marshal(map[string]interface{}{
							"event":   "status",
							"message": "Thinking...",
						})
						fmt.Println(string(statusJSON))
					case "tool_use":
						// Extract tool name if possible, otherwise generic message
						toolName := "Working..."
						if strings.Contains(line, `"name":"Read"`) {
							toolName = "Reading context files..."
						} else if strings.Contains(line, `"name":"Glob"`) {
							toolName = "Scanning directory..."
						}
						
						statusJSON, _ := json.Marshal(map[string]interface{}{
							"event":   "status",
							"message": toolName,
						})
						fmt.Println(string(statusJSON))
					case "text":
						// Extract and process text content
						modifiedText := strings.ReplaceAll(contentBlock.Text, "{{MODEL-NAME}}", effectiveModel)
						modifiedText = strings.ReplaceAll(modifiedText, "{{UTC-TIME}}", currentTime)

						fullResponse.WriteString(modifiedText)

						if format == "text" {
							fmt.Print(modifiedText)
						} else if format == "json" {
							cleanJSON, _ := json.Marshal(map[string]interface{}{
								"event": "text",
								"delta": modifiedText,
							})
							fmt.Println(string(cleanJSON))
						}
					}
				}
			}
			continue
		}

		switch baseEvent.Type {
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

	// Emit Done Event
	if format == "json" {
		doneJSON, _ := json.Marshal(map[string]interface{}{
			"event": "done",
		})
		fmt.Println(string(doneJSON))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading claude output: %w", err)
	}

	// Wait for command to finish and capture exit details
	exitCode := 0
	waitErr := cmd.Wait()
	if waitErr != nil {
		if exitError, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}

	// Log stderr if present
	stderrStr := stderrBuf.String()
	if stderrStr != "" {
		// Print stderr to console so the user sees the error immediately
		fmt.Fprintln(os.Stderr, stderrStr)

		logger.Error("Claude CLI stderr output", "output", stderrStr)
	}

	// Log execution history
	duration := time.Since(startTime)
	historyEntry := HistoryEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		ChatUUID:    chatUUID,
		Command:     strings.Join(cmd.Args, " "),
		WorkingDir:  chatDir,
		ExitCode:    exitCode,
		Stderr:      stderrStr,
		DurationMs:  duration.Milliseconds(),
	}
	if err := logExecutionHistory(gscHome, historyEntry); err != nil {
		logger.Warning("Failed to write execution history", "error", err)
	}

	if waitErr != nil {
		return fmt.Errorf("claude command exited with error: %w", waitErr)
	}

	// 13. Save Metrics
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
	templatePath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "claude_template.md")
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

// HistoryEntry represents a single execution record in history.jsonl
type HistoryEntry struct {
	Timestamp   string `json:"timestamp"`
	ChatUUID    string `json:"chat_uuid"`
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir"`
	ExitCode    int    `json:"exit_code"`
	Stderr      string `json:"stderr"`
	DurationMs  int64  `json:"duration_ms"`
}

// logExecutionHistory appends an execution record to history.jsonl
func logExecutionHistory(gscHome string, entry HistoryEntry) error {
	historyPath := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, "history.jsonl")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(historyPath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}
