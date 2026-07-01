/**
 * Component: Pi Session JSONL Parser
 * Block-UUID: 0f9da76e-5b96-4339-89d6-4d6a20e86254
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Parses complete Pi session JSONL lines losslessly from the file start or a committed byte offset.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0, v1.1.0)
 */


package sessions

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type parsedSession struct {
	header           sessionHeader
	entries          []parsedEntry
	syncedByteOffset int64
	hasPartialTail   bool
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

	reader := bufio.NewReaderSize(file, 1024*1024)
	headerLine, headerBytes, complete, err := readCompleteLine(reader)
	if err != nil {
		return parsedSession{}, err
	}
	if !complete {
		return parsedSession{}, fmt.Errorf("incomplete session header")
	}
	var header sessionHeader
	if err := json.Unmarshal([]byte(headerLine), &header); err != nil {
		return parsedSession{}, fmt.Errorf("parse header: %w", err)
	}
	if header.Type != "session" || header.UUID == "" {
		return parsedSession{}, fmt.Errorf("invalid session header")
	}
	header.RawLine = headerLine
	entries, consumed, partial, err := parseCompleteEntries(reader, 0)
	if err != nil {
		return parsedSession{}, err
	}
	return parsedSession{
		header:           header,
		entries:          entries,
		syncedByteOffset: headerBytes + consumed,
		hasPartialTail:   partial,
	}, nil
}

func parseSessionHeader(path string) (sessionHeader, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return sessionHeader{}, 0, err
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1024*1024)
	line, consumed, complete, err := readCompleteLine(reader)
	if err != nil {
		return sessionHeader{}, 0, err
	}
	if !complete {
		return sessionHeader{}, 0, fmt.Errorf("incomplete session header")
	}
	var header sessionHeader
	if err := json.Unmarshal([]byte(line), &header); err != nil {
		return sessionHeader{}, 0, fmt.Errorf("parse header: %w", err)
	}
	if header.Type != "session" || header.UUID == "" {
		return sessionHeader{}, 0, fmt.Errorf("invalid session header")
	}
	header.RawLine = line
	return header, consumed, nil
}

func parseSessionTail(path string, offset int64, seq int) ([]parsedEntry, int64, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, false, err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, false, err
	}
	entries, consumed, partial, err := parseCompleteEntries(bufio.NewReaderSize(file, 1024*1024), seq)
	return entries, offset + consumed, partial, err
}

func parseCompleteEntries(reader *bufio.Reader, startSeq int) ([]parsedEntry, int64, bool, error) {
	var entries []parsedEntry
	var consumed int64
	seq := startSeq
	for {
		line, lineBytes, complete, err := readCompleteLine(reader)
		if err != nil {
			return nil, consumed, false, err
		}
		if !complete {
			return entries, consumed, line != "", nil
		}
		consumed += lineBytes
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry parsedEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, consumed, false, fmt.Errorf("parse entry seq %d: %w", seq, err)
		}
		if entry.ID == "" {
			return nil, consumed, false, fmt.Errorf("entry seq %d missing id", seq)
		}
		entry.RawLine = line
		entry.RawHash = hashText(line)
		entry.Seq = seq
		entries = append(entries, entry)
		seq++
	}
}

func readCompleteLine(reader *bufio.Reader) (string, int64, bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", 0, false, err
	}
	if err == io.EOF {
		return line, 0, false, nil
	}
	return strings.TrimSuffix(line, "\n"), int64(len(line)), true, nil
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
