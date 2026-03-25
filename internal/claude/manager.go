/**
 * Component: Claude Code Execution Manager
 * Block-UUID: 7e503cc3-40d8-4567-92bd-77565018e5df
 * Parent-UUID: b613af12-716c-4e49-b32c-3aac3a2c00dd
 * Version: 1.53.2
 * Description: Strengthen context reading protocol prompt to ensure LLM always reads messages.map, user-message.md, and messages-active.json at every turn for proper context reconstruction
 * Language: Go
 * Created-at: 2026-03-25T15:18:18.206Z
 * Authors: claude-haiku-4-5-20251001 (v1.53.1), claude-haiku-4-5-20251001 (v1.53.2)
 */


package claude

import (
	"database/sql"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/google/uuid"
)

// Constants for stream processing
const (
	maxTokenSize = 10 * 1024 * 1024 // 10MB max buffer
	initialBufSize = 64 * 1024       // 64KB initial buffer
	dirPermissions = 0755
	filePermissions = 0644
)

// ExecuteChat is the main entry point for executing a Claude Code chat session.
// assistantMessageID is the ID of the assistant message (placeholder) in the database.
func ExecuteChat(chatUUID string, assistantMessageID int64, userMessage string, format string, appendMsg bool, save bool, appendSave bool, model string, thinkingBudget int) error {
	startTime := time.Now()

	// Phase 1: Setup & Prepare
	chatDB, metricsDB, _, chatDir, systemPromptPath, effectiveModel, storedSessionID, _, archiveSettings, err := setupAndPrepare(
		chatUUID, userMessage, assistantMessageID, appendMsg, appendSave, model,
	)
	if err != nil {
		return err
	}
	defer chatDB.Close()
	defer metricsDB.Close()

	// Phase 2: Execute Command & Process Stream
	streamResult, err := executeCommand(
		chatDir, systemPromptPath, effectiveModel, storedSessionID, thinkingBudget, format, archiveSettings,
	)
	if err != nil {
		return err
	}

	// Phase 3: Finalize & Save
	chat, err := db.FindChatByUUID(chatDB, chatUUID)
	if err != nil {
		return fmt.Errorf("failed to find chat for finalization: %w", err)
	}

	duration := time.Since(startTime)
	return finalizeAndSave(
		chatDB, metricsDB, chat, chatUUID, assistantMessageID, streamResult, effectiveModel, save, duration,
	)
}

