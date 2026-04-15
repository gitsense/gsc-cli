/**
 * Component: Claude Code Chat Stream Event Processor
 * Block-UUID: 7f1d4e7f-1c2d-4751-b903-06e28e0d59ea
 * Parent-UUID: 529c3037-f4cb-4846-8ad8-56f5fd3aed21
 * Version: 1.2.3
 * Description: Extract stream processing logic into dedicated processor module for improved separation of concerns and reusability. Moved StreamResult and StreamProcessor to types.go. Added structured event logging, dual output streams, and comprehensive debug logging for improved stream handling.
 * Language: Go
 * Created-at: 2026-04-14T21:16:18.630Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1), Gemini 2.5 Flash Lite (v1.2.2), GLM-4.7 (v1.2.3)
 */


package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/claude"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// TextDeltaEvent represents a chunk of text content.
type TextDeltaEvent struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

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

// ContentBlockDeltaEvent represents a streaming content block delta event with thinking or text
type ContentBlockDeltaEvent struct {
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

// StreamUsageEvent represents the final usage metrics.
type StreamUsageEvent struct {
	Type  string `json:"type"`
	Usage claude.Usage `json:"usage"`
	Cost  float64 `json:"cost"`
}

// StreamResultEvent represents the final result event containing usage stats and cost.
type StreamResultEvent struct {
	Type       string                 `json:"type"`
	Result     string                 `json:"result"`
	Subtype    string                 `json:"subtype"`
	DurationMs int                    `json:"duration_ms"`
	StopReason string                 `json:"stop_reason"`
	Usage      claude.Usage           `json:"usage"`
	ModelUsage map[string]ModelStats  `json:"modelUsage"`
	TotalCost  float64                `json:"total_cost_usd"`
}

// ModelStats represents per-model usage details.
type ModelStats struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens"`
	CostUSD                  float64 `json:"costUSD"`
	ContextWindow            int     `json:"contextWindow"`
	MaxOutputTokens          int     `json:"maxOutputTokens"`
}

// SystemInitEvent represents the first event from Claude containing session info
type SystemInitEvent struct {
	Type              string `json:"type"`
	Subtype           string `json:"subtype"`
	Model             string `json:"model"`
	SessionID         string `json:"session_id"`
	CWD               string `json:"cwd"`
	UUID              string `json:"uuid"`
	ClaudeCodeVersion string `json:"claude_code_version"`
}

