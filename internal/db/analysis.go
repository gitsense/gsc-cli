/**
 * Component: Analysis Copy Database Operations
 * Block-UUID: dbe01592-2364-417f-b1b3-fd02c34b54ec
 * Parent-UUID: f154a420-bb0b-4a0a-a6b9-819db65db187
 * Version: 1.12.0
 * Description: Handles the database logic for copying, dumping, loading, getting, and setting analyzer metadata. v1.10.0: Added GetAnalysisForFile and ListAnalysisForAnalyzer for the get command. Added GetLatestLeafMessageID (true leaf discovery using NOT EXISTS), ArchiveMessageToHistory, UpdateAnalysisMessage, and SetAnalysisBulk (bulk insert/update orchestrator with chat token updates) for the set command. v1.11.0: Removed unused normalizeAnalyzerForQuery; inlined pattern logic into get/list functions for clarity. v1.12.0: Added hash comparison to SetAnalysisBulk so unchanged analysis skips without --force while changed or legacy records update.
 * Language: Go
 * Created-at: 2026-05-14T18:40:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), MiMo-v2.5-Pro (v1.10.0), MiMo-v2.5-Pro (v1.11.0), Codex (v1.12.0)
 */


package db

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
)

// CountAnalysisCopy performs a pre-flight check to determine how many files will be affected by the copy operation.
// It counts matching files in the source branch that do not already have the specified analyzer type in the target branch.
func CountAnalysisCopy(db *sql.DB, sourceRefID, targetRefID int64, analyzerType string) (int64, error) {
	query := `
	SELECT COUNT(*)
	FROM chats AS source_blob
	JOIN chats AS target_blob
		ON json_extract(source_blob.meta, '$.path') = json_extract(target_blob.meta, '$.path')
	JOIN messages AS source_msg
		ON source_msg.chat_id = source_blob.id
		AND source_msg.type = ?
		AND source_msg.deleted = 0
	WHERE source_blob.type = 'git-blob'
	  AND source_blob.deleted = 0
	  AND json_extract(source_blob.meta, '$.refContext.refChatId') = ?
	  AND target_blob.type = 'git-blob'
	  AND target_blob.deleted = 0
	  AND json_extract(target_blob.meta, '$.refContext.refChatId') = ?
	  AND NOT EXISTS (
		  SELECT 1 FROM messages m
		  WHERE m.chat_id = target_blob.id
			AND m.type = ?
			AND m.deleted = 0
	  )`

	var count int64
	err := db.QueryRow(query, analyzerType, sourceRefID, targetRefID, analyzerType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count analysis copy candidates: %w", err)
	}

	return count, nil
}

// CopyAnalysis performs the actual copy operation within a transaction.
// It copies analyzer messages from the source branch to the target branch and updates token metadata.
// If dryRun is true, the transaction is rolled back after execution.
func CopyAnalysis(db *sql.DB, sourceRefID, targetRefID int64, analyzerType string, dryRun bool) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Insert Analyzer Messages
	// This query copies the message content from source to target, appending it to the end of the target's message chain.
	// It skips files where the target already has a message of this type.
	insertQuery := `
	INSERT INTO messages (
		type, deleted, visibility, chat_id, parent_id, level,
		model, real_model, temperature, role, message, meta, hash,
		created_at, updated_at
	)
	SELECT
		source_msg.type,
		source_msg.deleted,
		source_msg.visibility,
		target_blob.id,
		(SELECT MAX(id) FROM messages WHERE chat_id = target_blob.id AND deleted = 0),
		source_msg.level,
		(SELECT main_model FROM chats WHERE id = target_blob.id),
		source_msg.real_model,
		source_msg.temperature,
		source_msg.role,
		source_msg.message,
		source_msg.meta,
		source_msg.hash,
		datetime('now'),
		datetime('now')
	FROM chats AS source_blob
	JOIN chats AS target_blob
		ON json_extract(source_blob.meta, '$.path') = json_extract(target_blob.meta, '$.path')
	JOIN messages AS source_msg
		ON source_msg.chat_id = source_blob.id
		AND source_msg.type = ?
		AND source_msg.deleted = 0
	WHERE source_blob.type = 'git-blob'
	  AND source_blob.deleted = 0
	  AND json_extract(source_blob.meta, '$.refContext.refChatId') = ?
	  AND target_blob.type = 'git-blob'
	  AND target_blob.deleted = 0
	  AND json_extract(target_blob.meta, '$.refContext.refChatId') = ?
	  AND NOT EXISTS (
		  SELECT 1 FROM messages m
		  WHERE m.chat_id = target_blob.id
			AND m.type = ?
			AND m.deleted = 0
	  )`

	insertResult, err := tx.Exec(insertQuery, analyzerType, sourceRefID, targetRefID, analyzerType)
	if err != nil {
		return 0, fmt.Errorf("failed to insert analyzer messages: %w", err)
	}

	rowsAffected, err := insertResult.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	// Step 2: Update Chat Metadata (Tokens)
	// This query merges the 'tokens.analysis' object from the source chat into the target chat.
	// It only updates chats where the analysis tokens are currently null.
	updateQuery := `
	UPDATE chats
	SET meta = json_set(
		meta,
		'$.tokens.analysis',
		(
			SELECT json_extract(s.meta, '$.tokens.analysis')
			FROM chats s
			WHERE json_extract(s.meta, '$.path') = json_extract(chats.meta, '$.path')
			  AND json_extract(s.meta, '$.refContext.refChatId') = ?
			  AND s.deleted = 0
			LIMIT 1
		)
	)
	WHERE type = 'git-blob'
	  AND deleted = 0
	  AND json_extract(meta, '$.refContext.refChatId') = ?
	  AND json_extract(meta, '$.tokens.analysis') IS NULL`

	_, err = tx.Exec(updateQuery, sourceRefID, targetRefID)
	if err != nil {
		return 0, fmt.Errorf("failed to update chat metadata: %w", err)
	}

	// Commit or Rollback
	if dryRun {
		logger.Debug("Dry run: rolling back transaction")
		if err := tx.Rollback(); err != nil {
			return 0, fmt.Errorf("failed to rollback transaction: %w", err)
		}
	} else {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return rowsAffected, nil
}