// setupAndPrepare handles all setup, preparation, and context resolution.
// Returns databases, paths, and configuration needed for execution.
func setupAndPrepare(
	chatUUID string,
	userMessage string,
	assistantMessageID int64,
	appendMsg, appendSave bool,
	model string,
) (
	*sql.DB,   // chatDB
	*sql.DB,   // metricsDB
	string,    // gscHome
	string,    // chatDir
	string,    // systemPromptPath
	string,    // effectiveModel
	string,    // storedSessionID
	[]db.Message, // contextMessages
	Settings,  // archiveSettings
	error,
) {
	// 1. Pre-flight Check: Ensure 'claude' binary is in PATH
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("claude CLI not found in PATH. Please install Claude Code CLI first")
	}

	// 2. Resolve GSC_HOME
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// 3. Open Databases
	chatDBPath := settings.GetChatDatabasePath(gscHome)
	chatDB, err := db.OpenDB(chatDBPath)
	if err != nil {
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to open chat database: %w", err)
	}

	metricsDB, err := OpenMetricsDB()
	if err != nil {
		chatDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to open metrics database: %w", err)
	}

	// 4. Retrieve Chat ID from UUID
	chat, err := db.FindChatByUUID(chatDB, chatUUID)
	if err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to find chat: %w", err)
	}
	if chat == nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("chat not found")
	}

	// 5.5.1. Check if we have a stored session ID for this chat
	storedSessionID := ""
	storedSessionID, err = GetSessionByChatUUID(metricsDB, chatUUID)
	if err != nil {
		logger.Warning("Failed to query stored session ID", "error", err)
	} else if storedSessionID != "" {
		logger.Info("Found stored session ID", "session_id", storedSessionID)
	}

	// 5.5. Determine Parent ID (Hierarchy: explicit > append/append-save)
	if assistantMessageID == 0 && (appendMsg || appendSave) {
		lastID, err := db.GetLastMessageID(chatDB, chat.ID)
		if err != nil {
			chatDB.Close()
			metricsDB.Close()
			return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to get last message ID for append: %w", err)
		}
		assistantMessageID = lastID
		logger.Info("Auto-appending to latest message", "assistant_message_id", assistantMessageID)
	}

	if assistantMessageID == 0 {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("no assistant-message-id specified and no append flag set")
	}

	// 5.6. Handle --append-save (Insert User Message)
	if appendSave {
		logger.Info("Saving user message to database", "assistant_message_id", assistantMessageID)

		userMsg := &db.Message{
			Type:        "regular",
			Deleted:     0,
			Visibility:  "public",
			ChatID:      chat.ID,
			ParentID:    assistantMessageID,
			Level:       1,
			Role:        "user",
			Message:     sql.NullString{String: userMessage, Valid: true},
			RealModel:   sql.NullString{Valid: false},
			Temperature: sql.NullFloat64{Valid: false},
		}

		newID, err := db.InsertMessage(chatDB, userMsg)
		if err != nil {
			chatDB.Close()
			metricsDB.Close()
			return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to save user message to database: %w", err)
		}
		logger.Success("User message saved", "id", newID)
		assistantMessageID = newID
		logger.Info("Updated assistant_message_id for response", "assistant_message_id", assistantMessageID)
	}

	// 5. Retrieve Messages and Filter by Ancestry
	allMessages, err := db.GetMessagesRecursive(chatDB, chat.ID)
	if err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to retrieve messages: %w", err)
	}

	contextMessages, err := getAncestors(allMessages, assistantMessageID)
	if err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to filter message ancestry: %w", err)
	}

	// 6. Setup Chat Directory
	chatDir := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeChatsDirRelPath, chatUUID)
	if err := os.MkdirAll(chatDir, dirPermissions); err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to create chat directory: %w", err)
	}

	// Setup Logs Directory early for checkpointing
	logDir := filepath.Join(chatDir, "logs")
	if err := os.MkdirAll(logDir, dirPermissions); err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// 7. Reconstruct File-Based State
	archiveSettings := Settings{
		ChunkSize: settings.DefaultClaudeChunkSize,
		MaxFiles:  settings.DefaultClaudeMaxFiles,
		Model:     settings.DefaultClaudeModel,
	}

	settingsPath := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeSettingsFileName)
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &archiveSettings); err != nil {
			logger.Warning("Failed to parse settings.json, using defaults", "error", err)
		} else {
			logger.Debug("Loaded settings from file", "model", archiveSettings.Model, "max_files", archiveSettings.MaxFiles)
		}
	} else {
		logger.Debug("Settings file not found, using defaults", "path", settingsPath)
	}

	var historicalMessages []db.Message
	if len(contextMessages) > 0 {
		historicalMessages = contextMessages[:len(contextMessages)-1]
	}

	_, err = SyncArchive(chatDir, historicalMessages, archiveSettings)
	if err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to sync archive: %w", err)
	}

	// 8. Write User Message
	userMsgPath := filepath.Join(chatDir, "messages", "user-message.md")
	if err := os.WriteFile(userMsgPath, []byte(userMessage), filePermissions); err != nil {
		chatDB.Close()
		metricsDB.Close()
		return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to write user message: %w", err)
	}

	// 9. Prepare CLAUDE.md
	if err := prepareClaudeMD(chatDir, gscHome); err != nil {
		logger.Warning("Failed to prepare CLAUDE.md", "error", err)
	}

	// 10. Prepare System Prompt
	systemPromptPath := filepath.Join(chatDir, "messages", "system-prompt.md")
	defaultPrompt := "You are a helpful coding assistant."

	templatePath := filepath.Join(gscHome, settings.ClaudeTemplatesPath, "coding_assistant.md")
	if data, err := os.ReadFile(templatePath); err == nil {
		defaultPrompt = string(data)
		logger.Debug("Loaded coding_assistant.md template")
	} else {
		logger.Debug("coding_assistant.md not found, using default prompt", "error", err)
	}

	if _, err := os.Stat(systemPromptPath); os.IsNotExist(err) {
		if err := os.WriteFile(systemPromptPath, []byte(defaultPrompt), filePermissions); err != nil {
			chatDB.Close()
			metricsDB.Close()
			return nil, nil, "", "", "", "", "", nil, Settings{}, fmt.Errorf("failed to write system prompt: %w", err)
		}
	}

	prompt := "CRITICAL: At every turn, follow these steps in order:\n\n" +
		"## Step 1: Read messages/messages.map (ALWAYS)\n" +
		"This is your entry point. It contains metadata for all available files:\n" +
		"- read_sequence: Ordered list of files for this request\n" +
		"- context_files: Metadata for source code archive files\n" +
		"- messages: Metadata for dialogue files\n\n" +
		"## Step 2: Read messages/user-message.md (ALWAYS)\n" +
		"This contains the current user's request. Read immediately after messages.map.\n\n" +
		"## Step 3: Read messages-active.json (ALWAYS)\n" +
		"This contains the recent conversation window (last 5 messages).\n" +
		"This is current context, not history. Always read it.\n\n" +
		"## Step 4: Read Historical/Archive Files (AS NEEDED)\n" +
		"Use messages.map to find relevant files:\n" +
		"- messages-archive-*.json: Older conversation chunks (read if context requires it)\n" +
		"- context-range-*.md: Source code archives (read only the files needed to answer the request)\n" +
		"- cli-output-*.md: CLI output (read only if relevant)\n\n" +
		"## CRITICAL: Do Not Emit Analysis Until After Step 3\n" +
		"Emit ONLY Read tool calls for messages.map, user-message.md, and messages-active.json in your first response.\n" +
		"Do not attempt to analyze the request or provide a partial answer until you have read all three.\n" +
		"After reading these three files, you may selectively read additional archives as needed.\n\n" +
		"## Optimize for Cache Hit Rate\n" +
		"The read_sequence in messages.map is pre-ordered for cache optimization.\n" +
		"Only read additional files that are actually relevant to the current request.\n" +
		"Unnecessary reads waste tokens and reduce cache efficiency.\n\n" +
		"See CLAUDE.md for the complete protocol."

	prompt += "\n"
	prompt += "REFERENCE EXAMPLE:\n"
	prompt += "If your response includes a code modification (a patch), it must look EXACTLY like this:\n"
	prompt += "\n"
	prompt += "```diff\n"
	prompt += "# Patch Metadata\n"
	prompt += "# Component: Example\n"
	prompt += "# Source-Block-UUID: d3b2c976-7e72-44bc-a92d-7e571bed3818\n"
	prompt += "# Target-Block-UUID: {{GS-UUID}}\n"
	prompt += "# Source-Version: 1.0.0\n"
	prompt += "# Target-Version: 1.0.1\n"
	prompt += "# Description: Fix bug\n"
	prompt += "# Language: JavaScript\n"
	prompt += "# Created-at: 2026-03-25T14:54:29.604Z\n"
	prompt += "# Authors: User (v1.0.0), claude-haiku-4-5-20251001 (v1.0.1)\n"
	prompt += "\n"
	prompt += "\n"
	prompt += "# --- PATCH START MARKER ---\n"
	prompt += "--- Original\n"
	prompt += "+++ Modified\n"
	prompt += "@@ -1,1 +1,1 @@\n"
	prompt += "-console.log(\"old\");\n"
	prompt += "++console.log(\"new\");\n"
	prompt += "# --- PATCH END MARKER ---\n"
	prompt += "```\n\n"
	prompt += "Follow the protocol in CLAUDE.md."

	effectiveModel := archiveSettings.Model
	if model != "" {
		effectiveModel = model
		logger.Info("Model overridden by CLI flag", "model", effectiveModel)
	}

	return chatDB, metricsDB, gscHome, chatDir, systemPromptPath, effectiveModel, storedSessionID, contextMessages, archiveSettings, nil
}

