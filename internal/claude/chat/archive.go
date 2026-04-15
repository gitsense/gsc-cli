/**
 * Component: Claude Code Chat Archive Manager
 * Block-UUID: ed198402-6fa9-4f00-83b0-4c4369cc0dd6
 * Parent-UUID: f78136a5-8eb7-464a-8006-7ac8c0e217df
 * Version: 1.15.0
 * Description: Updated type references to use claude. prefix for shared types (Settings, ArchiveFile, MapFile, Repository) after moving chat code to separate package.
 * Language: Go
 * Created-at: 2026-04-14T21:10:05.141Z
 * Authors: GLM-4.7 (v1.8.0), claude-haiku-4-5-20251001 (v1.8.1), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), claude-haiku-4-5-20251001 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.12.1), GLM-4.7 (v1.13.1), GLM-4.7 (v1.13.2), Gemini 2.5 Flash (v1.14.0), claude-sonnet-4-6 (v1.15.0)
 */


package chat

import (
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gitsense/gsc-cli/internal/claude"
	"github.com/gitsense/gsc-cli/internal/context"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// SyncArchive reconstructs the file-based state for a chat session.
// It filters messages, writes context files using bucket-based organization,
// isolates CLI outputs, chunks the dialogue history, and updates the messages.map.
func SyncArchive(chatDir string, messages []db.Message, settings claude.Settings) ([]claude.ArchiveFile, error) {
	messagesDir := filepath.Join(chatDir, "messages")
	if err := os.MkdirAll(messagesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create messages directory: %w", err)
	}

	// Create contexts directory
	contextsDir := filepath.Join(chatDir, "contexts")
	if err := os.MkdirAll(contextsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create contexts directory: %w", err)
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
	if err := writeContextFiles(contextsDir, contextMessages); err != nil {
		return nil, fmt.Errorf("failed to write context files: %w", err)
	}

	// 3. Write CLI Output Files (always isolated)
	if err := writeCliOutputFiles(messagesDir, cliOutputMessages); err != nil {
		return nil, fmt.Errorf("failed to write CLI output files: %w", err)
	}

	// Cleanup orphaned CLI output files
	if err := cleanupOrphanedCliOutputFiles(messagesDir, cliOutputMessages); err != nil {
		return nil, fmt.Errorf("failed to cleanup orphaned CLI output files: %w", err)
	}

	// 4. Chunk Dialogue Messages
	// We process messages in reverse (newest first) to easily grab the active window,
	// then reverse the rest for archiving.
	
	// Sort messages by ID to ensure chronological order
	sort.Slice(dialogueMessages, func(i, j int) bool {
		return dialogueMessages[i].ID < dialogueMessages[j].ID
	})

	// Separate Active Window by token budget (falls back to message count)
	var activeMessages []db.Message
	var archiveMessages []db.Message

	if settings.ActiveWindowTokens > 0 {
		used := 0
		splitIndex := len(dialogueMessages)
		for i := len(dialogueMessages) - 1; i >= 0; i-- {
			est := 0
			if dialogueMessages[i].Message.Valid {
				est = len(dialogueMessages[i].Message.String) / 4
			}
			if used+est > settings.ActiveWindowTokens {
				break
			}
			used += est
			splitIndex = i
		}
		archiveMessages = dialogueMessages[:splitIndex]
		activeMessages = dialogueMessages[splitIndex:]
	} else {
		activeCount := settings.ChunkSize
		if len(dialogueMessages) > activeCount {
			splitIndex := len(dialogueMessages) - activeCount
			archiveMessages = dialogueMessages[:splitIndex]
			activeMessages = dialogueMessages[splitIndex:]
		} else {
			activeMessages = dialogueMessages
		}
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
	if err := writeMessagesMap(messagesDir, cliOutputMessages, archiveFiles); err != nil {
		return nil, fmt.Errorf("failed to write messages.map: %w", err)
	}

	// 8. Generate or Update contexts.map
	if err := writeContextsMap(contextsDir, contextFiles); err != nil {
		return nil, fmt.Errorf("failed to write contexts.map: %w", err)
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

	// 2. Load existing contexts.map if it exists
	mapPath := filepath.Join(dir, settings.ClaudeContextsMapFileName)
	var existingMap *claude.MapFile

	if _, err := os.Stat(mapPath); err == nil {
		// Map file exists, load it
		existingMap, err = loadMessagesMap(mapPath)
		if err != nil {
			logger.Warning("Failed to load existing contexts.map, using greedy bucketing", "error", err)
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
			writtenFiles = append(writtenFiles, filename)
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

// cleanupOrphanedCliOutputFiles deletes cli-output-*.md files that are not in the current messages list.
func cleanupOrphanedCliOutputFiles(dir string, currentMessages []db.Message) error {
	// Create a set of valid IDs for quick lookup
	validIDs := make(map[int64]bool)
	for _, msg := range currentMessages {
		validIDs[msg.ID] = true
	}

	// Read directory and delete orphaned files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read messages directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		var msgID int64
		_, err := fmt.Sscanf(name, "cli-output-%d.md", &msgID)
		if err != nil {
			continue // Not a CLI output file
		}

		// If ID not in valid set, delete the file
		if !validIDs[msgID] {
			path := filepath.Join(dir, name)
			if err := os.Remove(path); err != nil {
				logger.Warning("Failed to delete orphaned CLI output file", "file", name, "error", err)
			} else {
				logger.Debug("Deleted orphaned CLI output file", "file", name)
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
func writeMessagesMap(dir string, cliOutputMessages []db.Message, archiveFiles []claude.ArchiveFile) error {
	mapPath := filepath.Join(dir, "messages.map")

	// Build CLI output file metadata
	var cliOutputFiles []claude.FileMeta
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

		cliOutputFiles = append(cliOutputFiles, claude.FileMeta{
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

	// 1. User message (always exists - written before SyncArchive)
	readSequence = append(readSequence, "user-message.md")

	// 2. CLI output files (moderately volatile)
	for _, cof := range cliOutputFiles {
		readSequence = append(readSequence, cof.File)
	}

	// 3. Archive files (less volatile)
	for _, archive := range archives {
		readSequence = append(readSequence, archive)
	}

	// 4. Active window (most volatile) - ONLY if it exists
	activeWindowPath := filepath.Join(dir, "messages-active.json")
	if _, err := os.Stat(activeWindowPath); err == nil {
		readSequence = append(readSequence, "messages-active.json")
	}

	// Create map file structure
	mapFile := claude.MapFile{
		Version:        "1.0",
		ReadSequence:   readSequence,
		ContextFiles:   []claude.FileMeta{}, // Empty - context files moved to contexts.map
		CliOutputFiles: cliOutputFiles,
		Messages: claude.MessagesMeta{
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

	logger.Debug("Wrote messages.map", "cli_output_files", len(cliOutputFiles))
	return nil
}

// loadMessagesMap loads an existing messages.map file.
func loadMessagesMap(path string) (*claude.MapFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read messages.map: %w", err)
	}

	var mapFile claude.MapFile
	if err := json.Unmarshal(data, &mapFile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal messages.map: %w", err)
	}

	return &mapFile, nil
}

// writeContextsMap generates the contexts.map file with all context file metadata.
func writeContextsMap(dir string, contextFiles []context.ContextFile) error {
	mapPath := filepath.Join(dir, settings.ClaudeContextsMapFileName)

	// Build context file metadata from actual files on disk
	var contextFileMetas []claude.FileMeta
	contextDir, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read contexts directory: %w", err)
	}

	// Build unique repositories map
	repositories := make(map[string]*claude.Repository)
	repoCounter := 0
	repoIDMap := make(map[string]*claude.Repository) // Map repo ID to Repository for O(1) lookup

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
			var fileEntries []claude.FileEntry
			for _, file := range contextFiles {
				if file.ChatID >= minID && file.ChatID <= maxID {
					// Build repository information
					var repo *claude.Repository
					var repoID string
					if file.Repo != "" {
						if existingRepo, ok := repositories[file.Repo]; ok {
							repo = existingRepo
							repoID = existingRepo.ID
						} else {
							repoCounter++
							repoID = fmt.Sprintf("repo-%d", repoCounter)
							repo = &claude.Repository{
								ID:   repoID,
								Name: file.Repo,
								URL:  "", // Could be enhanced to include URL if available
							}
							repositories[file.Repo] = repo
							repoIDMap[repoID] = repo
						}
					}

					fileEntries = append(fileEntries, claude.FileEntry{
						ChatID: file.ChatID,
						Path:   file.Path, // Use full relative path for better context
						RepoID: repoID,
					})
				}
			}

			// Add repository to FileMeta if all files in this bucket are from the same repo
			var bucketRepo *claude.Repository
			if len(fileEntries) > 0 && fileEntries[0].RepoID != "" {
				allSameRepo := true
				for _, fe := range fileEntries {
					if fe.RepoID != fileEntries[0].RepoID {
						allSameRepo = false
						break
					}
				}
				if allSameRepo {
					bucketRepo = repoIDMap[fileEntries[0].RepoID]
				}
			}

			contextFileMetas = append(contextFileMetas, claude.FileMeta{
				ID:        fmt.Sprintf("context-range-%d-%d", minID, maxID),
				File:      entry.Name(),
				Type:      "source_code_archive",
				MinID:     minID,
				MaxID:     maxID,
				Size:      int(info.Size()),
				Stability: stability,
				FileCount: fileCount,
				Repository: bucketRepo, // Keep full repository object at bucket level for convenience
				Files:     fileEntries,
			})
		}
	}

	// Sort context files by MinID
	sort.Slice(contextFileMetas, func(i, j int) bool {
		return contextFileMetas[i].MinID < contextFileMetas[j].MinID
	})

	// Create contexts map file structure
	contextsMapFile := claude.MapFile{
		Version:        "1.0",
		Repositories:   buildRepositoryList(repositories),
		ReadSequence:   []string{}, // Empty - AI reads selectively
		ContextFiles:   contextFileMetas,
		CliOutputFiles: []claude.FileMeta{},
		Messages: claude.MessagesMeta{
			Active:   "",
			Archives: []string{},
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(contextsMapFile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contexts.map: %w", err)
	}

	// Write file
	if err := os.WriteFile(mapPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write contexts.map: %w", err)
	}

	logger.Debug("Wrote contexts.map", "context_files", len(contextFileMetas))
	return nil
}

// buildRepositoryList converts the repositories map to a sorted slice
func buildRepositoryList(repos map[string]*claude.Repository) []claude.Repository {
	var result []claude.Repository
	for _, repo := range repos {
		result = append(result, *repo)
	}
	// Sort by ID for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// writeArchiveChunks chunks the historical messages and writes them to JSON files.
func writeArchiveChunks(dir string, messages []db.Message, settings claude.Settings) ([]claude.ArchiveFile, error) {
	var archiveFiles []claude.ArchiveFile

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
		var msgFiles []claude.MessageFile
		for _, m := range chunk {
			if m.Role == "system" {
				continue
			}
			content := ""
			if m.Message.Valid {
				content = m.Message.String
			}
			msgFiles = append(msgFiles, claude.MessageFile{
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
			archiveFiles = append(archiveFiles, claude.ArchiveFile{
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

		archiveFiles = append(archiveFiles, claude.ArchiveFile{
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

		var mergedMessages []claude.MessageFile
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

			var chunk []claude.MessageFile
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
			newArchiveFiles := []claude.ArchiveFile{
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
func writeActiveWindow(dir string, messages []db.Message, archiveFiles []claude.ArchiveFile) error {
	path := filepath.Join(dir, "messages-active.json")

	// Convert messages
	var msgFiles []claude.MessageFile
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		content := ""
		if m.Message.Valid {
			content = m.Message.String
		}
		msgFiles = append(msgFiles, claude.MessageFile{
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
	window := claude.ActiveWindow{
		ArchiveMap: claude.ArchiveMap{Files: archiveFiles},
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
