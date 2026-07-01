/**
 * Component: Pi Sessions Verify
 * Block-UUID: 8a2f3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Reconstructs Pi session JSONL from SQLite and verifies lossless round-trip fidelity.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package sessions

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
)

// LosslessReport holds the result of a lossless verification check.
type LosslessReport struct {
	SessionID         string `json:"session_id"`
	SessionFile       string `json:"session_file"`
	SourceBytes       int    `json:"source_bytes"`
	ReconstructedBytes int   `json:"reconstructed_bytes"`
	SourceSHA256      string `json:"source_sha256"`
	ReconstructedSHA256 string `json:"reconstructed_sha256"`
	Match             bool   `json:"match"`
	SourceMissing     bool   `json:"source_missing,omitempty"`
	Error             string `json:"error,omitempty"`
}

// ReconstructJSONL reads raw_header and raw_lines from SQLite and returns the
// reconstructed JSONL bytes. The format matches the original file:
// raw_header + "\n" + join(raw_lines, "\n") + "\n"
func ReconstructJSONL(ctx context.Context, database *sql.DB, sessionUUID string) ([]byte, error) {
	// Look up chat by uuid to get chat_id and raw_header
	var chatID int64
	var rawHeader string
	err := database.QueryRowContext(ctx,
		"SELECT id, raw_header FROM pi_chats WHERE uuid = ?", sessionUUID,
	).Scan(&chatID, &rawHeader)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session %q not found", sessionUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}

	// Read all raw_lines ordered by seq
	rows, err := database.QueryContext(ctx,
		"SELECT raw_line FROM pi_messages WHERE chat_id = ? ORDER BY seq", chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var rawLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		rawLines = append(rawLines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}

	// Reconstruct: header + newline + joined lines + newline
	reconstructed := rawHeader + "\n" + strings.Join(rawLines, "\n") + "\n"
	return []byte(reconstructed), nil
}

// VerifyLossless compares the reconstructed JSONL bytes from SQLite against
// the source file on disk. Returns a LosslessReport with match status.
func VerifyLossless(ctx context.Context, database *sql.DB, sessionUUID string) (*LosslessReport, error) {
	report := &LosslessReport{SessionID: sessionUUID}

	// Get session_file path from pi_chats
	var sessionFile string
	err := database.QueryRowContext(ctx,
		"SELECT session_file FROM pi_chats WHERE uuid = ?", sessionUUID,
	).Scan(&sessionFile)
	if err == sql.ErrNoRows {
		report.Error = fmt.Sprintf("session %q not found", sessionUUID)
		return report, fmt.Errorf("session %q not found", sessionUUID)
	}
	if err != nil {
		report.Error = fmt.Sprintf("lookup session: %v", err)
		return report, fmt.Errorf("lookup session: %w", err)
	}
	report.SessionFile = sessionFile

	// Read source file
	sourceBytes, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			report.SourceMissing = true
			report.Error = "source file missing"
			return report, nil
		}
		report.Error = fmt.Sprintf("read source: %v", err)
		return report, fmt.Errorf("read source: %w", err)
	}
	report.SourceBytes = len(sourceBytes)
	sourceHash := sha256.Sum256(sourceBytes)
	report.SourceSHA256 = hex.EncodeToString(sourceHash[:])

	// Reconstruct from SQLite
	reconstructedBytes, err := ReconstructJSONL(ctx, database, sessionUUID)
	if err != nil {
		report.Error = fmt.Sprintf("reconstruct: %v", err)
		return report, fmt.Errorf("reconstruct: %w", err)
	}
	report.ReconstructedBytes = len(reconstructedBytes)
	reconstructedHash := sha256.Sum256(reconstructedBytes)
	report.ReconstructedSHA256 = hex.EncodeToString(reconstructedHash[:])

	// Compare
	report.Match = report.SourceSHA256 == report.ReconstructedSHA256
	return report, nil
}

// VerifyLosslessWithDB opens the database and runs verification.
func VerifyLosslessWithDB(ctx context.Context, dbPath string, sessionUUID string) (*LosslessReport, error) {
	database, err := OpenQueryMirror(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)
	return VerifyLossless(ctx, database, sessionUUID)
}
