/**
 * Component: Claude Code Archive Manager
 * Block-UUID: 94b5287f-caad-4145-9a98-a14f617f0b4d
 * Parent-UUID: 064b570e-2579-4980-8c47-275edf5107ff
 * Version: 1.7.0
 * Description: Integrated context parser and bucketer for cache-optimized context file construction. Implemented zombie cleanup for orphaned context files and updated messages.map generation with proper bucket metadata.
 * Language: Go
 * Created-at: 2026-03-24T05:30:53.297Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0)
 */


package claude

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gitsense/gsc-cli/internal/context"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// SyncArchive reconstructs the file-based state for a chat session.
// It filters messages, writes context files using bucket-based organization,
// isolates CLI outputs, chunks the dialogue history, and updates the messages.map.
func SyncArchive(chatDir string, messages []db.Message, settings Settings) ([]ArchiveFile, error) {
	messagesDir := filepath.Join(chatDir, "messages")
	if err := os.MkdirAll(messagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create messages directory: %w", err)
	}

	// 1. Filter and Separate Messages
	var dialogueMessages []db.Message
	var contextMessages []db.Message
	var cliOutputMessages []db.Message

	for _, msg := range messages {
		// Filter by visibility
		if msg.Visibility != "public" {
			continue
		}
		
		// Filter out empty user messages
		if msg.Role == "user" {
			if !msg.Message.Valid || strings.TrimSpace(msg.Message.String) == "" {
				continue
			}
		}

		// Separate message types
		switch msg.Type {
		case "context":
			contextMessages = append(contextMessages, msg)
		case "gsc-cli-output":
			cliOutputMessages = append(cliOutputMessages, msg)
		default:
			dialogueMessages = append(dialogueMessages, msg)
		}
	}

	// 2. Write Context Files using bucket-based organization
	if err := writeContextFiles(messagesDir, contextMessages); err != nil {
		return nil, fmt.Errorf("failed to write context files: %w", err)
	}

	// 3. Write CLI Output Files (always isolated)
	if err := writeCliOutputFiles(messagesDir, cliOutputMessages); err != nil {
		return nil, fmt.Errorf("failed to write CLI output files: %w", err)
	}

	// 4. Chunk Dialogue Messages
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

	// 5. Write Archive Chunks
	archiveFiles, err := writeArchiveChunks(messagesDir, archiveMessages, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to write archive chunks: %w", err)
	}

	// 6. Write Active Window
	if err := writeActiveWindow(messagesDir, activeMessages, archiveFiles); err != nil {
		return nil, fmt.Errorf("failed to write active window: %w", err)
	}

	// 7. Generate or Update messages.map
	// Extract context files for messages.map generation
	contextFiles := context.ExtractContextFiles(contextMessages)
	if err := writeMessagesMap(messagesDir, contextFiles, cliOutputMessages, archiveFiles); err != nil {
		return nil, fmt.Errorf("failed to write messages.map: %w", err)
	}

	return archiveFiles, nil
}

