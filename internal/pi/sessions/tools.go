/**
 * Component: Pi Session Tool Call Parser
 * Block-UUID: b2c3d4e5-f6a7-8901-bcde-f12345678901
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Extracts and normalizes tool calls from Pi session JSONL active branch.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package sessions

import (
	"encoding/json"
	"path/filepath"
)

// ToolCall represents a normalized tool call with its result.
type ToolCall struct {
	ToolCallID   string          `json:"toolCallId"`
	ToolName     string          `json:"toolName"`
	Arguments    json.RawMessage `json:"arguments,omitempty"`
	CallEntryID  string          `json:"callEntryId"`
	CallMsgID    string          `json:"callMessageId,omitempty"`
	ResultEntryID string         `json:"resultEntryId,omitempty"`
	IsError      bool            `json:"isError"`
	ResultText   string          `json:"resultText,omitempty"`
	Timestamp    string          `json:"timestamp"`
}

// ToolCallsResult represents tool calls from the active branch.
type ToolCallsResult struct {
	Session   SessionInfo `json:"session"`
	Leaf      string      `json:"leaf"`
	ToolCalls []ToolCall  `json:"toolCalls"`
}

// ExtractToolCalls reads a session JSONL file and returns tool calls from the active branch.
func ExtractToolCalls(sessionPath string, leafID string) (*ToolCallsResult, error) {
	// Get the branch
	branch, err := WalkBranch(sessionPath, leafID)
	if err != nil {
		return nil, err
	}

	// Collect tool calls from assistant messages
	type pendingCall struct {
		toolCallID string
		toolName   string
		arguments  json.RawMessage
		entryID    string
		msgID      string
		timestamp  string
	}

	pending := make(map[string]pendingCall)
	var toolCalls []ToolCall

	for _, entry := range branch.Entries {
		if entry.Message == nil {
			continue
		}

		var msg messagePayload
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		// Process assistant messages with tool calls
		if msg.Role == "assistant" {
			for _, block := range msg.Content {
				if block.Type == "toolCall" && block.ID != "" {
					pending[block.ID] = pendingCall{
						toolCallID: block.ID,
						toolName:   block.Name,
						arguments:  block.Arguments,
						entryID:    entry.ID,
						timestamp:  entry.Timestamp,
					}
				}
			}
		}

		// Process tool results
		if msg.Role == "toolResult" && msg.ToolCallID != "" {
			if call, ok := pending[msg.ToolCallID]; ok {
				tc := ToolCall{
					ToolCallID:    call.toolCallID,
					ToolName:      call.toolName,
					Arguments:     call.arguments,
					CallEntryID:   call.entryID,
					ResultEntryID: entry.ID,
					IsError:       msg.IsError,
					ResultText:    flattenMessageText(&msg),
					Timestamp:     call.timestamp,
				}
				toolCalls = append(toolCalls, tc)
				delete(pending, msg.ToolCallID)
			}
		}
	}

	// Add any pending calls without results
	for _, call := range pending {
		tc := ToolCall{
			ToolCallID:  call.toolCallID,
			ToolName:    call.toolName,
			Arguments:   call.arguments,
			CallEntryID: call.entryID,
			Timestamp:   call.timestamp,
		}
		toolCalls = append(toolCalls, tc)
	}

	absPath, _ := filepath.Abs(sessionPath)

	return &ToolCallsResult{
		Session: SessionInfo{
			Path:    absPath,
			ID:      branch.Session.ID,
			Version: branch.Session.Version,
			CWD:     branch.Session.CWD,
		},
		Leaf:      leafID,
		ToolCalls: toolCalls,
	}, nil
}
