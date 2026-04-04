/**
 * Component: Scout Stream Event Processor
 * Block-UUID: 925a054f-3dd3-4864-8c16-27858cc5f9a2
 * Parent-UUID: 472f75bd-f981-4869-8a80-a45c60f3b8aa
 * Version: 1.1.0
 * Description: Manages Claude output stream parsing, event handling, and state updates from streaming JSONL responses
 * Language: Go
 * Created-at: 2026-04-04T16:05:03.805Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0)
 */


package scout

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

		case "usage":
			// Parse usage event
			if usageData, ok := result["usage"].(map[string]interface{}); ok {
				usage = Usage{
					InputTokens:         int(usageData["input_tokens"].(float64)),
					OutputTokens:        int(usageData["output_tokens"].(float64)),
					CacheCreationTokens: int(usageData["cache_creation_input_tokens"].(float64)),
					CacheReadTokens:     int(usageData["cache_read_input_tokens"].(float64)),
				}
				if c, ok := result["cost"].(float64); ok {
					cost = c
				}
			}
			m.debugLogger.LogStreamEvent("USAGE", fmt.Sprintf("line %d: tokens=%d/%d", lineCount, usage.InputTokens, usage.OutputTokens))

		case "error":
			// Handle error events from Claude CLI
			if errorData, ok := result["error"].(map[string]interface{}); ok {
				errorType, _ := errorData["type"].(string)
				errorMsg, _ := errorData["message"].(string)
				m.eventWriter.WriteErrorEvent(ErrorEvent{
					Phase:     fmt.Sprintf("turn-%d", turn),
					ErrorCode: errorType,
					Message:   errorMsg,
					Details:   line,
				})
				m.debugLogger.LogError("Claude error event", fmt.Errorf("%s: %s", errorType, errorMsg))
				m.session.Status = "error"
				m.writeSessionState()
			}
			continue

		case "system":
			// Handle system events (API retries, etc.)
			m.debugLogger.LogStreamEvent("SYSTEM", fmt.Sprintf("line %d", lineCount))
			// Log but don't fail - these are informational
			continue

		case "result":
			// Parse result event
			if resultContent, ok := result["result"].(string); ok {
				// Check for error in result event
				if isError, ok := result["is_error"].(bool); ok && isError {
					m.debugLogger.LogError("Claude result error", fmt.Errorf("%v", result))
					m.eventWriter.WriteErrorEvent(ErrorEvent{
						Phase:     fmt.Sprintf("turn-%d", turn),
						ErrorCode: "RESULT_ERROR",
						Message:   fmt.Sprintf("Claude returned error: %v", result),
						Details:   line,
					})
					m.session.Status = "error"
					m.writeSessionState()
					continue
				}

				// Extract usage/cost from outer event
				if usageData, ok := result["usage"].(map[string]interface{}); ok {
					usage = Usage{
						InputTokens:         int(usageData["input_tokens"].(float64)),
						OutputTokens:        int(usageData["output_tokens"].(float64)),
						CacheCreationTokens: int(usageData["cache_creation_input_tokens"].(float64)),
						CacheReadTokens:     int(usageData["cache_read_input_tokens"].(float64)),
					}
				}
				if c, ok := result["total_cost_usd"].(float64); ok {
					cost = c
				}
				if d, ok := result["duration_ms"].(float64); ok {
					duration = int64(d)
				}
				if sid, ok := result["session_id"].(string); ok {
					claudeSessionID = sid
				}
				m.debugLogger.LogStreamEvent("RESULT", fmt.Sprintf("line %d: result length=%d", lineCount, len(resultContent)))

				// Parse Scout's JSON from result field
				// Try Turn 1 format first
				var turn1Result struct {
					Candidates []Candidate `json:"candidates"`
					TotalFound int         `json:"total_found"`
					Coverage   string      `json:"coverage"`
				}
				if err := json.Unmarshal([]byte(resultContent), &turn1Result); err == nil {
					// Write candidates event
					totalFound := turn1Result.TotalFound
					if totalFound == 0 {
						totalFound = 0 // Explicitly 0
					}

					m.eventWriter.WriteCandidatesEvent(CandidatesEvent{
						Phase:      "discovery",
						TotalFound: totalFound,
						Candidates: turn1Result.Candidates,
					})

					m.debugLogger.LogEventWrite("candidates", true, nil)
					// Write done event with usage/cost
					m.eventWriter.WriteDoneEvent(DoneEvent{
						Status:         "success",
						TotalCandidates: totalFound,
						PhaseCompleted: "discovery",
						Summary: CompletionSummary{
							FilesFound: totalFound,
						},
						Usage:           &usage,
						Cost:            &cost,
						Duration:        &duration,
						ClaudeSessionID: &claudeSessionID,
					})

					m.debugLogger.LogEventWrite("done", true, nil)
					// Update session status
					m.session.Status = "discovery_complete"
					m.writeSessionState()
					break
				}

				// Try Turn 2 format
				var turn2Result struct {
					VerifiedCandidates []VerificationUpdate `json:"verified_candidates"`
					Summary            struct {
						TotalVerified        int     `json:"total_verified"`
						CandidatesPromoted   int     `json:"candidates_promoted"`
						CandidatesDemoted    int     `json:"candidates_demoted"`
						CandidatesRemoved    int     `json:"candidates_removed"`
						AverageVerifiedScore float64 `json:"average_verified_score"`
						TopCandidatesCount   int     `json:"top_candidates_count"`
					} `json:"summary"`
				}
				if err := json.Unmarshal([]byte(resultContent), &turn2Result); err == nil {
					// Write verified event
					m.eventWriter.WriteVerifiedEvent(VerifiedEvent{
						Phase:             "verification",
						TotalVerified:     turn2Result.Summary.TotalVerified,
						UpdatedCandidates: turn2Result.VerifiedCandidates,
					})

					m.debugLogger.LogEventWrite("verified", true, nil)
					// Write done event with usage/cost
					m.eventWriter.WriteDoneEvent(DoneEvent{
						Status:         "success",
						TotalCandidates: turn2Result.Summary.TotalVerified,
						PhaseCompleted: "verification",
						Summary: CompletionSummary{
							FilesFound: turn2Result.Summary.TotalVerified,
						},
						Usage:           &usage,
						Cost:            &cost,
						Duration:        &duration,
						ClaudeSessionID: &claudeSessionID,
					})

					m.debugLogger.LogEventWrite("done", true, nil)
					// Update session status
					m.session.Status = "verification_complete"
					m.writeSessionState()
					break
				}
			}
		default:
			// Log unknown event types for debugging
			m.debugLogger.LogStreamEvent("UNKNOWN_EVENT", fmt.Sprintf("line %d: type=%s", lineCount, eventType))
			m.eventWriter.WriteRawEvent(line)
		}
	}

	// Handle scanner errors
	if err := scanner.Err(); err != nil {
		m.debugLogger.LogError("Scanner error", err)
		m.debugLogger.Log("STREAM", fmt.Sprintf("Scanner error details: %v", err))
		m.eventWriter.WriteErrorEvent(ErrorEvent{
			Phase:     fmt.Sprintf("turn-%d", turn),
			ErrorCode: "STREAM_ERROR",
			Message:   fmt.Sprintf("Error reading stream: %v", err),
		})
		m.session.Status = "error"
		m.writeSessionState()
	}
	m.debugLogger.Log("STREAM", fmt.Sprintf("Stream processing complete: %d lines processed", lineCount))
}