// writeContextFiles writes context messages using bucket-based organization.
// Uses the context parser to extract files and the bucketer to organize them.
func writeContextFiles(dir string, messages []db.Message) error {
	if len(messages) == 0 {
		return nil
	}

	// 1. Extract and deduplicate context files using the context parser
	contextFiles := context.ExtractContextFiles(messages)
	
	if len(contextFiles) == 0 {
		logger.Debug("No context files to write")
		return nil
	}

	// 2. Load existing messages.map if it exists
	mapPath := filepath.Join(dir, "messages.map")
	var existingMap *MapFile

	if _, err := os.Stat(mapPath); err == nil {
		// Map file exists, load it
		existingMap, err = loadMessagesMap(mapPath)
		if err != nil {
			logger.Warning("Failed to load existing messages.map, using greedy bucketing", "error", err)
			existingMap = nil
		}
	}

	// 3. Build buckets using the bucketer (Greedy or Leaware)
	buckets := BuildBuckets(contextFiles, existingMap)

	// 4. Create a lookup map for quick access to file content
	fileLookup := make(map[int64]context.ContextFile)
	for _, file := range contextFiles {
		fileLookup[file.ChatID] = file
	}

	// 5. Write each bucket to a file
	var writtenFiles []string
	for _, bucket := range buckets {
		filename := fmt.Sprintf("context-range-%d-%d.md", bucket.MinID, bucket.MaxID)
		path := filepath.Join(dir, filename)
		writtenFiles = append(writtenFiles, filename)

		// Build bucket content
		var content strings.Builder
		
		// Add bucket header
		var bucketFiles []context.ContextFile
		for _, fileEntry := range bucket.Files {
			if fullFile, ok := fileLookup[fileEntry.ChatID]; ok {
				bucketFiles = append(bucketFiles, fullFile)
			}
		}
		
		content.WriteString(context.GenerateBucketHeader(bucketFiles))
		
		// Add files
		for i, fileEntry := range bucket.Files {
			if fullFile, ok := fileLookup[fileEntry.ChatID]; ok {
				content.WriteString(context.FormatFileForBucket(fullFile))
				
				// Add separator between files (but not after last file)
				if i < len(bucket.Files)-1 {
					content.WriteString("\n---End of Item---\n")
				}
			}
		}

		// Check hash to avoid unnecessary writes
		currentHash := calculateHash(content.String())
		if existingHash, err := getFileHash(path); err == nil && existingHash == currentHash {
			logger.Debug("Skipping context bucket (hash match)", "file", filename)
			continue
		}

		if err := os.WriteFile(path, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("failed to write context bucket %s: %w", filename, err)
		}
		logger.Debug("Wrote context bucket", "file", filename, "files", len(bucket.Files), "size", bucket.TotalSize)
	}

	// 6. Zombie cleanup: Delete orphaned context files
	if err := cleanupOrphanedContextFiles(dir, writtenFiles); err != nil {
		logger.Warning("Failed to cleanup orphaned context files", "error", err)
	}

	return nil
}

// cleanupOrphanedContextFiles deletes context-range-*.md files that are not in the writtenFiles list.
func cleanupOrphanedContextFiles(dir string, writtenFiles []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read messages directory: %w", err)
	}

	// Create a set of written files for quick lookup
	writtenSet := make(map[string]bool)
	for _, file := range writtenFiles {
		writtenSet[file] = true
	}

	// Find and delete orphaned files
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasPrefix(name, "context-range-") && strings.HasSuffix(name, ".md") {
			if !writtenSet[name] {
				// This file is orphaned, delete it
				path := filepath.Join(dir, name)
				if err := os.Remove(path); err != nil {
					logger.Warning("Failed to delete orphaned context file", "file", name, "error", err)
				} else {
					logger.Debug("Deleted orphaned context file", "file", name)
				}
			}
		}
	}

	return nil
}

// writeCliOutputFiles writes CLI output messages to isolated files.
// Each CLI output gets its own file named cli-output-{db_id}.md.
func writeCliOutputFiles(dir string, messages []db.Message) error {
	for _, msg := range messages {
		filename := fmt.Sprintf("cli-output-%d.md", msg.ID)
		path := filepath.Join(dir, filename)

		content := ""
		if msg.Message.Valid {
			content = msg.Message.String
		}

		// Check hash to avoid unnecessary writes
		currentHash := calculateHash(content)
		if existingHash, err := getFileHash(path); err == nil && existingHash == currentHash {
			logger.Debug("Skipping CLI output file (hash match)", "file", filename)
			continue
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write CLI output file %s: %w", filename, err)
		}
		logger.Debug("Wrote CLI output file", "file", filename)
	}
	return nil
}

