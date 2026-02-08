/**
 * Component: Chat Database Operations
 * Block-UUID: f3c83cb7-fbf3-4c19-a8c2-5530e4eb77d0
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Library methods for interacting with the GitSense Chat database, including message insertion and validation.
 * Language: Go
 * Created-at: 2026-02-08T04:14:23.936Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package db

import (
	"database/sql"
	"fmt"
	"time"
)

// GetMessage retrieves a message by its ID.
// This is used to fetch the parent message's level.
func GetMessage(db *sql.DB, id int64) (*Message, error) {
	query := `SELECT id, chat_id, level FROM messages WHERE id = ? AND deleted = 0`
	
	var msg Message
	err := db.QueryRow(query, id).Scan(&msg.ID, &msg.ChatID, &msg.Level)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to query message: %w", err)
	}

	return &msg, nil
}

// IsLeafNode returns true if the message has no children.
// The bridge requires that we only reply to leaf nodes to maintain a clean thread.
func IsLeafNode(db *sql.DB, id int64) (bool, error) {
	query := `SELECT 1 FROM messages WHERE parent_id = ? AND deleted = 0 LIMIT 1`
	
	var exists int
	err := db.QueryRow(query, id).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return true, nil
		}
		return false, fmt.Errorf("failed to check for children: %w", err)
	}

	return false, nil
}

// InsertMessage inserts a new message record.
// It uses a subquery to automatically resolve the 'model' from the parent chat.
func InsertMessage(db *sql.DB, msg *Message) (int64, error) {
	query := `
		INSERT INTO messages (
			type, deleted, visibility, chat_id, parent_id, level, 
			model, real_model, temperature, role, message, 
			created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, 
			(SELECT main_model FROM chats WHERE id = ?), 
			?, ?, ?, ?, ?, ?
		)`

	now := time.Now()
	
	result, err := db.Exec(
		query,
		msg.Type,
		msg.Deleted,
		msg.Visibility,
		msg.ChatID,
		msg.ParentID,
		msg.Level,
		msg.ChatID, // For the subquery
		msg.RealModel,
		msg.Temperature,
		msg.Role,
		msg.Message,
		now,
		now,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}