// DumpRecord represents a single line in the JSONL dump file.
type DumpRecord struct {
	DumpID       string                 `json:"dump_id"`
	DumpedAt     string                 `json:"dumped_at"`
	Owner        string                 `json:"owner"` // v1.7.0: Added for portable dump files
	Repo         string                 `json:"repo"`  // v1.7.0: Now stores only repo name (not owner/repo)
	Branch       string                 `json:"branch"`
	Analyzer     string                 `json:"analyzer"`     // The analyzer flag used (e.g., "code-intent")
	MessageType  string                 `json:"message_type"` // The actual message type from DB (e.g., "code-intent::file-content::default")
	Path         string                 `json:"path"`
	CreatedAt    string                 `json:"created_at"`
	Tokens       map[string]interface{} `json:"tokens"`
	Message      map[string]interface{} `json:"message"`
	AnalysisHash string                 `json:"analysis_hash"` // Hash of path + content + metadata
}

// ComputeAnalysisHash generates a deterministic SHA256 hash for an analysis record.
// The hash is computed from: path + "|" + messageContent + "|" + messageMeta (JSON)
// The metadata JSON is marshaled with sorted keys to ensure determinism.
func ComputeAnalysisHash(path, messageContent string, messageMeta map[string]interface{}) (string, error) {
	// Marshal metadata to JSON (Go sorts map keys alphabetically, ensuring determinism)
	metaJSON, err := json.Marshal(messageMeta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message metadata for hashing: %w", err)
	}

	// Concatenate with pipe separator
	combined := fmt.Sprintf("%s|%s|%s", path, messageContent, string(metaJSON))

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:]), nil
}

// BuildExistingHashSet reads an existing JSONL dump file and builds a set of analysis hashes.
// This is used for incremental dumping to avoid writing duplicate records.
// Returns an empty map if the file does not exist (not an error).
func BuildExistingHashSet(filePath string) (map[string]bool, error) {
	hashSet := make(map[string]bool)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// File doesn't exist, return empty set
		return hashSet, nil
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing dump file: %w", err)
	}
	defer file.Close()

	// Use scanner with large buffer to handle large records
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Parse only the analysis_hash field
		var record struct {
			AnalysisHash string `json:"analysis_hash"`
		}
		if err := json.Unmarshal(line, &record); err != nil {
			// Log warning but continue - old dump files may not have this field
			logger.Warning("Failed to parse line in existing dump file", "line", lineNum, "error", err)
			continue
		}

		// Add non-empty hashes to the set
		if record.AnalysisHash != "" {
			hashSet[record.AnalysisHash] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading existing dump file: %w", err)
	}

	return hashSet, nil
}

// CountAnalysisDump performs a pre-flight check to determine how many analysis records exist for the dump.
// Supports flexible analyzer matching (e.g., "code-intent" matches "code-intent::file-content::default").
func CountAnalysisDump(db *sql.DB, refChatID int64, analyzerType string) (int64, error) {
	var query string
	var arg interface{}

	// Determine matching strategy
	if strings.Contains(analyzerType, "::") {
		// Exact match
		query = `
		SELECT COUNT(*)
		FROM chats AS c
		JOIN messages AS m
			ON m.chat_id = c.id
			AND m.type = ?
			AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?`
		arg = analyzerType
	} else {
		// Prefix match
		query = `
		SELECT COUNT(*)
		FROM chats AS c
		JOIN messages AS m
			ON m.chat_id = c.id
			AND m.type LIKE ?
			AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?`
		arg = analyzerType + "::%"
	}

	var count int64
	err := db.QueryRow(query, arg, refChatID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count analysis dump candidates: %w", err)
	}

	return count, nil
}

