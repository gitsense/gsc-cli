/**
 * Component: CLI Bridge Orchestrator
 * Block-UUID: 988e3eee-f244-49a4-85ea-48f6244e7e44
 * Parent-UUID: f77d7e2d-db29-41b1-8bb1-e00a933c9495
 * Version: 1.3.1
 * Description: Orchestrates the CLI Bridge lifecycle, including handshake file management, terminal prompts, signal handling, and database integration. Added the main Execute entry point with signal handling for SIGINT/SIGTERM and terminal prompt logic for user confirmation. Implemented bloat protection for the handshake file by truncating the output preview if it exceeds 100KB. Added debug logging to trace GSC_HOME resolution and handshake file path construction. Added helpful error message when GSC_HOME is not set and the handshake file is not found.
 * Language: Go
 * Created-at: 2026-02-08T19:06:26.138Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), Gemini 3 Flash (v1.3.1)
 */


package bridge

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/internal/output"
	"github.com/yourusername/gsc-cli/pkg/logger"
	"github.com/yourusername/gsc-cli/pkg/settings"
)

// BridgeError represents an error that occurred during bridge execution
// with a specific exit code for the CLI.
type BridgeError struct {
	ExitCode int
	Message  string
	Err      error
}

func (e *BridgeError) Error() string {
	return e.Message
}

// Handshake represents the JSON handshake file created by the Web UI.
type Handshake struct {
	Code              string    `json:"code"`
	ChatID            int64     `json:"chatId"`
	ChatUUID          string    `json:"chatUuid"`
	ChatTitle         string    `json:"chatTitle"`
	ParentMessageID   int64     `json:"parentMessageId"`
	DBPath            string    `json:"dbPath"`
	GSCHome           string    `json:"gscHome"`
	ExpiresAt         int64     `json:"expiresAt"` // Milliseconds
	CreatedAt         int64     `json:"createdAt"` // Milliseconds
	DefaultVisibility string    `json:"defaultVisibility"`
	Consumer          string    `json:"consumer"`
	Status            string    `json:"status"`
	Command           *string   `json:"command"`
	StartedAt         *int64    `json:"startedAt"`
	FinishedAt        *int64    `json:"finishedAt"`
	Error             *Error    `json:"error"`
	Result            Result    `json:"result"`
	MaxOutputSize     int64     `json:"maxOutputSize"`
}

type Error struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type Result struct {
	MessageID  *int64  `json:"messageId"`
	Output     *string `json:"output"`
	OutputSize *int64  `json:"outputSize"`
}

// Execute is the main entry point for the CLI Bridge.
// It handles the entire lifecycle from loading the handshake to final insertion.
func Execute(code string, rawOutput string, format string, cmdStr string, duration time.Duration, dbName string, force bool) error {
	// 1. Resolve GSC_HOME and Load Handshake
	gscHome, err := resolveGSCHome()
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	h, err := LoadHandshake(gscHome, code)
	if err != nil {
		return err // Already formatted
	}

	// 2. Setup Signal Handling (SIGINT/SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		// Hard 500ms timeout for cleanup write
		done := make(chan bool, 1)
		go func() {
			h.UpdateStatus("error", &Error{Code: "USER_ABORTED", Message: "Process interrupted by user"})
			done <- true
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		os.Exit(2) // Standardized exit code for User Abort
	}()

	// 3. Update Status to Running
	if err := h.UpdateStatus("running", nil); err != nil {
		return err
	}

	// 4. Format the Markdown Message
	markdown := output.FormatBridgeMarkdown(cmdStr, duration, dbName, format, rawOutput)
	outputSize := int64(len(markdown))

	// 5. Size Validation & Confirmation
	if outputSize > h.MaxOutputSize {
		h.UpdateStatus("oversized", nil)
		fmt.Fprintf(os.Stderr, "\n⚠️  Output (%.2f MB) exceeds the %.2f MB limit. Proceed anyway? [y/N] ", 
			float64(outputSize)/1024/1024, float64(h.MaxOutputSize)/1024/1024)
		
		if !askConfirmation(false) {
			h.UpdateStatus("error", &Error{Code: "USER_ABORTED_OVERSIZED", Message: "User declined oversized output"})
			return &BridgeError{ExitCode: 4, Message: "message was not added to chat (size exceeded limit)"}
		}
	} else if !force {
		h.UpdateStatus("awaiting-confirmation", nil)
		// Show hint for JSON format if in human mode
		if strings.ToLower(format) == "human" {
			fmt.Fprintln(os.Stderr, "\nHint: For better AI analysis, use '--format json' to provide structured data.")
		}

		fmt.Fprintf(os.Stderr, "Insert into chat \"%s\" (%.2f MB)? [Y/n] ", 
			h.ChatTitle, float64(outputSize)/1024/1024)
		
		if !askConfirmation(true) {
			h.UpdateStatus("error", &Error{Code: "USER_ABORTED", Message: "User declined insertion"})
			return &BridgeError{ExitCode: 2, Message: "message was not added to chat"}
		}
	}

	// 6. Database Insertion
	msgID, err := h.InsertToChat(markdown)
	if err != nil {
		h.UpdateStatus("error", &Error{Code: "ERR_DB_INSERT", Message: err.Error()})
		return &BridgeError{ExitCode: 3, Message: err.Error(), Err: err}
	}

	// 7. Success & Cleanup
	h.Result.MessageID = &msgID
	h.Result.OutputSize = &outputSize
	
	// Bloat Protection: Truncate output in JSON if > 100KB
	preview := markdown
	if outputSize > 102400 {
		preview = markdown[:102400] + "\n\n[Output truncated in handshake file. Full content is in the chat database.]"
	}
	h.Result.Output = &preview

	if err := h.UpdateStatus("success", nil); err != nil {
		return err
	}

	h.Cleanup()
	fmt.Fprintf(os.Stderr, "\n[BRIDGE] Success! Message ID: %d\n", msgID)
	fmt.Fprintln(os.Stderr, "[BRIDGE] Note: This bridge code has been consumed and cannot be reused.")

	return nil
}

