/*
 * Component: Analysis Copy Database Operations
 * Block-UUID: f154a420-bb0b-4a0a-a6b9-819db65db187
 * Parent-UUID: 83eb26d4-139c-4fcb-927f-7d7d8bd1dce3
 * Version: 1.9.0
 * Description: Handles the database logic for copying analyzer metadata between branches, and new functionality for dumping analysis results to JSONL format. v1.6.0: Added incremental dump support using analysis_hash. Added ComputeAnalysisHash and BuildExistingHashSet functions. Updated DumpAnalysis to accept existingHashes map and return written/skipped counts. Updated LoadAnalysis to populate hash column with analysis_hash. v1.7.0: Added Owner field to DumpRecord for portable dump files. Updated DumpAnalysis signature to accept owner and repo separately. v1.8.0: Added GetUniqueAnalyzers function to discover all distinct analyzer prefixes for a given branch, enabling multi-analyzer rebuild support. v1.9.0: Fixed LoadAnalysis to insert analyzer messages in a linear chain (parent-child) instead of as siblings. Candidates are now sorted by ChatID and Analyzer, and parent_id is dynamically tracked to ensure correct message ordering.
 * Language: Go
 * Created-at: 2026-05-14T18:40:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0)
 */


package db

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	Owner        string                 `json:"owner"`        // v1.7.0: Added for portable dump files
	Repo         string                 `json:"repo"`         // v1.7.0: Now stores only repo name (not owner/repo)
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