// DumpAnalysis exports analysis results to a JSONL format.
// It writes the output to the provided io.Writer.
// Supports flexible analyzer matching (e.g., "code-intent" matches "code-intent::file-content::default").
// Returns (written, skipped, error) where written is the number of new records written,
// and skipped is the number of records that already exist in the dump file.
// v1.7.0: Updated signature to accept owner and repo separately for portable dump files.
func DumpAnalysis(db *sql.DB, refChatID int64, analyzerType, dumpID, owner, repo, branch string, writer io.Writer, existingHashes map[string]bool) (int64, int64, error) {
	var query string
	var arg interface{}

	// Determine matching strategy
	if strings.Contains(analyzerType, "::") {
		// Exact match
		query = `
		SELECT 
			json_extract(c.meta, '$.path') as path,
			json_extract(c.meta, '$.tokens.analysis') as tokens,
			m.message as message_content,
			m.meta as message_meta,
			m.created_at as created_at,
			m.type as message_type
		FROM chats AS c
		JOIN messages AS m
			ON m.chat_id = c.id
			AND m.type = ?
			AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?`
		arg = analyzerType
	} else {
		// Prefix match
		query = `
		SELECT 
			json_extract(c.meta, '$.path') as path,
			json_extract(c.meta, '$.tokens.analysis') as tokens,
			m.message as message_content,
			m.meta as message_meta,
			m.created_at as created_at,
			m.type as message_type
		FROM chats AS c
		JOIN messages AS m
			ON m.chat_id = c.id
			AND m.type LIKE ?
			AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?`
		arg = analyzerType + "::%"
	}

	rows, err := db.Query(query, arg, refChatID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to query analysis for dump: %w", err)
	}
	defer rows.Close()

	dumpedAt := time.Now().UTC().Format(time.RFC3339Nano)
	written := int64(0)
	skipped := int64(0)

	for rows.Next() {
		var path string
		var tokensJSON sql.NullString
		var messageContent string
		var messageMetaJSON sql.NullString
		var createdAt string
		var messageType string

		if err := rows.Scan(&path, &tokensJSON, &messageContent, &messageMetaJSON, &createdAt, &messageType); err != nil {
			return 0, 0, fmt.Errorf("failed to scan dump row: %w", err)
		}

		// Parse tokens JSON
		var tokens map[string]interface{}
		if tokensJSON.Valid {
			if err := json.Unmarshal([]byte(tokensJSON.String), &tokens); err != nil {
				logger.Warning("Failed to parse tokens JSON for path", "path", path, "error", err)
				tokens = make(map[string]interface{})
			}
		} else {
			tokens = make(map[string]interface{})
		}

		// Parse message meta JSON
		var messageMeta map[string]interface{}
		if messageMetaJSON.Valid {
			if err := json.Unmarshal([]byte(messageMetaJSON.String), &messageMeta); err != nil {
				logger.Warning("Failed to parse message meta JSON for path", "path", path, "error", err)
				messageMeta = make(map[string]interface{})
			}
		} else {
			messageMeta = make(map[string]interface{})
		}

		// Compute analysis hash
		analysisHash, err := ComputeAnalysisHash(path, messageContent, messageMeta)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to compute analysis hash for path %s: %w", path, err)
		}

		// Check if this record already exists in the dump file
		if existingHashes[analysisHash] {
			skipped++
			continue
		}

		// Construct the message object
		messageObj := map[string]interface{}{
			"role":    "assistant", // Assuming analysis results are always assistant role
			"content": messageContent,
			"meta":    messageMeta,
		}

		// Construct the full record
		// v1.7.0: Populate Owner and Repo separately
		record := DumpRecord{
			DumpID:       dumpID,
			DumpedAt:     dumpedAt,
			Owner:        owner,
			Repo:         repo,
			Branch:       branch,
			Analyzer:     analyzerType,
			MessageType:  messageType,
			Path:         path,
			CreatedAt:    createdAt,
			Tokens:       tokens,
			Message:      messageObj,
			AnalysisHash: analysisHash,
		}

		// Marshal to JSON
		data, err := json.Marshal(record)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to marshal dump record for path %s: %w", path, err)
		}

		// Write to writer with newline
		if _, err := writer.Write(data); err != nil {
			return 0, 0, fmt.Errorf("failed to write dump record for path %s: %w", path, err)
		}
		if _, err := writer.Write([]byte("\n")); err != nil {
			return 0, 0, fmt.Errorf("failed to write newline for path %s: %w", path, err)
		}

		written++
	}

	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("error iterating dump rows: %w", err)
	}

	return written, skipped, nil
}

// FileState represents the current DB state of a git-blob file in the target branch.
type FileState struct {
	ChatID      int64
	ParentID    int64 // MAX(id) from messages - used as parent_id for new inserts
	HasAnalysis bool  // true if analyzer message already exists
}

// LoadItem is a resolved candidate ready for insertion.
type LoadItem struct {
	Path     string
	ChatID   int64
	ParentID int64
	Record   DumpRecord // the source JSONL record
}

// BuildTargetSnapshot queries the database to build a map of all files in the target branch.
// This map is used for O(1) lookups during the load process.
// analyzerPrefix is used to check if analysis already exists (e.g., "code-intent%").
func BuildTargetSnapshot(db *sql.DB, refChatID int64, analyzerPrefix string) (map[string]FileState, error) {
	query := `
	SELECT
		c.id,
		json_extract(c.meta, '$.path') as path,
		COALESCE((SELECT MAX(id) FROM messages m WHERE m.chat_id = c.id AND m.deleted = 0), 0) as parent_id,
		EXISTS (
			SELECT 1 FROM messages m2
			WHERE m2.chat_id = c.id
			  AND m2.type LIKE ?
			  AND m2.deleted = 0
		) as has_analysis
	FROM chats c
	WHERE c.type = 'git-blob'
	  AND c.deleted = 0
	  AND json_extract(c.meta, '$.refContext.refChatId') = ?`

	rows, err := db.Query(query, analyzerPrefix, refChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to build target snapshot: %w", err)
	}
	defer rows.Close()

	snapshot := make(map[string]FileState)

	for rows.Next() {
		var state FileState
		var path string
		var hasAnalysis int // SQLite returns 0 or 1 for EXISTS

		if err := rows.Scan(&state.ChatID, &path, &state.ParentID, &hasAnalysis); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot row: %w", err)
		}

		state.HasAnalysis = (hasAnalysis == 1)
		snapshot[path] = state
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshot rows: %w", err)
	}

	return snapshot, nil
}