// captureStderr reads and logs stderr from the subprocess
func (m *Manager) captureStderr(stderr io.Reader) {
	m.debugLogger.Log("DEBUG", "Stderr capture started")
	
	// Open output.log for writing raw stderr
	outputLogPath := m.config.GetTurnDir(m.currentTurn) + "/output.log"
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
			outputLogFile.WriteString(fmt.Sprintf("[%s] [STDERR] %s\n", timestamp, line))
		}
	}
	
	defer func() {
		m.debugLogger.Log("DEBUG", "Stderr capture ended")
		m.wg.Done()
	}()

	scanner := bufio.NewScanner(stderr)
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()
		
		// Write to output.log
		writeToOutputLog(line)
		
		m.debugLogger.Log("STDERR", fmt.Sprintf("line %d: %s", lineCount, line))
	}

	if err := scanner.Err(); err != nil {
		m.debugLogger.LogError("Stderr scanner error", err)
		m.debugLogger.Log("STDERR", fmt.Sprintf("Stderr scanner error: %v", err))
	}

	if lineCount > 0 {
		m.debugLogger.Log("STDERR", fmt.Sprintf("Stderr capture complete: %d lines captured", lineCount))
	} else {
		m.debugLogger.Log("STDERR", "Stderr capture complete: no output")
	}
}
