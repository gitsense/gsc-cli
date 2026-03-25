/**
 * Component: Claude Code Execution Manager
 * Block-UUID: fa48684c-a562-43cd-8bfa-db257467d976
 * Parent-UUID: 581915a2-b9c2-40f6-93a5-82ef0048714d
 * Version: 1.49.0
 * Description: Added deferred error logging to capture stack traces if the function returns before metrics are written.
 * Language: Go
 * Created-at: 2026-03-25T00:13:47.724Z
 * Authors: GLM-4.7 (v1.31.0), ..., GLM-4.7 (v1.48.0), GLM-4.7 (v1.49.0)
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
	"runtime/debug"
	"strconv"
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
// assistantMessageID is the ID of the assistant message (placeholder) in the database.
func ExecuteChat(chatUUID string, assistantMessageID int64, userMessage string, format string, appendMsg bool, save bool, appendSave bool, model string, thinkingBudget int) error {
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

	// 5.5.1. Check if we have a stored session ID for this chat
	var storedSessionID string
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
			return fmt.Errorf("failed to get last message ID for append: %w", err)
		}
		assistantMessageID = lastID
		logger.Info("Auto-appending to latest message", "assistant_message_id", assistantMessageID)
	}

	if assistantMessageID == 0 {
		return fmt.Errorf("no assistant-message-id specified and no append flag set")
	}

	// 5.6. Handle --append-save (Insert User Message)
	if appendSave {
		logger.Info("Saving user message to database", "assistant_message_id", assistantMessageID)

		// Insert User Message
		userMsg := &db.Message{
			Type:       "regular",
			Deleted:    0,
			Visibility: "public",
			ChatID:     chat.ID,
			ParentID:   assistantMessageID,
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
		assistantMessageID = newID // Update assistantMessageID for the response
		logger.Info("Updated assistant_message_id for response", "assistant_message_id", assistantMessageID)
	}

	// 5. Retrieve Messages and Filter by Ancestry
	allMessages, err := db.GetMessagesRecursive(chatDB, chat.ID)
	if err != nil {
		return fmt.Errorf("failed to retrieve messages: %w", err)
	}

	// Filter messages to only include ancestors of the parentID (Fork-safe)
	contextMessages, err := getAncestors(allMessages, assistantMessageID)
	if err != nil {
		return fmt.Errorf("failed to filter message ancestry: %w", err)
	}

	// 6. Setup Chat Directory
	chatDir := filepath.Join(gscHome, settings.ClaudeCodeDirRelPath, settings.ClaudeChatsDirRelPath, chatUUID)
	if err := os.MkdirAll(chatDir, 0755); err != nil {
		return fmt.Errorf("failed to create chat directory: %w", err)
	}
	
	// Setup Logs Directory early for checkpointing
	logDir := filepath.Join(chatDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// 7. Reconstruct File-Based State
	// Load Settings
	archiveSettings := Settings{
		ChunkSize: settings.DefaultClaudeChunkSize,
		MaxFiles:  settings.DefaultClaudeMaxFiles,
		Model:     settings.DefaultClaudeModel,
	}

	// Try to load from settings.json
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

	// Exclude the current user message from the archive contextMessages 
	// ends with the current user message (chatParentID) We want everything 
	// BEFORE it for the archive (historical context only)
	var historicalMessages []db.Message
	if len(contextMessages) > 0 {
		historicalMessages = contextMessages[:len(contextMessages)-1]
	}

	// Sync Archive with new cache-optimized context file construction
	_, err = SyncArchive(chatDir, historicalMessages, archiveSettings)
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
	// We use a static prompt to ensure cache stability. The agent reads the map to discover files.
	prompt := "1. Read messages/messages.map to understand the available context.\n" +
	          "2. Read messages/user-message.md to understand the user's request.\n" +
	          "3. Use the Read tool to access any files you need."

	// Add Few-Shot Reference Example for Haiku
	prompt += "\n"
	prompt += "REFERENCE EXAMPLE:\n"
	prompt += "If your response includes a code modification (a patch), it must look EXACTLY like this:\n"
	prompt += "\n"
	prompt += "```diff\n"
	prompt += "# Patch Metadata\n"
	prompt += "# Component: Example\n"
	prompt += "# Source-Block-UUID: 6bb2255d-2a79-4bb4-a5ab-f49ac36f5df1\n"
	prompt += "# Target-Block-UUID: {{GS-UUID}}\n"
	prompt += "# Source-Version: 1.0.0\n"
	prompt += "# Target-Version: 1.0.1\n"
	prompt += "# Description: Fix bug\n"
	prompt += "# Language: JavaScript\n"
	prompt += "# Created-at: {{UTC-TIME}}\n"
	prompt += "# Authors: User (v1.0.0), {{MODEL-NAME}} (v1.0.1)\n"
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

	// Determine Effective Model (Priority: Flag > Config > Default)
	effectiveModel := archiveSettings.Model
	if model != "" {
		effectiveModel = model
		logger.Info("Model overridden by CLI flag", "model", effectiveModel)
	}

	// Inject Identity into System Prompt
	identityPrompt := "Your name is {{MODEL-NAME}}. When generating code, you must include this name in the Authors field."

	// Check if identity prompt already exists before appending
	existingContent, err := os.ReadFile(systemPromptPath)
	needsAppend := true
	if err != nil {
		return fmt.Errorf("failed to read system prompt: %w", err)
	}
	
	if strings.Contains(string(existingContent), identityPrompt) {
		needsAppend = false
	}
	
	if needsAppend {
		f, err := os.OpenFile(systemPromptPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open system prompt for appending: %w", err)
		}
		if _, err := f.WriteString(identityPrompt); err != nil {
			f.Close()
			return fmt.Errorf("failed to append identity to system prompt: %w", err)
		}
		f.Close()
	}

	flags := []string{
		"-p", fmt.Sprintf("%q", prompt),
		"--append-system-prompt-file", "./messages/system-prompt.md",
		"--allowedTools", "Read",
		"--verbose",
		"--include-partial-messages",
		"--output-format", "stream-json",
	}

	// Add model flag if specified
	if effectiveModel != "" {
		flags = append(flags, "--model", effectiveModel)
	}
	
	// Add session ID flag if we have a stored session
	if storedSessionID != "" {
		flags = append(flags, "--resume", storedSessionID)
		logger.Info("Reusing session", "session_id", storedSessionID)
	}
	
	// Add thinking flag if specified
	if thinkingBudget > 0 {
		flags = append(flags, "--thinking", strconv.Itoa(thinkingBudget))
	}

	// 11. Execute Claude Code CLI (Streaming)
	cmd := exec.Command("claude", flags...)
	cmd.Dir = chatDir

	//// DEBUG: Write full command to checkpoint file for debugging
	//fullCommand := strings.Join(cmd.Args, " ")
	//checkpointPath := filepath.Join(logDir, "checkpoint-cli-start.txt")
	//checkpointContent := fmt.Sprintf(
	//	"ExecuteChat started at %s\nUUID: %s\nParentID: %d\nCommand: %s\n",
	//	time.Now().UTC().Format(time.RFC3339),
	//	chatUUID,
	//	assistantMessageID,
	//	fullCommand,
	//)
	//if err := os.WriteFile(checkpointPath, []byte(checkpointContent), 0644); err != nil {
	//	logger.Warning("Failed to write checkpoint file", "error", err)
	//}

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
	// Increase buffer size to handle large JSON objects or long log lines
	const maxTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)       // Initial 64KB buffer
	scanner.Buffer(buf, maxTokenSize)
	var fullResponse strings.Builder
	var nonJSONOutput strings.Builder // New buffer to capture non-JSON lines
	var exitCode int // Declare exitCode at function scope so defer can access it
	
	// CRITICAL: Deferred cleanup to ensure checkpoint and error logs are written even on panic/early return
	defer func() {
		// DEBUG: Unconditionally write non-JSON stdout if it has content
		if nonJSONOutput.Len() > 0 {
			nonJSONPath := filepath.Join(logDir, "debug-stdout-non-json.txt")
			if writeErr := os.WriteFile(nonJSONPath, []byte(nonJSONOutput.String()), 0644); writeErr != nil {
				logger.Warning("Failed to write debug non-JSON stdout file", "error", writeErr)
			}
		}

		// If the process exited with an error, capture stderr to a specific error file
		if exitCode != 0 {
			errorPath := filepath.Join(logDir, fmt.Sprintf("error-output-%s.txt", time.Now().Format("20060102-150405")))
			stderrStr := stderrBuf.String()
			if stderrStr != "" {
				if writeErr := os.WriteFile(errorPath, []byte(stderrStr), 0644); writeErr != nil {
					logger.Warning("Failed to write error output file", "error", writeErr)
				}
			} else {
				os.WriteFile(errorPath, []byte("Process exited with error but no stderr output was captured."), 0644)
			}
		}
	}()

	var finalUsage Usage
	var finalCost float64
	var sessionID string
	var toolsFinished bool
	var responseBuffer strings.Builder
	currentTime := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	isFirstLine := true

	logFileName := fmt.Sprintf("raw-stream-%s.ndjson", time.Now().Format("20060102-150405"))
	logFilePath := filepath.Join(logDir, logFileName)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create raw stream log file: %w", err)
	}
	defer logFile.Close()

	// DEBUG: Flag to track if metrics were successfully written
	metricsWritten := false
	
	// DEBUG: Defer function to catch early returns and log stack trace
	defer func() {
		if !metricsWritten {
			// Capture the stack trace at the point of return
			stackTrace := debug.Stack()
			
			errorMsg := fmt.Sprintf("ExecuteChat returned before metrics were written.\n\nStack Trace:\n%s", string(stackTrace))
			
			// Write to error file
			errorPath := filepath.Join(logDir, fmt.Sprintf("error-return-%s.txt", time.Now().Format("20060102-150405")))
			if writeErr := os.WriteFile(errorPath, []byte(errorMsg), 0644); writeErr != nil {
				// If we can't write to the log dir, print to stderr as a last resort
				fmt.Fprintf(os.Stderr, "CRITICAL: Failed to write error return log: %v\nOriginal Error:\n%s", writeErr, errorMsg)
			}
		}
	}()

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
			// CRITICAL DEBUGGING: Capture non-JSON lines
			nonJSONOutput.WriteString(line + "\n")

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
			} else {
				// Fallback: Try to extract session_id directly from JSON
				logger.Warning("Failed to unmarshal SystemInitEvent, attempting fallback extraction", "error", err)
				var rawEvent map[string]interface{}
				if rawErr := json.Unmarshal([]byte(line), &rawEvent); rawErr == nil {
					if sid, ok := rawEvent["session_id"].(string); ok && sid != "" {
						sessionID = sid
						logger.Info("Extracted session_id via fallback", "session_id", sessionID)
					}
					if model, ok := rawEvent["model"].(string); ok && model != "" {
						effectiveModel = model
					}
				}
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

				// Always write to fullResponse for saving to DB
				fullResponse.WriteString(modifiedDelta)

				// Gatekeeper: Only stream to user if tools are finished
				if !toolsFinished {
					// Buffer the text (suppresses "plan" leaks)
					responseBuffer.WriteString(modifiedDelta)
				} else {
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
					}
				}
			}
			continue
		}

		// Handle Stream Wrapper (for content_block_delta events like thinking)
		if baseEvent.Type == "stream_event" {
			var wrapperEvent struct {
				Type  string `json:"type"`
				Event struct {
					Type  string `json:"type"`
					Index int    `json:"index"`
					Delta struct {
						Type     string `json:"type"`
						Thinking string `json:"thinking"`
						Text     string `json:"text"`
					} `json:"delta"`
				} `json:"event"`
			}
			
			if err := json.Unmarshal([]byte(line), &wrapperEvent); err == nil {
				// Check if this is a thinking delta
				if wrapperEvent.Event.Type == "content_block_delta" && 
				   wrapperEvent.Event.Delta.Type == "thinking_delta" {
					
					if format == "json" {
						// Emit Clean Stream JSON for thinking
						cleanJSON, _ := json.Marshal(map[string]interface{}{
							"event": "thinking",
							"delta": wrapperEvent.Event.Delta.Thinking,
						})
						fmt.Println(string(cleanJSON))
					}
				}
				
				// Handle Text (Index 1)
				if wrapperEvent.Event.Delta.Type == "text_delta" {
					modifiedText := strings.ReplaceAll(wrapperEvent.Event.Delta.Text, "{{MODEL-NAME}}", effectiveModel)
					modifiedText = strings.ReplaceAll(modifiedText, "{{UTC-TIME}}", currentTime)

					if !toolsFinished {
						responseBuffer.WriteString(modifiedText)
					} else {
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
		
		// Handle User Event (Tool Result) - Signals end of thinking phase
		if baseEvent.Type == "user" {
			toolsFinished = true
			
			// CRITICAL FIX: Flush the buffered "plan" text to the user
			if responseBuffer.Len() > 0 {
				bufferedText := responseBuffer.String()
				responseBuffer.Reset()
				
				if format == "text" {
					fmt.Print(bufferedText)
				} else if format == "json" {
					cleanJSON, _ := json.Marshal(map[string]interface{}{
						"event": "text",
						"delta": bufferedText,
					})
					fmt.Println(string(cleanJSON))
				}
			}
		}

		// Handle Result Event - Final stats
		if baseEvent.Type == "result" {
			var resultEvent StreamResultEvent
			if err := json.Unmarshal([]byte(line), &resultEvent); err == nil {
				// CRITICAL FIX: Update final usage and cost from the result event
				// The stream does not emit a standalone "usage" event, only this one.
				finalUsage = resultEvent.Usage
				finalCost = resultEvent.TotalCost

				doneJSON, _ := json.Marshal(map[string]interface{}{
					"event": "done",
					"stats": resultEvent,
					"result": resultEvent.Result,
				})
				fmt.Println(string(doneJSON))
			}
		}
	}

	if err := scanner.Err(); err != nil {
		// DEBUG: Log the specific scanner error to diagnose why the stream failed
		logger.Error("Stream scanner encountered an error", "error", err, "stderr", stderrBuf.String(), "non_json_output", nonJSONOutput.String())
		fmt.Fprintln(os.Stderr, "Stream Error:", err)
		exitCode = -2 // Use a distinct exit code for scanner errors
		return fmt.Errorf("error reading claude output: %w", err)
	}

	// Wait for command to finish and capture exit details
	exitCode = 0
	waitErr := cmd.Wait()
	if waitErr != nil {
		if exitError, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode() // Update the outer scope exitCode
		} else {
			exitCode = 1 // Update the outer scope exitCode
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
	// Fallback: Parse raw stream log to extract session ID if not found during streaming
	if sessionID == "" {
		logger.Info("Session ID not found during streaming, attempting to extract from raw stream log")
		if data, err := os.ReadFile(logFilePath); err == nil {
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				line := scanner.Text()
				var rawEvent map[string]interface{}
				if err := json.Unmarshal([]byte(line), &rawEvent); err == nil {
					if eventType, ok := rawEvent["type"].(string); ok && eventType == "system" {
						if sid, ok := rawEvent["session_id"].(string); ok && sid != "" {
							sessionID = sid
							logger.Info("Extracted session_id from raw stream log", "session_id", sessionID)
							break
						}
					}
				}
			}
		} else {
			logger.Warning("Failed to read raw stream log for session ID extraction", "error", err)
		}
	}
	
	// For now, we will use a placeholder or check if it was in a specific event.
	// *Self-correction*: The standard Claude Code CLI stream-json usually includes session info in the first event or similar.
	// We will look for a specific event or just use a generated UUID for tracking purposes if not found.
	// Let's assume for now we generate a session ID for tracking purposes if not provided by the stream.
	if sessionID == "" {
		sessionID = fmt.Sprintf("stream-%d", time.Now().UnixNano())
	}

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

	// Mark metrics as successfully written
	metricsWritten = true

	// 14. Save Response to Database (if --save flag is set)
	if save {
		logger.Info("Saving response to database", "parent_id", assistantMessageID)

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
			ParentID:   assistantMessageID,
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