// LoadAnalysis inserts analysis records into the database.
// It uses prepared statements and batched transactions for performance.
// onProgress is called after each record is processed.
// v1.9.0: Updated to insert analyzer messages in a linear chain (parent-child) instead of as siblings.
// Candidates are sorted by ChatID and Analyzer to ensure deterministic ordering.
func LoadAnalysis(ctx context.Context, db *sql.DB, candidates []LoadItem, batchSize int, onProgress func(n int, path string)) (int64, error) {
	if len(candidates) == 0 {
		return 0, nil
	}

	// Sort candidates to ensure deterministic ordering
	// We sort by ChatID first, then by Analyzer name alphabetically
	// This ensures that for a specific file, analyzers are inserted in a consistent order (e.g., code-intent -> family-code-intent)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ChatID != candidates[j].ChatID {
			return candidates[i].ChatID < candidates[j].ChatID
		}
		return candidates[i].Record.Analyzer < candidates[j].Record.Analyzer
	})

	// Prepare Insert Statement (Pool Level)
	insertQuery := `
	INSERT INTO messages (
		type, deleted, visibility, chat_id, parent_id, level,
		model, real_model, temperature, role, message, meta, hash,
		created_at, updated_at
	) VALUES (?, 0, 'public', ?, ?, 3, 'GitSense Notes', NULL, 0, 'assistant', ?, ?, ?, datetime('now'), datetime('now'))`

	stmtInsert, err := db.Prepare(insertQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmtInsert.Close()

	// Prepare Update Statement (Pool Level)
	updateQuery := `
	UPDATE chats
	SET meta = json_set(meta, '$.tokens.analysis', json(?))
	WHERE id = ?`

	stmtUpdate, err := db.Prepare(updateQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmtUpdate.Close()

	// Begin Transaction
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Bind pool-level statements to the current transaction
	// This is critical for SQLite with MaxOpenConns(1) to avoid deadlock
	txInsert := tx.Stmt(stmtInsert)
	txUpdate := tx.Stmt(stmtUpdate)

	processedCount := 0

	// Track the last inserted message ID for each ChatID to build the linear chain
	lastInsertedID := make(map[int64]int64)

	for _, item := range candidates {
		// Check for cancellation
		if ctx.Err() != nil {
			// Context cancelled, stop processing
			// The loop will exit, and the final commit will happen
			break
		}

		// 1. Update Chat Metadata (Tokens)
		tokensJSON, err := json.Marshal(item.Record.Tokens)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal tokens for path %s: %w", item.Path, err)
		}

		_, err = txUpdate.Exec(tokensJSON, item.ChatID)
		if err != nil {
			return 0, fmt.Errorf("failed to update chat metadata for path %s: %w", item.Path, err)
		}

		// 2. Determine Parent ID for Linear Chain
		// If we've already inserted a message for this ChatID in this batch, use its ID as the parent.
		// Otherwise, use the ParentID from the snapshot (the last message from the previous import).
		parentID := item.ParentID
		if lastID, exists := lastInsertedID[item.ChatID]; exists {
			parentID = lastID
		}

		// 3. Insert Message
		// Extract message content and meta from the DumpRecord
		messageContent, ok := item.Record.Message["content"].(string)
		if !ok {
			return 0, fmt.Errorf("invalid message content type for path %s", item.Path)
		}

		messageMeta, err := json.Marshal(item.Record.Message["meta"])
		if err != nil {
			return 0, fmt.Errorf("failed to marshal message meta for path %s: %w", item.Path, err)
		}

		// Use the actual message type from the dump record
		analyzerType := item.Record.MessageType
		if analyzerType == "" {
			// Fallback if not in record (shouldn't happen with valid dump)
			analyzerType = "unknown"
		}

		// Compute or retrieve hash
		hashValue := item.Record.AnalysisHash
		if hashValue == "" {
			// Backward compatibility: re-compute from record data for old dump files
			messageMetaMap, ok := item.Record.Message["meta"].(map[string]interface{})
			if !ok {
				return 0, fmt.Errorf("invalid message meta type for path %s", item.Path)
			}
			hashValue, err = ComputeAnalysisHash(item.Path, messageContent, messageMetaMap)
			if err != nil {
				return 0, fmt.Errorf("failed to compute fallback hash for path %s: %w", item.Path, err)
			}
		}

		result, err := txInsert.Exec(
			analyzerType,
			item.ChatID,
			parentID, // Use the dynamically determined parentID
			messageContent,
			messageMeta,
			hashValue,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to insert message for path %s: %w", item.Path, err)
		}

		// Get the ID of the newly inserted message to update the chain tracker
		newID, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("failed to get last insert id for path %s: %w", item.Path, err)
		}

		// Update the tracker so the next message for this ChatID becomes a child of this one
		lastInsertedID[item.ChatID] = newID

		processedCount++

		// 4. Progress Callback
		if onProgress != nil {
			onProgress(processedCount, item.Path)
		}

		// 5. Batch Commit
		if processedCount%batchSize == 0 {
			// Close transaction-bound statements before committing
			txInsert.Close()
			txUpdate.Close()

			if err := tx.Commit(); err != nil {
				return 0, fmt.Errorf("failed to commit batch: %w", err)
			}
			// Start new transaction
			tx, err = db.Begin()
			if err != nil {
				return 0, fmt.Errorf("failed to begin new transaction: %w", err)
			}
			// Re-bind statements to new transaction
			txInsert = tx.Stmt(stmtInsert)
			txUpdate = tx.Stmt(stmtUpdate)
		}
	}

	// Close transaction-bound statements before final commit
	txInsert.Close()
	txUpdate.Close()

	// Commit remaining records
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit final batch: %w", err)
	}

	return int64(processedCount), nil
}