// processStream handles the stream event processing loop
func (sp *StreamProcessor) processStream(stdout io.Reader, logDir string) (StreamResult, error) {
	result := StreamResult{
		ExitCode: 0,
	}

	// NEW: Initialize debug logger (disabled by default, can be enabled via flag)
	debugLogger, err := NewChatDebugLogger(logDir, false) // Set to true to enable
	if err != nil {
		logger.Warning("Failed to create debug logger", "error", err)
	}
	sp.DebugLogger = debugLogger
	defer debugLogger.Close()

	// NEW: Initialize event writer
	eventLogFilename := fmt.Sprintf("events-%s.ndjson", time.Now().Format("20060102-150405"))
	eventLogPath := filepath.Join(logDir, eventLogFilename)
	eventWriter, err := NewChatEventWriter(eventLogPath)
	if err != nil {
		logger.Warning("Failed to create event writer", "error", err)
	}
	sp.EventWriter = eventWriter
	defer eventWriter.Close()

	// NEW: Open raw output log for dual stream output
	outputLogFilename := fmt.Sprintf("output-%s.log", time.Now().Format("20060102-150405"))
	outputLogPath := filepath.Join(logDir, outputLogFilename)
	outputLogFile, err := os.OpenFile(outputLogPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, FilePermissions)
	if err != nil {
		logger.Warning("Failed to create output log", "error", err)
	}
	defer outputLogFile.Close()

	// Helper function to write to output log
	writeToOutputLog := func(line string) {
		if outputLogFile != nil {
			timestamp := time.Now().UTC().Format(time.RFC3339Nano)
			outputLogFile.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, line))
		}
	}

	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, InitialBufSize)
	scanner.Buffer(buf, MaxTokenSize)

	var fullResponse strings.Builder
	var nonJSONOutput strings.Builder
	var toolsFinished bool
	var responseBuffer strings.Builder
	var isFirstLine = true
	var metricsWritten = false

	// Defer cleanup
	defer func() {
		if nonJSONOutput.Len() > 0 {
			nonJSONPath := filepath.Join(logDir, "debug-stdout-non-json.txt")
			if writeErr := os.WriteFile(nonJSONPath, []byte(nonJSONOutput.String()), FilePermissions); writeErr != nil {
				logger.Warning("Failed to write debug non-JSON stdout file", "error", writeErr)
			}
		}

		if !metricsWritten {
			stackTrace := debug.Stack()
			errorMsg := fmt.Sprintf("processStream returned before completion.\n\nStack Trace:\n%s", string(stackTrace))
			errorPath := filepath.Join(logDir, fmt.Sprintf("error-stream-%s.txt", time.Now().Format("20060102-150405")))
			if writeErr := os.WriteFile(errorPath, []byte(errorMsg), FilePermissions); writeErr != nil {
				fmt.Fprintf(os.Stderr, "CRITICAL: Failed to write error log: %v\n", writeErr)
			}
		}
	}()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// NEW: Write raw line to output log
		writeToOutputLog(line)

		// Write raw line to log file
		if _, err := sp.LogFile.WriteString(line + "\n"); err != nil {
			logger.Warning("Failed to write to raw stream log", "error", err)
		}

		// Parse event
		var baseEvent claude.StreamEvent
		if err := json.Unmarshal([]byte(line), &baseEvent); err != nil {
			// NEW: Log parsing error
			sp.DebugLogger.LogStreamEvent("JSON_PARSE_ERROR", fmt.Sprintf("Failed to parse: %v", err))
			logger.Warning("Failed to parse stream event line", "line", line, "error", err)
			nonJSONOutput.WriteString(line + "\n")
			continue
		}

		// NEW: Log successful parse
		sp.DebugLogger.LogStreamEvent("EVENT_PARSED", fmt.Sprintf("Type: %s", baseEvent.Type))

		// Handle Init Event
		if isFirstLine && baseEvent.Type == "system" {
			sp.handleInitEvent(line, &result)
			
			// NEW: Write structured init event
			if sp.EventWriter != nil {
				sp.EventWriter.WriteInitEvent(ChatInitEvent{
					ChatUUID:  "", // Need to pass this in or extract from context
					Model:     sp.EffectiveModel,
					SessionID: result.SessionID,
					Format:    sp.Format,
				})
			}
			
			isFirstLine = false
			continue
		}

		// Handle Text Delta
		if baseEvent.Type == "text_delta" {
			sp.handleTextDelta(line, &fullResponse, &responseBuffer, &toolsFinished)
			
			// NEW: Write structured text delta event
			if sp.EventWriter != nil {
				var deltaEvent TextDeltaEvent
				if err := json.Unmarshal([]byte(line), &deltaEvent); err == nil {
					sp.EventWriter.WriteTextDeltaEvent(ChatTextDeltaEvent{
						Delta: deltaEvent.Delta,
					})
				}
			}
			
			continue
		}

		// Handle Thinking / Tool Use
		if baseEvent.Type == "assistant" {
			sp.handleAssistantEvent(line)
			continue
		}

		// Handle Stream Wrapper
		if baseEvent.Type == "stream_event" {
			sp.handleStreamWrapper(line, &fullResponse, &responseBuffer, &toolsFinished)
			continue
		}

		// Handle Usage and Result events
		switch baseEvent.Type {
		case "usage":
			sp.handleUsageEvent(line, &result)
			
			// NEW: Write structured usage event
			if sp.EventWriter != nil {
				var usageEvent StreamUsageEvent
				if err := json.Unmarshal([]byte(line), &usageEvent); err == nil {
					sp.EventWriter.WriteUsageEvent(ChatUsageEvent{
						InputTokens:         usageEvent.Usage.InputTokens,
						OutputTokens:        usageEvent.Usage.OutputTokens,
						CacheCreationTokens: usageEvent.Usage.CacheCreationTokens,
						CacheReadTokens:     usageEvent.Usage.CacheReadTokens,
						Cost:                usageEvent.Cost,
					})
				}
			}
			
		case "error":
			logger.Error("Claude CLI stream error", "data", line)
			
			// NEW: Write structured error event
			if sp.EventWriter != nil {
				sp.EventWriter.WriteErrorEvent(ChatErrorEvent{
					Type:    "stream_error",
					Message: line,
				})
			}
			
		case "result":
			sp.handleResultEvent(line, &result)
			
			// NEW: Write structured result event
			if sp.EventWriter != nil {
				var resultEvent StreamResultEvent
				if err := json.Unmarshal([]byte(line), &resultEvent); err == nil {
					sp.EventWriter.WriteResultEvent(ChatResultEvent{
						Result:     resultEvent.Result,
						StopReason: resultEvent.StopReason,
						DurationMs: resultEvent.DurationMs,
					})
				}
			}
		}

		// Handle User Event - Signals end of thinking phase
		if baseEvent.Type == "user" {
			sp.handleUserEvent(&responseBuffer, &toolsFinished)
		}
	}

	if err := scanner.Err(); err != nil {
		// NEW: Log scanner error
		sp.DebugLogger.LogError("Scanner error", err)
		logger.Error("Stream scanner encountered an error", "error", err)
		fmt.Fprintln(os.Stderr, "Stream Error:", err)
		return result, fmt.Errorf("error reading claude output: %w", err)
	}

	result.FullResponse = fullResponse.String()
	metricsWritten = true

	// NEW: Log final metrics
	sp.DebugLogger.LogMetrics(fmt.Sprintf(
		"Final Metrics - Duration: %dms, Cost: $%.6f, InputTokens: %d, OutputTokens: %d",
		0, // Need to track duration
		result.Cost,
		result.Usage.InputTokens,
		result.Usage.OutputTokens,
	))

	return result, nil
}

