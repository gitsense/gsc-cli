/**
 * Component: Pi Session JSONL Parser
 * Block-UUID: 0f9da76e-5b96-4339-89d6-4d6a20e86254
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Parses Pi session JSONL files into lossless raw lines and extracted phase-one import records.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */


package sessions

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type parsedSession struct {
	header  sessionHeader
	entries []parsedEntry
}

type sessionHeader struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	UUID          string `json:"id"`
	Timestamp     string `json:"timestamp"`
	CWD           string `json:"cwd"`
	ParentSession string `json:"parentSession"`
	RawLine       string
}

type parsedEntry struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	ParentID  *string         `json:"parentId"`
	Timestamp string          `json:"timestamp"`
	Message   *messagePayload `json:"message"`
	Provider  string          `json:"provider"`
	ModelID   string          `json:"modelId"`
	FromID    string          `json:"fromId"`
	Details   detailsPayload  `json:"details"`
	Name      string          `json:"name"`
	RawLine   string
	RawHash   string
	Seq       int
}

type messagePayload struct {
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	API        string         `json:"api"`
	Provider   string         `json:"provider"`
	Model      string         `json:"model"`
	ToolCallID string         `json:"toolCallId"`
	ToolName   string         `json:"toolName"`
	IsError    bool           `json:"isError"`
	Command    string         `json:"command"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type detailsPayload struct {
	ReadFiles     []string `json:"readFiles"`
	ModifiedFiles []string `json:"modifiedFiles"`
}

func parseSessionFile(path string) (parsedSession, error) {
	file, err := os.Open(path)
	if err != nil {
		return parsedSession{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return parsedSession{}, err
		}
		return parsedSession{}, fmt.Errorf("empty session file")
	}

	headerLine := scanner.Text()
	var header sessionHeader
	if err := json.Unmarshal([]byte(headerLine), &header); err != nil {
		return parsedSession{}, fmt.Errorf("parse header: %w", err)
	}
	if header.Type != "session" || header.UUID == "" {
		return parsedSession{}, fmt.Errorf("invalid session header")
	}
	header.RawLine = headerLine

	var entries []parsedEntry
	seq := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry parsedEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return parsedSession{}, fmt.Errorf("parse entry seq %d: %w", seq, err)
		}
		if entry.ID == "" {
			return parsedSession{}, fmt.Errorf("entry seq %d missing id", seq)
		}
		entry.RawLine = line
		entry.RawHash = hashText(line)
		entry.Seq = seq
		entries = append(entries, entry)
		seq++
	}
	if err := scanner.Err(); err != nil {
		return parsedSession{}, err
	}

	return parsedSession{header: header, entries: entries}, nil
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func flattenMessageText(message *messagePayload) string {
	if message == nil {
		return ""
	}
	var parts []string
	if message.Role == "bashExecution" && message.Command != "" {
		parts = append(parts, message.Command)
	}
	for _, block := range message.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "thinking":
			if block.Thinking != "" {
				parts = append(parts, block.Thinking)
			}
		case "custom":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