// GetUniqueAnalyzers retrieves all distinct analyzer prefixes for a given branch.
// It queries the messages table to find all unique message types that match the analyzer pattern (e.g., "code-intent::file-content::default").
// It then extracts the prefix (e.g., "code-intent") and returns a sorted list of unique prefixes.
// This is used by the rebuild workflow to discover all analyzers that need to be dumped and restored.
func GetUniqueAnalyzers(db *sql.DB, refChatID int64) ([]string, error) {
	query := `
	SELECT DISTINCT m.type
	FROM chats c
	JOIN messages m ON m.chat_id = c.id AND m.deleted = 0
	WHERE c.type = 'git-blob'
	  AND c.deleted = 0
	  AND json_extract(c.meta, '$.refContext.refChatId') = ?
	  AND m.type LIKE '%::%'`

	rows, err := db.Query(query, refChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query unique analyzers: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	var result []string

	for rows.Next() {
		var messageType string
		if err := rows.Scan(&messageType); err != nil {
			return nil, fmt.Errorf("failed to scan analyzer type: %w", err)
		}

		// Extract prefix (e.g., "code-intent" from "code-intent::file-content::default")
		parts := strings.SplitN(messageType, "::", 2)
		if len(parts) > 0 {
			prefix := parts[0]
			if _, exists := seen[prefix]; !exists {
				seen[prefix] = struct{}{}
				result = append(result, prefix)
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating analyzer rows: %w", err)
	}

	// Sort for consistent ordering
	sort.Strings(result)

	return result, nil
}

// =====================================================================
// GET COMMAND SUPPORT
// =====================================================================

// ErrFileNotFound is returned when a file path cannot be resolved to a chat ID in the target branch.
var ErrFileNotFound = errors.New("file not found in database")

// GetAnalysisForFile retrieves the extracted_metadata fields for a single file from a specific analyzer.
// It returns a flat map containing file_path, chat_id, and all (or selected) extracted_metadata fields.
//
// Returns:
//   - (result, nil) if analysis is found
//   - (nil, nil) if the file exists but has no analysis of the requested type
//   - (nil, ErrFileNotFound) if the file does not exist in the target branch
func GetAnalysisForFile(db *sql.DB, refChatID int64, filePath string, analyzer string, selectFields []string) (map[string]interface{}, error) {
	// Build analyzer pattern for flexible matching
	analyzerPattern := analyzer
	if !strings.Contains(analyzer, "::") {
		analyzerPattern = analyzer + "::%"
	}

	query := `
		SELECT c.id, m.meta
		FROM chats c
		JOIN messages m ON m.chat_id = c.id AND m.type LIKE ? AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?
		  AND json_extract(c.meta, '$.path') = ?
		ORDER BY m.id DESC
		LIMIT 1`

	var chatID int64
	var metaJSON sql.NullString

	err := db.QueryRow(query, analyzerPattern, refChatID, filePath).Scan(&chatID, &metaJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			// File might exist but have no analysis - check file existence
			var exists int
			checkQuery := `SELECT 1 FROM chats WHERE type = 'git-blob' AND deleted = 0 AND json_extract(meta, '$.refContext.refChatId') = ? AND json_extract(meta, '$.path') = ? LIMIT 1`
			err2 := db.QueryRow(checkQuery, refChatID, filePath).Scan(&exists)
			if err2 == sql.ErrNoRows {
				return nil, ErrFileNotFound
			}
			if err2 != nil {
				return nil, fmt.Errorf("failed to check file existence: %w", err2)
			}
			// File exists but no analysis of this type
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query analysis for file '%s': %w", filePath, err)
	}

	// Build the flat result map
	result := map[string]interface{}{
		"file_path": filePath,
		"chat_id":   chatID,
	}

	// Parse meta and extract fields into the flat map
	if metaJSON.Valid {
		var meta map[string]interface{}
		if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
			if em, ok := meta["extracted_metadata"].(map[string]interface{}); ok {
				for k, v := range em {
					result[k] = v
				}
			}
		} else {
			logger.Warning("Failed to parse message meta for file", "file", filePath, "error", err)
		}
	}

	// Filter to requested fields if specified (file_path and chat_id are always included)
	if len(selectFields) > 0 {
		filtered := map[string]interface{}{
			"file_path": filePath,
			"chat_id":   chatID,
		}
		for _, field := range selectFields {
			if v, ok := result[field]; ok {
				filtered[field] = v
			} else {
				logger.Debug("Requested field not found in analysis", "field", field, "file", filePath)
			}
		}
		return filtered, nil
	}

	return result, nil
}

// ListAnalysisForAnalyzer retrieves all files with analysis for a specific analyzer in the target branch.
// Returns a slice of maps, each containing file_path, chat_id, and extracted_metadata fields.
func ListAnalysisForAnalyzer(db *sql.DB, refChatID int64, analyzer string, selectFields []string) ([]map[string]interface{}, error) {
	// Build analyzer pattern for flexible matching
	analyzerPattern := analyzer
	if !strings.Contains(analyzer, "::") {
		analyzerPattern = analyzer + "::%"
	}

	query := `
		SELECT json_extract(c.meta, '$.path') as path, c.id as chat_id, m.meta
		FROM chats c
		JOIN messages m ON m.chat_id = c.id AND m.type LIKE ? AND m.deleted = 0
		WHERE c.type = 'git-blob'
		  AND c.deleted = 0
		  AND json_extract(c.meta, '$.refContext.refChatId') = ?
		ORDER BY path ASC`

	rows, err := db.Query(query, analyzerPattern, refChatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query analysis list: %w", err)
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var filePath string
		var chatID int64
		var metaJSON sql.NullString

		if err := rows.Scan(&filePath, &chatID, &metaJSON); err != nil {
			logger.Warning("Failed to scan analysis list row", "error", err)
			continue
		}

		result := map[string]interface{}{
			"file_path": filePath,
			"chat_id":   chatID,
		}

		// Parse meta and extract fields
		if metaJSON.Valid {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(metaJSON.String), &meta); err == nil {
				if em, ok := meta["extracted_metadata"].(map[string]interface{}); ok {
					for k, v := range em {
						result[k] = v
					}
				}
			}
		}

		// Filter to requested fields if specified
		if len(selectFields) > 0 {
			filtered := map[string]interface{}{
				"file_path": filePath,
				"chat_id":   chatID,
			}
			for _, field := range selectFields {
				if v, ok := result[field]; ok {
					filtered[field] = v
				}
			}
			results = append(results, filtered)
		} else {
			results = append(results, result)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating analysis list rows: %w", err)
	}

	return results, nil
}

// =====================================================================
// SET COMMAND SUPPORT
// =====================================================================

// SetItem represents a single analysis record to be inserted or updated by SetAnalysisBulk.
type SetItem struct {
	ChatID      int64                  // Resolved chat ID for the file
	Analyzer    string                 // Message type, e.g., "rust-depmap::file-content::default"
	Content     string                 // Generated markdown message content
	Meta        map[string]interface{} // Full meta envelope including extracted_metadata
	SourceModel string                 // real_model, e.g., "external::syn"
}

// SetResult tracks the outcome of a SetAnalysisBulk operation.
type SetResult struct {
	Inserted int
	Updated  int
	Skipped  int
	Errors   int
}

// GetLatestLeafMessageID finds the true leaf message for a chat - a message that has no children.
// When multiple leaf messages exist, returns the one with the latest created_at (highest id as tiebreaker).
// This follows the JavaScript getLatestChatMessageIds pattern using NOT EXISTS.
//
// This is distinct from GetLastMessageID (in chats.go) which walks the deepest recursive path.
// For analysis message attachment, the true leaf is more correct because it guarantees
// the new message won't create unexpected branching in the conversation tree.
func GetLatestLeafMessageID(db *sql.DB, chatID int64) (int64, error) {
	query := `
		SELECT m.id
		FROM messages m
		WHERE m.chat_id = ?
			AND m.deleted = 0
			AND NOT EXISTS (
				SELECT 1 FROM messages m2
				WHERE m2.parent_id = m.id
					AND m2.chat_id = m.chat_id
					AND m2.deleted = 0
			)
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1`

	var id int64
	err := db.QueryRow(query, chatID).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("no leaf message found for chat %d", chatID)
		}
		return 0, fmt.Errorf("failed to find leaf message for chat %d: %w", chatID, err)
	}
	return id, nil
}