// handleInitEvent processes the initial system event
func (sp *StreamProcessor) handleInitEvent(line string, result *StreamResult) {
	var initEvent SystemInitEvent
	if err := json.Unmarshal([]byte(line), &initEvent); err == nil {
		sp.EffectiveModel = initEvent.Model
		result.SessionID = initEvent.SessionID

		initJSON, _ := json.Marshal(map[string]interface{}{
			"event":      "init",
			"model":      sp.EffectiveModel,
			"session_id": result.SessionID,
		})
		fmt.Println(string(initJSON))
	} else {
		logger.Warning("Failed to unmarshal SystemInitEvent, attempting fallback extraction", "error", err)
		var rawEvent map[string]interface{}
		if rawErr := json.Unmarshal([]byte(line), &rawEvent); rawErr == nil {
			if sid, ok := rawEvent["session_id"].(string); ok && sid != "" {
				result.SessionID = sid
				logger.Info("Extracted session_id via fallback", "session_id", result.SessionID)
			}
			if model, ok := rawEvent["model"].(string); ok && model != "" {
				sp.EffectiveModel = model
			}
		}
	}
}

// handleTextDelta processes text delta events
func (sp *StreamProcessor) handleTextDelta(line string, fullResponse, responseBuffer *strings.Builder, toolsFinished *bool) {
	var deltaEvent TextDeltaEvent
	if err := json.Unmarshal([]byte(line), &deltaEvent); err == nil {
		modifiedDelta := replacePlaceholders(deltaEvent.Delta, sp.EffectiveModel, sp.CurrentTime)
		fullResponse.WriteString(modifiedDelta)

		if !*toolsFinished {
			responseBuffer.WriteString(modifiedDelta)
		} else {
			if sp.Format == "text" {
				fmt.Print(modifiedDelta)
			} else if sp.Format == "json" {
				cleanJSON, _ := json.Marshal(map[string]interface{}{
					"event": "text",
					"delta": modifiedDelta,
				})
				fmt.Println(string(cleanJSON))
			}
		}
	}
}

