/**
 * Component: Agent Stream Event Processor
 * Block-UUID: 6f185928-ff17-44c5-8056-4df80c6eef0e
 * Parent-UUID: bc01e342-928d-4036-a61f-b69a21444b5e
 * Version: 2.4.0
 * Description: Stream event processor for agent sessions that parses Claude's streaming JSONL output, handles events, and updates session state. Refactored to delegate JSON parsing to dedicated parser files.
 * Language: Go
 * Created-at: 2026-04-15T15:36:57.179Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), claude-sonnet-4-6 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), claude-haiku-4-5-20251001 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0)
 */


package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// processStream reads Claude's stdout and processes events
func (m *Manager) processStream(stdout io.Reader, turn int) {
	m.debugLogger.Log("STREAM", "Stream processing started")
	
	// Open output.log for writing raw stdout
	outputLogPath := m.config.GetTurnDir(turn) + "/output.log"
	outputLogFile, err := os.OpenFile(outputLogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		m.debugLogger.LogError("Failed to open output.log", err)
	} else {
		defer outputLogFile.Close()
	}
	
	// Helper function to write to output.log
	writeToOutputLog := func(line string) {
		if outputLogFile != nil {
			timestamp := time.Now().UTC().Format(time.RFC3339Nano)
			outputLogFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, line))
		}
	}
	
	defer func() {
		m.debugLogger.Log("STREAM", "Stream processing ended")
		m.wg.Done()
	}()

	defer func() {
		// Close event writer when done
		if m.eventWriter != nil {
			m.eventWriter.Close()
		}
	}()

	// 1. WRITE START MARKER
	startMarker := map[string]interface{}{
		"type":       "gsc-agent-stream-start",
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"source":     "gsc-agent",
		"session_id": m.session.SessionID,
		"turn":       turn,
	}
	if markerBytes, err := json.Marshal(startMarker); err == nil {
		m.eventWriter.WriteRawEvent(string(markerBytes))
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)

	m.debugLogger.Log("STREAM", "Scanner initialized, starting to read lines")
	var usage Usage
	var cost float64
	var duration int64
	var claudeSessionID string

	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()
		
		// Write raw line to output.log
		writeToOutputLog(line)
		
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		
		// Quick check: JSON objects must start with '{'
		if !strings.HasPrefix(trimmed, "{") {
			// Not JSON, skip (already written to output.log)
			continue
		}
		
		m.debugLogger.LogStreamEvent("LINE_READ", fmt.Sprintf("line %d: %s", lineCount, m.truncateForLog(line, 200)))

		// Parse as generic map
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			// Not valid JSON, skip
			// Log as raw event for debugging
			m.debugLogger.LogStreamEvent("JSON_PARSE_ERROR", fmt.Sprintf("line %d: %v", lineCount, err))
			continue
		}
		m.debugLogger.LogStreamEvent("JSON_PARSED", fmt.Sprintf("line %d: valid JSON", lineCount))

		// Check event type
		eventType, _ := result["type"].(string)
		m.debugLogger.LogStreamEvent("EVENT_TYPE", fmt.Sprintf("line %d: %s", lineCount, eventType))

		switch eventType {
		case "ping":
			// Skip keep-alive events
			m.debugLogger.LogStreamEvent("PING", fmt.Sprintf("line %d", lineCount))
			continue

		case "error":
			// Handle error events from Claude CLI
			if errorData, ok := result["error"].(map[string]interface{}); ok {
				errorType, _ := errorData["type"].(string)
				errorMsg, _ := errorData["message"].(string)
				// Update session state
				m.session.Status = "error"
				errMsg := fmt.Sprintf("%s: %s", errorType, errorMsg)
				m.session.Error = &errMsg
				m.writeSessionState()
			}
			continue

		case "system":
			// Handle system events (API retries, etc.)
			m.debugLogger.LogStreamEvent("SYSTEM", fmt.Sprintf("line %d", lineCount))
			// Log but don't fail - these are informational
			continue

		case "result":
			// 2. WRITE RAW RESULT EVENT TO STREAM
			m.eventWriter.WriteRawEvent(line)

			// DEBUG: Log raw result event line
			m.debugLogger.Log("METRICS", fmt.Sprintf("RAW RESULT EVENT (line %d): %s", lineCount, m.truncateForLog(line, 500)))

			// Parse result event using a typed struct for safe metric extraction
			var resultMeta struct {
				IsError      bool    `json:"is_error"`
				DurationMS   float64 `json:"duration_ms"`
				TotalCostUSD float64 `json:"total_cost_usd"`
				SessionID    string  `json:"session_id"`
				Result       string  `json:"result"`
				Usage        struct {
					InputTokens              int `json:"input_tokens"`
					OutputTokens             int `json:"output_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(line), &resultMeta); err != nil {
				m.debugLogger.LogError("Failed to parse result event", err)
				continue
			} else {
				// DEBUG: Log successful parsing
				m.debugLogger.Log("METRICS", fmt.Sprintf(
					"PARSED SUCCESSFULLY - Duration: %.0fms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d, CacheCreation: %d, CacheRead: %d, ResultLen: %d, IsError: %v",
					resultMeta.DurationMS,
					resultMeta.TotalCostUSD,
					resultMeta.Usage.InputTokens,
					resultMeta.Usage.OutputTokens,
					resultMeta.Usage.CacheCreationInputTokens,
					resultMeta.Usage.CacheReadInputTokens,
					len(resultMeta.Result),
					resultMeta.IsError,
				))
			}
			resultContent := resultMeta.Result
			if resultContent != "" {
				// Check for error in result event
				if resultMeta.IsError {
					m.debugLogger.LogError("Claude result error", fmt.Errorf("%v", result))
					m.session.Status = "error"
					errMsg := fmt.Sprintf("Claude returned error: %v", result)
					m.session.Error = &errMsg
					m.writeSessionState()
					continue
				}
				
				// DEBUG: Log entering conditional block
				m.debugLogger.Log("METRICS", "ENTERING resultContent processing block")

				// Extract usage/cost/duration from outer event (type-safe via struct)
				usage = Usage{
					InputTokens:         resultMeta.Usage.InputTokens,
					OutputTokens:        resultMeta.Usage.OutputTokens,
					CacheCreationTokens: resultMeta.Usage.CacheCreationInputTokens,
					CacheReadTokens:     resultMeta.Usage.CacheReadInputTokens,
				}
				cost = resultMeta.TotalCostUSD
				duration = int64(resultMeta.DurationMS)
				claudeSessionID = resultMeta.SessionID
				
				// DEBUG: Log extracted values
				m.debugLogger.Log("METRICS", fmt.Sprintf(
					"EXTRACTED TO LOCAL VARS - Duration: %dms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d",
					duration,
					cost,
					usage.InputTokens,
					usage.OutputTokens,
				))
				
				m.debugLogger.LogStreamEvent("RESULT", fmt.Sprintf("line %d: result length=%d, cost=%.6f, duration=%d", lineCount, len(resultContent), cost, duration))

				// Determine turn type from session state
				turnType := "discovery" // Default
				if len(m.session.Turns) > 0 {
					lastTurn := m.session.Turns[len(m.session.Turns)-1]
					turnType = lastTurn.TurnType
				}

				// Delegate to specific parser
				var results *TurnResults
				var parseErr error
				
				switch turnType {
				case "discovery":
					results, parseErr = ParseDiscoveryResult(resultContent)
				case "validation":
					results, parseErr = ParseValidationResult(resultContent)
				case "change":
					results, parseErr = ParseChangeResult(resultContent)
				default:
					parseErr = fmt.Errorf("unknown turn type: %s", turnType)
				}

				if parseErr != nil {
					m.debugLogger.LogError("Failed to parse result content", parseErr)
					// Don't fail the whole session, just log the error
					continue
				}

				// Calculate total found for populateTurnState
				totalFound := 0
				if results.Candidates != nil {
					totalFound = len(results.Candidates)
				}
				// For change turns, use FilesModifiedCount if available
				if results.ChangeResults != nil {
					totalFound = results.ChangeResults.ChangeSummary.FilesModifiedCount
				}

				// Populate session state
				m.populateTurnState(turn, results.Candidates, totalFound, usage, cost, duration, claudeSessionID, results)
				
				// DEBUG: Log after populateTurnState
				m.debugLogger.Log("METRICS", fmt.Sprintf(
					"AFTER populateTurnState (%s) - Duration: %dms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d",
					turnType,
					duration,
					cost,
					usage.InputTokens,
					usage.OutputTokens,
				))
				
				m.session.Status = turnType + "_complete"
				m.MarkTurnComplete(turnType)
			}
		default:
			// Log unknown event types for debugging
			m.debugLogger.LogStreamEvent("UNKNOWN_EVENT", fmt.Sprintf("line %d: type=%s", lineCount, eventType))
			m.eventWriter.WriteRawEvent(line) // Keep logging all raw events for audit trail
			// Store assistant message for post-processing
			if eventType == "assistant" {
				m.lastAssistantMessage = line
			}
		}
	}

	// Handle scanner errors
	if err := scanner.Err(); err != nil {
		m.debugLogger.LogError("Scanner error", err)
		m.debugLogger.Log("STREAM", fmt.Sprintf("Scanner error details: %v", err))
		// Even on scanner error, try to process the last assistant message
		if m.lastAssistantMessage != "" {
			// DEBUG: Log processing last assistant message
			m.debugLogger.Log("METRICS", "Processing last assistant message as fallback")

			if err := m.processAssistantMessage(m.lastAssistantMessage, turn, usage, cost, duration, claudeSessionID); err != nil {
				m.debugLogger.LogError("Failed to process assistant message", err)
			}
		}
		m.session.Status = "error"
		errMsg := fmt.Sprintf("Error reading stream: %v", err)
		m.session.Error = &errMsg
		m.writeSessionState()
	}
	
	// Post-process the last assistant message to extract results
	if m.lastAssistantMessage != "" {
		// DEBUG: Log post-processing last assistant message
		m.debugLogger.Log("METRICS", "Post-processing last assistant message")

		if err := m.processAssistantMessage(m.lastAssistantMessage, turn, usage, cost, duration, claudeSessionID); err != nil {
			m.debugLogger.LogError("Failed to process assistant message", err)
		}
	}
	
	// 3. WRITE END MARKER
	endMarker := map[string]interface{}{
		"type":       "gsc-agent-stream-end",
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"source":     "gsc-agent",
		"session_id": m.session.SessionID,
		"turn":       turn,
	}
	if markerBytes, err := json.Marshal(endMarker); err == nil {
		m.eventWriter.WriteRawEvent(string(markerBytes))
	}

	// DEBUG: Log final state before exiting
	m.debugLogger.Log("METRICS", fmt.Sprintf(
		"STREAM ENDING - Final local vars - Duration: %dms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d",
		duration,
		cost,
		usage.InputTokens,
		usage.OutputTokens,
	))

	m.debugLogger.Log("STREAM", fmt.Sprintf("Stream processing complete: %d lines processed", lineCount))
}

// captureStderr reads and logs stderr from the subprocess
func (m *Manager) captureStderr(stderr io.Reader) {
	m.debugLogger.Log("DEBUG", "Stderr capture started")
	
	// DEBUG: Log capture start
	m.debugLogger.Log("METRICS", "Stderr capture started")

	// Open output.log for writing raw stderr
	outputLogPath := m.config.GetTurnDir(m.currentTurn) + "/output.log"
	outputLogFile, err := os.OpenFile(outputLogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		m.debugLogger.LogError("Failed to open output.log", err)
	} else {
		defer outputLogFile.Close()
		
		// DEBUG: Log output.log opened
		m.debugLogger.Log("METRICS", fmt.Sprintf("Output.log opened: %s", outputLogPath))
	}
	
	// Helper function to write to output.log
	writeToOutputLog := func(line string) {
		if outputLogFile != nil {
			timestamp := time.Now().UTC().Format(time.RFC3339Nano)
			outputLogFile.WriteString(fmt.Sprintf("[%s] [STDERR] %s\n", timestamp, line))
		}
	}
	
	// DEBUG: Log helper function created
	m.debugLogger.Log("METRICS", "Stderr helper function created")

	defer func() {
		m.debugLogger.Log("DEBUG", "Stderr capture ended")
		m.wg.Done()
	}()

	scanner := bufio.NewScanner(stderr)
	lineCount := 0
	
	// DEBUG: Log scanner created
	m.debugLogger.Log("METRICS", "Stderr scanner created")

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()
		
		// Write to output.log
		writeToOutputLog(line)
		
		// DEBUG: Log stderr line
		m.debugLogger.Log("METRICS", fmt.Sprintf("Stderr line %d: %s", lineCount, m.truncateForLog(line, 200)))
		
		m.debugLogger.Log("STDERR", fmt.Sprintf("line %d: %s", lineCount, line))
	}

	if err := scanner.Err(); err != nil {
		m.debugLogger.LogError("Stderr scanner error", err)
		m.debugLogger.Log("STDERR", fmt.Sprintf("Stderr scanner error: %v", err))
	}

	if lineCount > 0 {
		m.debugLogger.Log("STDERR", fmt.Sprintf("Stderr capture complete: %d lines captured", lineCount))
		
		// DEBUG: Log stderr capture complete
		m.debugLogger.Log("METRICS", fmt.Sprintf("Stderr capture complete: %d lines", lineCount))
	} else {
		m.debugLogger.Log("STDERR", "Stderr capture complete: no output")
		
		// DEBUG: Log no stderr output
		m.debugLogger.Log("METRICS", "Stderr capture complete: no output")
	}
}

// processAssistantMessage extracts and processes JSON from an assistant message
// This is a fallback when the 'result' event is missing or malformed
func (m *Manager) processAssistantMessage(rawMessage string, turn int, usage Usage, cost float64, duration int64, claudeSessionID string) error {
	m.debugLogger.Log("DEBUG", "Processing assistant message")
	
	// DEBUG: Log processing start
	m.debugLogger.Log("METRICS", fmt.Sprintf("processAssistantMessage called for turn %d", turn))
	
	// Parse the assistant message JSON
	var assistantMsg struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	
	// DEBUG: Log parsing assistant message
	m.debugLogger.Log("METRICS", "Parsing assistant message JSON")
	
	if err := json.Unmarshal([]byte(rawMessage), &assistantMsg); err != nil {
		return fmt.Errorf("failed to parse assistant message: %w", err)
	}
	
	// DEBUG: Log successful parse
	m.debugLogger.Log("METRICS", "Assistant message parsed successfully")
	
	// Extract text from content blocks
	var fullText strings.Builder
	for _, content := range assistantMsg.Message.Content {
		if content.Type == "text" {
			fullText.WriteString(content.Text)
		}
	}
	
	// DEBUG: Log text extraction
	m.debugLogger.Log("METRICS", fmt.Sprintf("Extracted %d characters from assistant message", fullText.Len()))
	
	text := fullText.String()
	if text == "" {
		return fmt.Errorf("no text content found in assistant message")
	}
	
	// DEBUG: Log text content
	m.debugLogger.Log("METRICS", fmt.Sprintf("Text content length: %d", len(text)))
	
	// Strip markdown code fences
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimSpace(text)
	}
	
	// DEBUG: Log after stripping json fence
	m.debugLogger.Log("METRICS", fmt.Sprintf("After stripping json fence: %d chars", len(text)))
	
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSpace(text)
	}
	
	// DEBUG: Log after stripping fence
	m.debugLogger.Log("METRICS", fmt.Sprintf("After stripping fence: %d chars", len(text)))
	
	if strings.HasSuffix(text, "```") {
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	
	// DEBUG: Log after stripping suffix
	m.debugLogger.Log("METRICS", fmt.Sprintf("After stripping suffix: %d chars", len(text)))
	
	// Find JSON between outermost braces
	jsonStart := strings.Index(text, "{")
	if jsonStart == -1 {
		return fmt.Errorf("no JSON object found in assistant message")
	}
	
	// DEBUG: Log JSON start position
	m.debugLogger.Log("METRICS", fmt.Sprintf("JSON start position: %d", jsonStart))
	
	// Find matching closing brace
	braceCount := 0
	jsonEnd := -1
	for i := jsonStart; i < len(text); i++ {
		if text[i] == '{' {
			braceCount++
		} else if text[i] == '}' {
			braceCount--
			if braceCount == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}
	
	// DEBUG: Log JSON end position
	m.debugLogger.Log("METRICS", fmt.Sprintf("JSON end position: %d", jsonEnd))
	
	if jsonEnd == -1 {
		return fmt.Errorf("unmatched braces in JSON")
	}
	
	jsonStr := text[jsonStart:jsonEnd]
	
	// DEBUG: Log JSON string length
	m.debugLogger.Log("METRICS", fmt.Sprintf("JSON string length: %d", len(jsonStr)))
	
	// Validate JSON syntax
	var jsontest interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsontest); err != nil {
		return fmt.Errorf("invalid JSON syntax: %w", err)
	}
	
	// DEBUG: Log JSON validation
	m.debugLogger.Log("METRICS", "JSON syntax validated")
	
	// Determine turn type from session state
	turnType := "discovery" // Default
	if len(m.session.Turns) > 0 {
		lastTurn := m.session.Turns[len(m.session.Turns)-1]
		turnType = lastTurn.TurnType
	}

	// Delegate to specific parser
	var results *TurnResults
	var parseErr error
	
	switch turnType {
	case "discovery":
		results, parseErr = ParseDiscoveryResult(jsonStr)
	case "validation":
		results, parseErr = ParseValidationResult(jsonStr)
	case "change":
		results, parseErr = ParseChangeResult(jsonStr)
	default:
		parseErr = fmt.Errorf("unknown turn type: %s", turnType)
	}

	if parseErr != nil {
		return fmt.Errorf("failed to parse assistant message: %w", parseErr)
	}

	// Calculate total found for populateTurnState
	totalFound := 0
	if results.Candidates != nil {
		totalFound = len(results.Candidates)
	}
	// For change turns, use FilesModifiedCount if available
	if results.ChangeResults != nil {
		totalFound = results.ChangeResults.ChangeSummary.FilesModifiedCount
	}

	// Populate session state
	m.populateTurnState(turn, results.Candidates, totalFound, usage, cost, duration, claudeSessionID, results)
	
	m.session.Status = turnType + "_complete"
	m.writeSessionState()
	
	m.debugLogger.Log("DEBUG", "Results processed successfully")
	return nil
}

// populateTurnState is a helper to populate the session state for a turn
func (m *Manager) populateTurnState(turn int, candidates []Candidate, totalFound int, usage Usage, cost float64, duration int64, claudeSessionID string, results *TurnResults) {
	// DEBUG: Log populateTurnState entry
	m.debugLogger.Log("METRICS", fmt.Sprintf(
		"populateTurnState ENTRY - Turn: %d, Duration: %dms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d, Candidates: %d",
		turn,
		duration,
		cost,
		usage.InputTokens,
		usage.OutputTokens,
		len(candidates),
	))
	
	// Find the current turn in session
	var turnState *TurnState
	for i := range m.session.Turns {
		if m.session.Turns[i].TurnNumber == turn {
			turnState = &m.session.Turns[i]
			break
		}
	}
	
	// DEBUG: Log turn state found
	if turnState != nil {
		m.debugLogger.Log("METRICS", fmt.Sprintf("Turn state found for turn %d", turn))
	} else {
		m.debugLogger.Log("METRICS", fmt.Sprintf("WARNING: No turn state found for turn %d", turn))
	}
	
	if turnState == nil {
		return
	}
	
	// Populate quick candidates list
	turnState.Candidates = make([]QuickCandidate, len(candidates))
	for i, cand := range candidates {
		turnState.Candidates[i] = QuickCandidate{
			WorkdirID:   cand.WorkdirID,
			WorkdirName: cand.WorkdirName,
			FilePath:    cand.FilePath,
			Score:       cand.Score,
		}
	}
	turnState.TotalFound = totalFound
	
	// DEBUG: Log candidates populated
	m.debugLogger.Log("METRICS", fmt.Sprintf("Populated %d candidates, TotalFound: %d", len(candidates), totalFound))
	
	// Populate full results
	turnState.Results = results
	
	// DEBUG: Log results populated
	m.debugLogger.Log("METRICS", "Results populated")
	
	// Populate usage/cost/duration
	turnState.Usage = &usage
	turnState.Cost = &cost
	turnState.Duration = &duration
	if claudeSessionID != "" {
		turnState.ClaudeSessionID = &claudeSessionID
	}
	
	// DEBUG: Log metrics set in turn state
	m.debugLogger.Log("METRICS", fmt.Sprintf(
		"METRICS SET IN TURN STATE - Duration: %dms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d",
		duration,
		cost,
		usage.InputTokens,
		usage.OutputTokens,
	))
	
	// DEBUG: Log turn state values
	if turnState.Usage != nil {
		m.debugLogger.Log("METRICS", fmt.Sprintf(
			"TURN STATE VALUES - Usage.InputTokens: %d, Usage.OutputTokens: %d, Cost: $%.6f, Duration: %dms",
			turnState.Usage.InputTokens,
			turnState.Usage.OutputTokens,
			*turnState.Cost,
			*turnState.Duration,
		))
	} else {
		m.debugLogger.Log("METRICS", "WARNING: turnState.Usage is nil")
	}
}
