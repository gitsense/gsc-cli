/**
 * Component: Claude Code Archive Manager
 * Block-UUID: 24354d07-2ed7-4b20-85ad-2fbd58c383bb
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the Tiered Rolling Archive logic for managing conversation history, including chunking, context isolation, and hash-based change detection.
 * Language: Go
 * Created-at: 2026-03-22T03:46:45.910Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// SyncArchive reconstructs the file-based state for a chat session.
// It filters messages, writes context files, chunks the dialogue history,
// and updates the active window.
func SyncArchive(chatDir string, messages []db.Message, settings Settings) ([]ArchiveFile, error) {
	messagesDir := filepath.Join(chatDir, "messages")
	if err := os.MkdirAll(messagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create messages directory: %w", err)
	}

	// 1. Filter and Separate Messages
	var dialogueMessages []db.Message
	var contextMessages []db.Message

	for _, msg := range messages {
		// Filter by visibility
		if msg.Visibility != "public" {
			continue
		}

		// Separate context messages
		if msg.Type == "context" {
			contextMessages = append(contextMessages, msg)
		} else {
			dialogueMessages = append(dialogueMessages, msg)
		}
	}

	// 2. Write Context Files
	if err := writeContextFiles(messagesDir, contextMessages); err != nil {
		return nil, fmt.Errorf("failed to write context files: %w", err)
	}

	// 3. Chunk Dialogue Messages
	// We process messages in reverse (newest first) to easily grab the active window,
	// then reverse the rest for archiving.
	
	// Sort messages by ID to ensure chronological order
	sort.Slice(dialogueMessages, func(i, j int) bool {
		return dialogueMessages[i].ID < dialogueMessages[j].ID
	})

	// Separate Active Window (last N messages)
	activeCount := settings.ChunkSize
	var activeMessages []db.Message
	var archiveMessages []db.Message

	if len(dialogueMessages) > activeCount {
		splitIndex := len(dialogueMessages) - activeCount
		archiveMessages = dialogueMessages[:splitIndex]
		activeMessages = dialogueMessages[splitIndex:]
	} else {
		activeMessages = dialogueMessages
	}

	// 4. Write Archive Chunks
	archiveFiles, err := writeArchiveChunks(messagesDir, archiveMessages, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to write archive chunks: %w", err)
	}

	// 5. Write Active Window
	if err := writeActiveWindow(messagesDir, activeMessages, archiveFiles); err != nil {
		return nil, fmt.Errorf("failed to write active window: %w", err)
	}

	return archiveFiles, nil
}

// writeContextFiles writes individual context messages to markdown files.
func writeContextFiles(dir string, messages []db.Message) error {
	for _, msg := range messages {
		filename := fmt.Sprintf("context-%d.md", msg.ID)
		path := filepath.Join(dir, filename)

		content := ""
		if msg.Message.Valid {
			content = msg.Message.String
		}

		// Check hash to avoid unnecessary writes
		currentHash := calculateHash(content)
		if existingHash, err := getFileHash(path); err == nil && existingHash == currentHash {
			continue // File hasn't changed
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write context file %s: %w", filename, err)
		}
	}
	return nil
}

// writeArchiveChunks chunks the historical messages and writes them to JSON files.
func writeArchiveChunks(dir string, messages []db.Message, settings Settings) ([]ArchiveFile, error) {
	var archiveFiles []ArchiveFile

	// Simple chunking strategy: split into chunks of ChunkSize
	for i := 0; i < len(messages); i += settings.ChunkSize {
		end := i + settings.ChunkSize
		if end > len(messages) {
			end = len(messages)
		}

		chunk := messages[i:end]
		chunkNum := (i / settings.ChunkSize) + 1
		filename := fmt.Sprintf("messages-archive-%d.json", chunkNum)
		path := filepath.Join(dir, filename)

		// Convert to MessageFile format
		var msgFiles []MessageFile
		for _, m := range chunk {
			content := ""
			if m.Message.Valid {
				content = m.Message.String
			}
			msgFiles = append(msgFiles, MessageFile{
				Role:    m.Role,
				Content: content,
			})
		}

		// Serialize
		data, err := json.MarshalIndent(msgFiles, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal chunk %d: %w", chunkNum, err)
		}

		// Check hash
		currentHash := calculateHash(string(data))
		if existingHash, err := getFileHash(path); err == nil && existingHash == currentHash {
			// File exists and hasn't changed, just add to list
			archiveFiles = append(archiveFiles, ArchiveFile{
				Name:     filename,
				Hash:     currentHash,
				Messages: len(chunk),
			})
			continue
		}

		// Write file
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write archive chunk %s: %w", filename, err)
		}

		archiveFiles = append(archiveFiles, ArchiveFile{
			Name:     filename,
			Hash:     currentHash,
			Messages: len(chunk),
		})
	}

	// TODO: Implement Merge Logic here if len(archiveFiles) > settings.MaxFiles
	// This would combine the oldest chunks into a single large archive.

	return archiveFiles, nil
}

// writeActiveWindow writes the recent messages and the archive map to messages-active.json.
func writeActiveWindow(dir string, messages []db.Message, archiveFiles []ArchiveFile) error {
	path := filepath.Join(dir, "messages-active.json")

	// Convert messages
	var msgFiles []MessageFile
	for _, m := range messages {
		content := ""
		if m.Message.Valid {
			content = m.Message.String
		}
		msgFiles = append(msgFiles, MessageFile{
			Role:    m.Role,
			Content: content,
		})
	}

	// Construct Active Window
	window := ActiveWindow{
		ArchiveMap: ArchiveMap{Files: archiveFiles},
		Messages:   msgFiles,
	}

	data, err := json.MarshalIndent(window, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal active window: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// calculateHash generates a SHA256 hash of a string.
func calculateHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// getFileHash reads a file and returns its SHA256 hash.
func getFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return calculateHash(string(data)), nil
}