// handleAssistantEvent processes assistant message events
func (sp *StreamProcessor) handleAssistantEvent(line string) {
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
}

// handleStreamWrapper processes stream wrapper events
func (sp *StreamProcessor) handleStreamWrapper(line string, fullResponse, responseBuffer *strings.Builder, toolsFinished *bool) {
	var wrapperEvent ContentBlockDeltaEvent
	if err := json.Unmarshal([]byte(line), &wrapperEvent); err == nil {
		if wrapperEvent.Event.Type == "content_block_delta" &&
			wrapperEvent.Event.Delta.Type == "thinking_delta" {
			cleanJSON, _ := json.Marshal(map[string]interface{}{
				"event": "thinking",
				"delta": wrapperEvent.Event.Delta.Thinking,
			})
			fmt.Println(string(cleanJSON))
		}

		if wrapperEvent.Event.Delta.Type == "text_delta" {
			modifiedText := replacePlaceholders(wrapperEvent.Event.Delta.Text, sp.EffectiveModel, sp.CurrentTime)

			fullResponse.WriteString(modifiedText)
			if !*toolsFinished {
				responseBuffer.WriteString(modifiedText)
			} else {
				if sp.Format == "text" {
					fmt.Print(modifiedText)
				} else if sp.Format == "json" {
					cleanJSON, _ := json.Marshal(map[string]interface{}{
						"event": "text",
						"delta": modifiedText,
					})
					fmt.Println(string(cleanJSON))
				}
			}
		}
	}
}

// handleUsageEvent processes usage events
func (sp *StreamProcessor) handleUsageEvent(line string, result *StreamResult) {
	var usageEvent StreamUsageEvent
	if err := json.Unmarshal([]byte(line), &usageEvent); err == nil {
		result.Usage = usageEvent.Usage
		result.Cost = usageEvent.Cost
	}
}

// handleResultEvent processes the final result event
func (sp *StreamProcessor) handleResultEvent(line string, result *StreamResult) {
	var resultEvent StreamResultEvent
	if err := json.Unmarshal([]byte(line), &resultEvent); err == nil {
		result.Usage = resultEvent.Usage
		result.Cost = resultEvent.TotalCost

		doneJSON, _ := json.Marshal(map[string]interface{}{
			"event":  "done",
			"stats": map[string]interface{}{
				"type":                  resultEvent.Type,
				"result":                resultEvent.Result,
				"subtype":               resultEvent.Subtype,
				"duration_ms":           resultEvent.DurationMs,
				"stop_reason":           resultEvent.StopReason,
				"usage":                 resultEvent.Usage,
				"modelUsage":            resultEvent.ModelUsage,
				"total_cost_usd":        resultEvent.TotalCost,
				"model":                 sp.EffectiveModel,
				"session_id":            result.SessionID,
			},
			"result": resultEvent.Result,
		})
		fmt.Println(string(doneJSON))
	}
}

// handleUserEvent signals the end of the thinking phase
func (sp *StreamProcessor) handleUserEvent(responseBuffer *strings.Builder, toolsFinished *bool) {
	*toolsFinished = true

	if responseBuffer.Len() > 0 {
		bufferedText := responseBuffer.String()
		responseBuffer.Reset()

		if sp.Format == "text" {
			fmt.Print(bufferedText)
		} else if sp.Format == "json" {
			cleanJSON, _ := json.Marshal(map[string]interface{}{
				"event": "text",
				"delta": bufferedText,
			})
			fmt.Println(string(cleanJSON))
		}
	}
}