// writeMessagesMap generates or updates the messages.map file.
// Creates a stable-to-volatile read sequence and includes file metadata.
func writeMessagesMap(dir string, contextFiles []context.ContextFile, cliOutputMessages []db.Message, archiveFiles []ArchiveFile) error {
	mapPath := filepath.Join(dir, "messages.map")

	// Build context file metadata from actual files on disk
	var contextFileMetas []FileMeta
	contextDir, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read messages directory: %w", err)
	}

	for _, entry := range contextDir {
		if strings.HasPrefix(entry.Name(), "context-range-") && strings.HasSuffix(entry.Name(), ".md") {
			// Parse filename to get min/max IDs
			var minID, maxID int64
			_, err := fmt.Sscanf(entry.Name(), "context-range-%d-%d.md", &minID, &maxID)
			if err != nil {
				logger.Warning("Failed to parse context filename", "file", entry.Name(), "error", err)
				continue
			}

			// Get file size
			info, err := entry.Info()
			if err != nil {
				logger.Warning("Failed to get file info", "file", entry.Name(), "error", err)
				continue
			}

			// Determine stability based on file count (fewer files = more stable)
			stability := "medium"
			fileCount := int(maxID - minID + 1)
			if fileCount < 10 {
				stability = "high"
			} else if fileCount > 50 {
				stability = "low"
			}

			// Extract file entries from contextFiles
			var fileEntries []FileEntry
			for _, file := range contextFiles {
				if file.ChatID >= minID && file.ChatID <= maxID {
					fileEntries = append(fileEntries, FileEntry{
						ChatID: file.ChatID,
						Name:   file.Name,
						Size:   file.Size,
					})
				}
			}

			contextFileMetas = append(contextFileMetas, FileMeta{
				ID:        fmt.Sprintf("context-range-%d-%d", minID, maxID),
				File:      entry.Name(),
				MinID:     minID,
				MaxID:     maxID,
				Size:      int(info.Size()),
				Stability: stability,
				FileCount: fileCount,
				Files:     fileEntries,
			})
		}
	}

	// Sort context files by MinID
	sort.Slice(contextFileMetas, func(i, j int) bool {
		return contextFileMetas[i].MinID < contextFileMetas[j].MinID
	})

	// Build CLI output file metadata
	var cliOutputFiles []FileMeta
	for _, msg := range cliOutputMessages {
		filename := fmt.Sprintf("cli-output-%d.md", msg.ID)
		path := filepath.Join(dir, filename)

		// Get file size
		info, err := os.Stat(path)
		if err != nil {
			logger.Warning("Failed to get CLI output file info", "file", filename, "error", err)
			continue
		}

		// Determine lifecycle based on message metadata
		// In a real implementation, you'd parse the message meta field
		// For now, default to "volatile"
		lifecycle := "volatile"

		cliOutputFiles = append(cliOutputFiles, FileMeta{
			ID:        fmt.Sprintf("cli-output-%d", msg.ID),
			File:      filename,
			DBID:      msg.ID,
			Size:      int(info.Size()),
			Stability: "low",
			Lifecycle: lifecycle,
		})
	}

	// Sort CLI output files by DBID
	sort.Slice(cliOutputFiles, func(i, j int) bool {
		return cliOutputFiles[i].DBID < cliOutputFiles[j].DBID
	})

	// Build archive file list
	var archives []string
	for _, archive := range archiveFiles {
		archives = append(archives, archive.Name)
	}

	// Build read sequence (stable-to-volatile order)
	var readSequence []string

	// 1. Context files (most stable)
	for _, cf := range contextFileMetas {
		readSequence = append(readSequence, cf.File)
	}

	// 2. CLI output files (moderately volatile)
	for _, cof := range cliOutputFiles {
		readSequence = append(readSequence, cof.File)
	}

	// 3. Archive files (less volatile)
	for _, archive := range archives {
		readSequence = append(readSequence, archive)
	}

	// 4. Active window (most volatile)
	readSequence = append(readSequence, "messages-active.json")

	// Create map file structure
	mapFile := MapFile{
		Version:        "1.0",
		ReadSequence:   readSequence,
		ContextFiles:   contextFileMetas,
		CliOutputFiles: cliOutputFiles,
		Messages: MessagesMeta{
			Active:   "messages-active.json",
			Archives: archives,
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(mapFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal messages.map: %w", err)
	}

	// Write file
	if err := os.WriteFile(mapPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write messages.map: %w", err)
	}

	logger.Debug("Wrote messages.map", "context_files", len(contextFileMetas), "cli_output_files", len(cliOutputFiles))
	return nil
}

// loadMessagesMap loads an existing messages.map file.
func loadMessagesMap(path string) (*MapFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages.map: %w", err)
	}

	var mapFile MapFile
	if err := json.Unmarshal(data, &mapFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages.map: %w", err)
	}

	return &mapFile, nil
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
			if m.Role == "system" {
				continue
			}
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
			logger.Debug("Skipping archive chunk (hash match)", "file", filename)
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
		logger.Info("Wrote archive chunk", "file", filename, "messages", len(chunk))

		archiveFiles = append(archiveFiles, ArchiveFile{
			Name:     filename,
			Hash:     currentHash,
			Messages: len(chunk),
		})
	}

	// Merge Logic: Consolidate oldest chunks if we exceed MaxFiles
	if len(archiveFiles) > settings.MaxFiles {
		// Calculate how many files to merge to get under the limit
		// We merge the oldest files (start of the slice)
		excess := len(archiveFiles) - settings.MaxFiles
		filesToMergeCount := excess + 1 // Merging N files reduces count by N-1

		var mergedMessages []MessageFile
		var filesToDelete []string

		for i := 0; i < filesToMergeCount; i++ {
			file := archiveFiles[i]
			path := filepath.Join(dir, file.Name)

			// Read content
			data, err := os.ReadFile(path)
			if err != nil {
				logger.Warning("Failed to read archive file for merging", "file", file.Name, "error", err)
				continue
			}

			var chunk []MessageFile
			if err := json.Unmarshal(data, &chunk); err != nil {
				logger.Warning("Failed to unmarshal archive file for merging", "file", file.Name, "error", err)
				continue
			}

			mergedMessages = append(mergedMessages, chunk...)
			filesToDelete = append(filesToDelete, path)
		}

		if len(mergedMessages) > 0 {
			// Create new merged filename (e.g., messages-archive-1-2.json)
			mergedFilename := fmt.Sprintf("messages-archive-1-%d.json", filesToMergeCount)
			mergedPath := filepath.Join(dir, mergedFilename)

			// Write merged file
			data, err := json.MarshalIndent(mergedMessages, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal merged archive: %w", err)
			}

			if err := os.WriteFile(mergedPath, data, 0644); err != nil {
				return nil, fmt.Errorf("failed to write merged archive: %w", err)
			}

			// Calculate hash
			mergedHash := calculateHash(string(data))

			// Delete old files
			for _, delPath := range filesToDelete {
				if err := os.Remove(delPath); err != nil {
					logger.Warning("Failed to delete old archive file", "path", delPath, "error", err)
				}
			}

			// Reconstruct archiveFiles list: [Merged] + [Remaining]
			newArchiveFiles := []ArchiveFile{
				{
					Name:     mergedFilename,
					Hash:     mergedHash,
					Messages: len(mergedMessages),
				},
			}
			newArchiveFiles = append(newArchiveFiles, archiveFiles[filesToMergeCount:]...)
			archiveFiles = newArchiveFiles

			logger.Info("Merged archive chunks", "merged_files", filesToMergeCount, "new_file", mergedFilename)
		}
	}

	return archiveFiles, nil
}

// writeActiveWindow writes the recent messages and the archive map to messages-active.json.
func writeActiveWindow(dir string, messages []db.Message, archiveFiles []ArchiveFile) error {
	path := filepath.Join(dir, "messages-active.json")

	// Convert messages
	var msgFiles []MessageFile
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		content := ""
		if m.Message.Valid {
			content = m.Message.String
		}
		msgFiles = append(msgFiles, MessageFile{
			Role:    m.Role,
			Content: content,
		})
	}

	// Skip writing if there is no content
	if len(msgFiles) == 0 && len(archiveFiles) == 0 {
		logger.Debug("Skipping active window write (no content)")
		return nil
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

	logger.Debug("Wrote active window", "messages", len(messages))
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