// ArchiveMessageToHistory copies a message to the message_history table before it is updated in place.
// This preserves the original version for auditability, matching the JavaScript updateChatAnalysisMessages pattern.
// The message_history table stores all prior versions of a message.
func ArchiveMessageToHistory(db *sql.DB, messageID int64) error {
	query := `
		INSERT INTO message_history (id, type, deleted, visibility, chat_id, parent_id, level,
			hash, message, chat_completion_stats, meta, created_at, updated_at, modified_at)
		SELECT id, type, deleted, visibility, chat_id, parent_id, level,
			hash, message, chat_completion_stats, meta, created_at, updated_at, modified_at
		FROM messages
		WHERE id = ?`

	result, err := db.Exec(query, messageID)
	if err != nil {
		return fmt.Errorf("failed to archive message %d to history: %w", messageID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message %d not found for archival", messageID)
	}

	logger.Debug("Archived message to history", "message_id", messageID)
	return nil
}

// UpdateAnalysisMessage updates an analysis message in place, preserving its ID and position
// in the conversation tree. The old version should be archived first via ArchiveMessageToHistory.
// Updates: message content, meta (extracted_metadata), updated_at, and modified_at.
func UpdateAnalysisMessage(db *sql.DB, messageID int64, content string, meta map[string]interface{}) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	query := `UPDATE messages SET message = ?, meta = ?, updated_at = ?, modified_at = ? WHERE id = ? AND deleted = 0`
	result, err := db.Exec(query, content, string(metaJSON), now, now, messageID)
	if err != nil {
		return fmt.Errorf("failed to update analysis message %d: %w", messageID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message %d not found or already deleted", messageID)
	}

	logger.Debug("Updated analysis message in place", "message_id", messageID)
	return nil
}

// SetAnalysisBulk inserts or updates analysis records for a batch of files.
// This is the primary entry point for the 'gsc app analysis set' command.
//
// For each item, the function routes through one of four paths:
//   - NOT EXISTS → INSERT: Attach as child of the latest leaf message in the chat.
//   - EXISTS + force=false + unchanged hash → SKIP: Increment skipped counter.
//   - EXISTS + force=false + changed or missing hash → UPDATE: Archive old to message_history, update in place.
//   - EXISTS + force=true → UPDATE: Archive old to message_history, update in place.
//
// After each insert or update, the chat's token metadata is updated with the new estimate
// (matching the JavaScript implementation pattern which updates tokens for every processed item).
//
// Batched transactions are used for performance, matching the LoadAnalysis pattern.
func SetAnalysisBulk(db *sql.DB, items []SetItem, force bool, batchSize int, onProgress func(n int, chatID int64)) (*SetResult, error) {
	if len(items) == 0 {
		return &SetResult{}, nil
	}

	result := &SetResult{}

	// Use default batch size if not specified
	if batchSize <= 0 {
		batchSize = 1000
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare existence check statement (bound to transaction)
	checkStmt, err := tx.Prepare(`SELECT id, meta FROM messages WHERE chat_id = ? AND type = ? AND deleted = 0 LIMIT 1`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare check statement: %w", err)
	}
	defer checkStmt.Close()

	for i, item := range items {
		processed := false

		// 1. Check for existing analysis message
		var existingID int64
		var existingMetaJSON sql.NullString
		err := checkStmt.QueryRow(item.ChatID, item.Analyzer).Scan(&existingID, &existingMetaJSON)

		if err == nil {
			// --- EXISTS ---
			if force {
				if archiveErr := archiveMsgInTx(tx, existingID); archiveErr != nil {
					logger.Warning("Failed to archive message, skipping", "chat_id", item.ChatID, "error", archiveErr)
					result.Errors++
				} else if updateErr := updateMsgInTx(tx, existingID, item.Content, item.Meta); updateErr != nil {
					logger.Warning("Failed to update message, skipping", "chat_id", item.ChatID, "error", updateErr)
					result.Errors++
				} else {
					result.Updated++
					processed = true
				}
			} else {
				incomingHash := extractAnalysisHash(item.Meta)
				existingHash := extractExistingAnalysisHash(&existingMetaJSON)

				if incomingHash != "" && existingHash != "" && incomingHash == existingHash {
					result.Skipped++
				} else if archiveErr := archiveMsgInTx(tx, existingID); archiveErr != nil {
					logger.Warning("Failed to archive message, skipping", "chat_id", item.ChatID, "error", archiveErr)
					result.Errors++
				} else if updateErr := updateMsgInTx(tx, existingID, item.Content, item.Meta); updateErr != nil {
					logger.Warning("Failed to update message, skipping", "chat_id", item.ChatID, "error", updateErr)
					result.Errors++
				} else {
					result.Updated++
					processed = true
				}
			}
		} else if err == sql.ErrNoRows {
			// --- NOT EXISTS: insert new message ---
			if insertErr := insertAnalysisInTx(tx, item); insertErr != nil {
				logger.Warning("Failed to insert analysis message, skipping", "chat_id", item.ChatID, "error", insertErr)
				result.Errors++
			} else {
				result.Inserted++
				processed = true
			}
		} else {
			// --- Database error ---
			logger.Warning("Database error checking existence, skipping", "chat_id", item.ChatID, "error", err)
			result.Errors++
		}

		// 2. Update chat token metadata (only if the message was inserted or updated)
		if processed {
			if tokenErr := updateChatTokensInTx(tx, item.ChatID, item.Analyzer, item.Content); tokenErr != nil {
				logger.Warning("Failed to update chat tokens", "chat_id", item.ChatID, "error", tokenErr)
				// Non-fatal: message was already committed to the messages table
			}
		}

		// 3. Progress callback
		if onProgress != nil {
			onProgress(i+1, item.ChatID)
		}

		// 4. Batch commit
		if (i+1)%batchSize == 0 {
			checkStmt.Close()
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("failed to commit batch at item %d: %w", i+1, err)
			}
			tx, err = db.Begin()
			if err != nil {
				return nil, fmt.Errorf("failed to begin new transaction at item %d: %w", i+1, err)
			}
			checkStmt, err = tx.Prepare(`SELECT id, meta FROM messages WHERE chat_id = ? AND type = ? AND deleted = 0 LIMIT 1`)
			if err != nil {
				return nil, fmt.Errorf("failed to re-prepare check statement: %w", err)
			}
		}
	}

	// Commit remaining records
	checkStmt.Close()
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit final batch: %w", err)
	}

	return result, nil
}