// executeCommand builds and executes the Claude CLI command, processing the stream.
func executeCommand(
	chatDir string,
	systemPromptPath string,
	effectiveModel string,
	storedSessionID string,
	thinkingBudget int,
	format string,
	archiveSettings Settings,
) (StreamResult, error) {
	emptyResult := StreamResult{}

	// Build the prompt
	prompt := "CRITICAL: At every turn, follow these steps in order:\n\n" +
		"## Step 1: Read messages/messages.map (ALWAYS)\n" +
		"This is your entry point. It contains metadata for all available files:\n" +
		"- read_sequence: Ordered list of files for this request\n" +
		"- context_files: Metadata for source code archive files\n" +
		"- messages: Metadata for dialogue files\n\n" +
		"## Step 2: Read messages/user-message.md (ALWAYS)\n" +
		"This contains the current user's request. Read immediately after messages.map.\n\n" +
		"## Step 3: Read messages-active.json (ALWAYS)\n" +
		"This contains the recent conversation window (last 5 messages).\n" +
		"This is current context, not history. Always read it.\n\n" +
		"## Step 4: Read Historical/Archive Files (AS NEEDED)\n" +
		"Use messages.map to find relevant files:\n" +
		"- messages-archive-*.json: Older conversation chunks (read if context requires it)\n" +
		"- context-range-*.md: Source code archives (read only the files needed to answer the request)\n" +
		"- cli-output-*.md: CLI output (read only if relevant)\n\n" +
		"## CRITICAL: Do Not Emit Analysis Until After Step 3\n" +
		"Emit ONLY Read tool calls for messages.map, user-message.md, and messages-active.json in your first response.\n" +
		"Do not attempt to analyze the request or provide a partial answer until you have read all three.\n" +
		"After reading these three files, you may selectively read additional archives as needed.\n\n" +
		"## Optimize for Cache Hit Rate\n" +
		"The read_sequence in messages.map is pre-ordered for cache optimization.\n" +
		"Only read additional files that are actually relevant to the current request.\n" +
		"Unnecessary reads waste tokens and reduce cache efficiency.\n\n" +
		"See CLAUDE.md for the complete protocol."

	prompt += "\n"
	prompt += "REFERENCE EXAMPLE:\n"
	prompt += "If your response includes a code modification (a patch), it must look EXACTLY like this:\n"
	prompt += "\n"
	prompt += "```diff\n"
	prompt += "# Patch Metadata\n"
	prompt += "# Component: Example\n"
	prompt += "# Source-Block-UUID: 35d83f6a-4731-4831-a714-68b7df5af6dc\n"
	prompt += "# Target-Block-UUID: {{GS-UUID}}\n"
	prompt += "# Source-Version: 1.0.0\n"
	prompt += "# Target-Version: 1.0.1\n"
	prompt += "# Description: Fix bug\n"
	prompt += "# Language: JavaScript\n"
	prompt += "# Created-at: 2026-03-25T14:54:29.604Z\n"
	prompt += "# Authors: User (v1.0.0), claude-haiku-4-5-20251001 (v1.0.1)\n"
	prompt += "\n"
	prompt += "\n"
	prompt += "# --- PATCH START MARKER ---\n"
	prompt += "--- Original\n"
	prompt += "+++ Modified\n"
	prompt += "@@ -1,1 +1,1 @@\n"
	prompt += "-console.log(\"old\");\n"
	prompt += "++console.log(\"new\");\n"
	prompt += "# --- PATCH END MARKER ---\n"
	prompt += "```\n\n"
	prompt += "Follow the protocol in CLAUDE.md."

	// Build CLI flags
	flags := []string{
		"-p", fmt.Sprintf("%q", prompt),
		"--append-system-prompt-file", "./messages/system-prompt.md",
		"--allowedTools", "Read",
		"--verbose",
		"--include-partial-messages",
		"--output-format", "stream-json",
	}

	if effectiveModel != "" {
		flags = append(flags, "--model", effectiveModel)
	}

	if storedSessionID != "" {
		flags = append(flags, "--resume", storedSessionID)
		logger.Info("Reusing session", "session_id", storedSessionID)
	}

	if thinkingBudget > 0 {
		flags = append(flags, "--thinking", strconv.Itoa(thinkingBudget))
	}

	// Execute Command
	cmd := exec.Command("claude", flags...)
	cmd.Dir = chatDir

	logger.Debug("Executing Claude CLI command", "command", strings.Join(cmd.Args, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return emptyResult, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return emptyResult, fmt.Errorf("failed to start claude command: %w", err)
	}

	// Capture stderr
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// Setup logging directory
	logDir := filepath.Join(chatDir, "logs")

	// Create StreamProcessor
	processor := &StreamProcessor{
		LogDir:         logDir,
		Format:         format,
		EffectiveModel: effectiveModel,
		CurrentTime:    time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		StderrBuf:      &stderrBuf,
	}

	// Open log file
	logFileName := fmt.Sprintf("raw-stream-%s.ndjson", time.Now().Format("20060102-150405"))
	logFilePath := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, filePermissions)
	if err != nil {
		return emptyResult, fmt.Errorf("failed to create raw stream log file: %w", err)
	}
	defer logFile.Close()
	processor.LogFile = logFile

	// Process stream
	streamResult, err := processor.processStream(stdout, logDir)
	if err != nil {
		return emptyResult, err
	}

	// Wait for command to complete
	waitErr := cmd.Wait()
	streamResult.StderrOutput = stderrBuf.String()

	if streamResult.StderrOutput != "" {
		fmt.Fprintln(os.Stderr, streamResult.StderrOutput)
		logger.Error("Claude CLI stderr output", "output", streamResult.StderrOutput)
	}

	if waitErr != nil {
		if exitError, ok := waitErr.(*exec.ExitError); ok {
			streamResult.ExitCode = exitError.ExitCode()
		} else {
			streamResult.ExitCode = 1
		}
	} else {
		streamResult.ExitCode = 0
	}

	return streamResult, nil
}

