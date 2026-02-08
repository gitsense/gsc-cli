/**
 * Component: CLI Bridge Orchestrator
 * Block-UUID: bcb2fffe-0d56-4e3e-a646-07397fec6087
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Orchestrates the CLI Bridge lifecycle, including handshake file management and database integration.
 * Language: Go
 * Created-at: 2026-02-08T04:18:45.123Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package bridge

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

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

// LoadHandshake reads and validates the handshake file for a given code.
func LoadHandshake(gscHome, code string) (*Handshake, error) {
	path := filepath.Join(gscHome, "data", "codes", code+".json")
	
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
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

	path := filepath.Join(h.GSCHome, "data", "codes", h.Code+".json")
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

	// 1. Validate Parent
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
		Role:       "system",
		RealModel:  sql.NullString{String: "GitSense Notes", Valid: true},
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
	path := filepath.Join(h.GSCHome, "data", "codes", h.Code+".json")
	if err := os.Remove(path); err != nil {
		logger.Debug("[BRIDGE] Failed to delete handshake file", "path", path, "error", err)
	}
}