// =====================================================================
// PRIVATE HELPERS (Set Analysis - transaction-aware)
// =====================================================================

func extractAnalysisHash(meta map[string]interface{}) string {
	if hash, ok := meta["analysis_hash"].(string); ok {
		return hash
	}
	return ""
}

func extractExistingAnalysisHash(existingMetaJSON *sql.NullString) string {
	if existingMetaJSON == nil || !existingMetaJSON.Valid {
		return ""
	}

	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(existingMetaJSON.String), &meta); err != nil {
		return ""
	}

	if hash, ok := meta["analysis_hash"].(string); ok {
		return hash
	}
	return ""
}

// archiveMsgInTx copies a message to message_history within an active transaction.
func archiveMsgInTx(tx *sql.Tx, messageID int64) error {
	query := `
		INSERT INTO message_history (id, type, deleted, visibility, chat_id, parent_id, level,
			hash, message, chat_completion_stats, meta, created_at, updated_at, modified_at)
		SELECT id, type, deleted, visibility, chat_id, parent_id, level,
			hash, message, chat_completion_stats, meta, created_at, updated_at, modified_at
		FROM messages
		WHERE id = ?`

	result, err := tx.Exec(query, messageID)
	if err != nil {
		return fmt.Errorf("failed to archive message %d: %w", messageID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message %d not found for archival", messageID)
	}

	return nil
}

// updateMsgInTx updates a message's content and meta within an active transaction.
// Sets: message, meta, updated_at, modified_at. Preserves: type, id, chat_id, parent_id.
func updateMsgInTx(tx *sql.Tx, messageID int64, content string, meta map[string]interface{}) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	query := `UPDATE messages SET message = ?, meta = ?, updated_at = ?, modified_at = ? WHERE id = ? AND deleted = 0`
	result, err := tx.Exec(query, content, string(metaJSON), now, now, messageID)
	if err != nil {
		return fmt.Errorf("failed to update message %d: %w", messageID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("message %d not found or already deleted", messageID)
	}

	return nil
}

// insertAnalysisInTx inserts a new analysis message within an active transaction.
// It resolves the latest leaf message and chat's main_model, then inserts as a child.
// The level is set to parent.level + 1, matching the JavaScript INSERT pattern.
func insertAnalysisInTx(tx *sql.Tx, item SetItem) error {
	// 1. Find the latest leaf message (true leaf, not deepest path)
	leafQuery := `
		SELECT m.id, IFNULL(m.level, 0)
		FROM messages m
		WHERE m.chat_id = ?
			AND m.deleted = 0
			AND NOT EXISTS (
				SELECT 1 FROM messages m2
				WHERE m2.parent_id = m.id
					AND m2.chat_id = m.chat_id
					AND m2.deleted = 0
			)
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1`

	var parentID int64
	var parentLevel int
	if err := tx.QueryRow(leafQuery, item.ChatID).Scan(&parentID, &parentLevel); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no leaf message found for chat %d - chat may be empty", item.ChatID)
		}
		return fmt.Errorf("failed to find leaf message for chat %d: %w", item.ChatID, err)
	}

	// 2. Get the chat's main_model
	var mainModel string
	if err := tx.QueryRow("SELECT main_model FROM chats WHERE id = ?", item.ChatID).Scan(&mainModel); err != nil {
		return fmt.Errorf("failed to get main_model for chat %d: %w", item.ChatID, err)
	}

	// 3. Marshal meta
	metaJSON, err := json.Marshal(item.Meta)
	if err != nil {
		return fmt.Errorf("failed to marshal meta for chat %d: %w", item.ChatID, err)
	}

	// 4. Calculate hash and timestamps
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	nowTime := time.Now().UTC()
	hash := calculateMessageHash(item.Content, nowTime)
	level := parentLevel + 1

	// 5. Insert the message
	insertQuery := `
		INSERT INTO messages (
			type, deleted, visibility, chat_id, parent_id, level,
			hash, sample, model, real_model, temperature,
			role, message, meta, created_at, updated_at
		) VALUES (
			?, 0, 'public', ?, ?, ?,
			?, 1, ?, ?, 0,
			'assistant', ?, ?, ?, ?
		)`

	result, err := tx.Exec(
		insertQuery,
		item.Analyzer,    // type
		item.ChatID,      // chat_id
		parentID,         // parent_id
		level,            // level (parent + 1)
		hash,             // hash
		mainModel,        // model (chat's main_model)
		item.SourceModel, // real_model (e.g., "external::syn")
		item.Content,     // message
		string(metaJSON), // meta
		now,              // created_at
		now,              // updated_at
	)
	if err != nil {
		return fmt.Errorf("failed to insert analysis message for chat %d: %w", item.ChatID, err)
	}

	msgID, _ := result.LastInsertId()
	logger.Debug("Inserted analysis message", "chat_id", item.ChatID, "msg_id", msgID, "parent_id", parentID, "level", level)

	return nil
}