// finalizeAndSave handles metrics and database persistence
func finalizeAndSave(
	chatDB *sql.DB,
	metricsDB *sql.DB,
	chat *db.Chat,
	chatUUID string,
	assistantMessageID int64,
	streamResult StreamResult,
	effectiveModel string,
	save bool,
	duration time.Duration,
) error {
	// Log execution history
	historyEntry := HistoryEntry{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		ChatUUID:    chatUUID,
		WorkingDir:  "",
		ExitCode:    streamResult.ExitCode,
		Stderr:      streamResult.StderrOutput,
		DurationMs:  duration.Milliseconds(),
	}
	if err := logExecutionHistory("", historyEntry); err != nil {
		logger.Warning("Failed to write execution history", "error", err)
	}

	// 13. Save Metrics
	if streamResult.SessionID == "" {
		streamResult.SessionID = fmt.Sprintf("stream-%d", time.Now().UnixNano())
	}

	if err := InsertCompletion(
		metricsDB,
		chatUUID,
		0,
		streamResult.SessionID,
		"claude-code",
		streamResult.Usage,
		streamResult.Cost,
		int(duration.Milliseconds()),
		"",
		0,
	); err != nil {
		logger.Error("Failed to save completion metrics", "error", err)
	}

	if err := UpsertSession(
		metricsDB,
		streamResult.SessionID,
		chatUUID,
		streamResult.Usage,
		streamResult.Cost,
	); err != nil {
		logger.Error("Failed to upsert session metrics", "error", err)
	}

	// 14. Save Response to Database (if --save flag is set)
	if save {
		logger.Info("Saving response to database", "parent_id", assistantMessageID)

		responseContent := streamResult.FullResponse
		currentTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

		uuidPattern := regexp.MustCompile(`{{GS-UUID}}`)
		timePattern := regexp.MustCompile(`2026-03-25T14:54:29.604Z`)

		finalContent := uuidPattern.ReplaceAllStringFunc(responseContent, func(match string) string {
			return uuid.New().String()
		})

		finalContent = timePattern.ReplaceAllString(finalContent, currentTime)

		newMessage := &db.Message{
			Type:        "regular",
			Deleted:     0,
			Visibility:  "public",
			ChatID:      chat.ID,
			ParentID:    assistantMessageID,
			Level:       1,
			Role:        "assistant",
			Message:     sql.NullString{String: finalContent, Valid: true},
			RealModel:   sql.NullString{String: effectiveModel, Valid: true},
			Temperature: sql.NullFloat64{Valid: false},
		}

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
	return os.WriteFile(destPath, []byte(finalContent), filePermissions)
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

// replacePlaceholders replaces template variables in text content
func replacePlaceholders(text, modelName, utcTime string) string {
	text = strings.ReplaceAll(text, "{{MODEL-NAME}}", modelName)
	text = strings.ReplaceAll(text, "{{UTC-TIME}}", utcTime)
	return text
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
	if err := os.MkdirAll(filepath.Dir(historyPath), dirPermissions); err != nil {
		return err
	}

	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, filePermissions)
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
