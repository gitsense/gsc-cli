/**
 * Component: CLI Bridge Orchestrator
 * Block-UUID: e199447b-efa6-4317-8950-81021425300c
 * Parent-UUID: 683dcaff-d2da-47ee-b619-ab88c0899e36
 * Version: 1.5.0
 * Description: Orchestrates the CLI Bridge lifecycle. Added stage-based validation (Discovery, Execution, Insertion) to handle long-running tasks and prevent race conditions. ValidateCode now enforces strict state checks (e.g., StartedAt must be nil for new tasks) and ensures codes haven't expired or been invalidated during execution.
 * Language: Go
 * Created-at: 2026-02-09T02:43:34.123Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), Gemini 3 Flash (v1.3.1), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0)
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

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// BridgeStage defines the lifecycle point for validation
type BridgeStage int

const (
	// StageDiscovery is the initial check by the CLI before any work starts.
	StageDiscovery BridgeStage = iota
	// StageExecution is the check performed when bridge.Execute begins.
	StageExecution
	// StageInsertion is the final check after work is done, before DB insertion.
	StageInsertion
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
func Execute(code string, rawOutput string, format string, cmdStr string, duration time.Duration, dbName string, force bool) error {
	// 1. Resolve GSC_HOME and Load Handshake
	gscHome, err := resolveGSCHome()
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// Use StageExecution for the initial load
	h, err := LoadHandshake(gscHome, code, StageExecution)
	if err != nil {
		return err
	}

	// 2. Setup Signal Handling (SIGINT/SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		done := make(chan bool, 1)
		go func() {
			h.UpdateStatus("error", &Error{Code: "USER_ABORTED", Message: "Process interrupted by user"})
			done <- true
		}()
		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
		}
		os.Exit(2)
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
		if strings.ToLower(format) == "human" {
			fmt.Fprintln(os.Stderr, "\nHint: For better AI analysis, use '--format json' to provide structured data.")
		}

		fmt.Fprintf(os.Stderr, "\n\nInsert into chat \"%s\" (%.2f MB)? [Y/n] ", 
			h.ChatTitle, float64(outputSize)/1024/1024)
		
		if !askConfirmation(true) {
			h.UpdateStatus("error", &Error{Code: "USER_ABORTED", Message: "User declined insertion"})
			return &BridgeError{ExitCode: 2, Message: "message was not added to chat"}
		}
	}

	// 6. Final Validation: Ensure code is still valid after potentially long execution/wait
	if err := ValidateCode(code, StageInsertion); err != nil {
		return &BridgeError{ExitCode: 2, Message: err.Error()}
	}

	// 7. Database Insertion
	msgID, err := h.InsertToChat(markdown)
	if err != nil {
		h.UpdateStatus("error", &Error{Code: "ERR_DB_INSERT", Message: err.Error()})
		return &BridgeError{ExitCode: 3, Message: err.Error(), Err: err}
	}

	// 8. Success & Cleanup
	h.Result.MessageID = &msgID
	h.Result.OutputSize = &outputSize
	
	preview := markdown
	if outputSize > 102400 {
		preview = markdown[:102400] + "\n\n[Output truncated in handshake file. Full content is in the chat database.]"
	}
	h.Result.Output = &preview

	if err := h.UpdateStatus("success", nil); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\n[BRIDGE] Success! Message ID: %d\n", msgID)
	fmt.Fprintln(os.Stderr, "[BRIDGE] Note: This bridge code has been consumed and cannot be reused.")

	return nil
}

// ValidateCode checks if a bridge code is valid for a specific lifecycle stage.
func ValidateCode(code string, stage BridgeStage) error {
	gscHome, err := resolveGSCHome()
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	path := filepath.Join(gscHome, settings.BridgeHandshakeDir, code+".json")

	// 1. Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		msg := fmt.Sprintf("bridge code %s not found or expired", code)
		if os.Getenv("GSC_HOME") == "" {
			msg += fmt.Sprintf(".\n\nHint: GSC_HOME is not set. The CLI looked in: %s", path)
		}
		return fmt.Errorf(msg)
	}

	// 2. Read and Parse
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read handshake file: %w", err)
	}

	var h Handshake
	if err := json.Unmarshal(data, &h); err != nil {
		return fmt.Errorf("failed to parse handshake file: %w", err)
	}

	// 3. Common Validations
	if h.Consumer != "gsc" {
		return fmt.Errorf("invalid consumer: %s", h.Consumer)
	}

	now := time.Now().UnixNano() / 1e6
	if h.ExpiresAt < now {
		return fmt.Errorf("bridge code %s has expired", code)
	}

	// 4. Stage-Specific Validations
	switch stage {
	case StageDiscovery, StageExecution:
		// Code must be fresh and unclaimed
		if h.Status != "pending" {
			return fmt.Errorf("bridge code %s is already in state: %s", code, h.Status)
		}
		if h.StartedAt != nil {
			return fmt.Errorf("bridge code %s was already started at %d", code, *h.StartedAt)
		}

	case StageInsertion:
		// Code must be in an active state and not finished
		if h.FinishedAt != nil {
			return fmt.Errorf("bridge code %s was already finished", code)
		}
		if h.StartedAt == nil {
			return fmt.Errorf("bridge code %s was never started", code)
		}
		// Ensure it wasn't marked as error by an external process/timeout
		if h.Status == "error" {
			return fmt.Errorf("bridge code %s was invalidated (status: error)", code)
		}
	}

	return nil
}

// LoadHandshake reads and validates the handshake file for a given code.
func LoadHandshake(gscHome, code string, stage BridgeStage) (*Handshake, error) {
	logger.Debug("Loading handshake", "gscHome", gscHome, "code", code)
	
	if err := ValidateCode(code, stage); err != nil {
		return nil, err
	}

	path := filepath.Join(gscHome, settings.BridgeHandshakeDir, code+".json")
	data, _ := os.ReadFile(path) // Already validated by ValidateCode
	
	var h Handshake
	json.Unmarshal(data, &h)
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

	parent, err := db.GetMessage(sqliteDB, h.ParentMessageID)
	if err != nil {
		return 0, fmt.Errorf("parent validation failed: %w", err)
	}

	isLeaf, err := db.IsLeafNode(sqliteDB, h.ParentMessageID)
	if err != nil {
		return 0, err
	}
	if !isLeaf {
		return 0, fmt.Errorf("cannot reply to message %d: it already has replies", h.ParentMessageID)
	}

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