// LoadHandshake reads and validates the handshake file for a given code.
func LoadHandshake(gscHome, code string) (*Handshake, error) {
	logger.Debug("Loading handshake", "gscHome", gscHome, "code", "code")
	
	path := filepath.Join(gscHome, settings.BridgeHandshakeDir, code+".json")
	logger.Debug("Handshake file path", "path", path, "dir", settings.BridgeHandshakeDir)
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if GSC_HOME is set to provide a helpful hint
			if os.Getenv("GSC_HOME") == "" {
				return nil, fmt.Errorf("bridge code %s not found or expired.\n\nHint: GSC_HOME is not set. The CLI looked in the default location: %s", code, path)
			}
			return nil, fmt.Errorf("bridge code %s not found or expired", code)
		}
		return nil, fmt.Errorf("failed to read handshake file: %w", err)
	}

	var h Handshake
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("failed to parse handshake file: %w", err)
	}

	// Validate expiration
	now := time.Now().UnixNano() / 1e6
	if h.ExpiresAt < now {
		return nil, fmt.Errorf("bridge code %s has expired", code)
	}

	// Validate consumer
	if h.Consumer != "gsc" {
		return nil, fmt.Errorf("invalid consumer: %s", h.Consumer)
	}

	return &h, nil
}

// UpdateStatus performs an atomic write to update the handshake file status.
func (h *Handshake) UpdateStatus(status string, errObj *Error) error {
	h.Status = status
	if errObj != nil {
		h.Error = errObj
	}
	
	now := time.Now().UnixNano() / 1e6
	if status == "running" {
		h.StartedAt = &now
	} else if status == "success" || status == "error" {
		h.FinishedAt = &now
	}

	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal handshake: %w", err)
	}

	path := filepath.Join(h.GSCHome, settings.BridgeHandshakeDir, h.Code+".json")
	tmpPath := path + ".tmp"

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp handshake file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename handshake file: %w", err)
	}

	return nil
}

// InsertToChat performs the database insertion logic.
func (h *Handshake) InsertToChat(markdown string) (int64, error) {
	sqliteDB, err := db.OpenDB(h.DBPath)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to chat database: %w", err)
	}
	defer sqliteDB.Close()

	// 1. Validate Parent and fetch level
	parent, err := db.GetMessage(sqliteDB, h.ParentMessageID)
	if err != nil {
		return 0, fmt.Errorf("parent validation failed: %w", err)
	}

	// 2. Verify Leaf Node
	isLeaf, err := db.IsLeafNode(sqliteDB, h.ParentMessageID)
	if err != nil {
		return 0, err
	}
	if !isLeaf {
		return 0, fmt.Errorf("cannot reply to message %d: it already has replies", h.ParentMessageID)
	}

	// 3. Prepare Message
	msg := &db.Message{
		Type:       "gsc-cli-output",
		Deleted:    0,
		Visibility: h.DefaultVisibility,
		ChatID:     h.ChatID,
		ParentID:   h.ParentMessageID,
		Level:      parent.Level + 1,
		Role:       "assistant",
		RealModel:  sql.NullString{String: settings.RealModelNotes, Valid: true},
		Temperature: sql.NullFloat64{Float64: 0, Valid: true},
		Message:    sql.NullString{String: markdown, Valid: true},
	}

	// 4. Insert
	msgID, err := db.InsertMessage(sqliteDB, msg)
	if err != nil {
		return 0, err
	}

	return msgID, nil
}

// Cleanup deletes the handshake file upon success.
func (h *Handshake) Cleanup() {
	path := filepath.Join(h.GSCHome, settings.BridgeHandshakeDir, h.Code+".json")
	if err := os.Remove(path); err != nil {
		logger.Debug("[BRIDGE] Failed to delete handshake file", "path", path, "error", err)
	}
}

// resolveGSCHome determines the GSC_HOME directory.
func resolveGSCHome() (string, error) {
	if gscHome := os.Getenv("GSC_HOME"); gscHome != "" {
		logger.Debug("Resolved GSC_HOME", "path", gscHome, "source", "env")
		return gscHome, nil
	}
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	defaultPath := filepath.Join(homeDir, ".gitsense")
	logger.Debug("Resolved GSC_HOME", "path", defaultPath, "source", "default")
	return defaultPath, nil
}

// askConfirmation prompts the user for a Y/n or y/N response.
func askConfirmation(defaultYes bool) bool {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}