// updateChatTokensInTx updates the chat's meta with token estimates for this analyzer.
// It reads the current meta, merges the token info at meta.tokens.analysis[analyzer],
// and writes it back. This preserves all existing meta fields, matching the JavaScript
// updateChatAnalysisMessages pattern which reads → modifies → writes the full meta object.
func updateChatTokensInTx(tx *sql.Tx, chatID int64, analyzer string, content string) error {
	// 1. Read current meta
	var metaStr sql.NullString
	if err := tx.QueryRow("SELECT meta FROM chats WHERE id = ?", chatID).Scan(&metaStr); err != nil {
		return fmt.Errorf("failed to read chat meta for chat %d: %w", chatID, err)
	}

	// 2. Parse existing meta (or start fresh)
	meta := make(map[string]interface{})
	if metaStr.Valid && metaStr.String != "" {
		if err := json.Unmarshal([]byte(metaStr.String), &meta); err != nil {
			logger.Warning("Failed to parse chat meta, starting fresh", "chat_id", chatID, "error", err)
			meta = make(map[string]interface{})
		}
	}

	// 3. Navigate to meta.tokens.analysis, creating intermediate objects as needed
	tokens, _ := meta["tokens"].(map[string]interface{})
	if tokens == nil {
		tokens = make(map[string]interface{})
		meta["tokens"] = tokens
	}

	analysisTokens, _ := tokens["analysis"].(map[string]interface{})
	if analysisTokens == nil {
		analysisTokens = make(map[string]interface{})
		tokens["analysis"] = analysisTokens
	}

	// 4. Set the token estimate for this analyzer
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	analysisTokens[analyzer] = map[string]interface{}{
		"estimate":    estimateTokens(content),
		"estimatedAt": now,
	}

	// 5. Write back the full meta object
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal updated chat meta: %w", err)
	}

	if _, err := tx.Exec(
		"UPDATE chats SET meta = ?, updated_at = ? WHERE id = ?",
		string(metaJSON), now, chatID,
	); err != nil {
		return fmt.Errorf("failed to update chat meta for chat %d: %w", chatID, err)
	}

	return nil
}

// estimateTokens provides a rough token count estimate based on character length.
// Uses approximately 4 characters per token, which is a standard heuristic for English text.
func estimateTokens(content string) int {
	if content == "" {
		return 0
	}
	return len(content) / 4
}
