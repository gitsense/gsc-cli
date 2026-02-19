/**
 * Component: Chat Database Operations
 * Block-UUID: da0d314e-caaa-4dc7-b64e-72554161cccd
 * Parent-UUID: b307fcec-1757-4ade-83f2-a71dd56382ca
 * Version: 1.2.0
 * Description: Expanded library methods for hierarchical chat management, message upserts, and manifest indexing. Enforces strict ISO 8601 UTC timestamps and supports the "Find or Create" pattern for intelligence manifest pages.
 * Language: Go
 * Created-at: 2026-02-19T18:08:53.555Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0)
 */


package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GetMessage retrieves a message by its ID.
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

// FindChatByUUID retrieves a chat by its UUID.
func FindChatByUUID(db *sql.DB, uuidStr string) (*Chat, error) {
	query := `SELECT id, uuid, type, visibility, owner, name, parent_id, main_model FROM chats WHERE uuid = ? AND deleted = 0`
	
	var c Chat
	err := db.QueryRow(query, uuidStr).Scan(&c.ID, &c.UUID, &c.Type, &c.Visibility, &c.Owner, &c.Name, &c.ParentID, &c.MainModel)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query chat by uuid: %w", err)
	}
	return &c, nil
}

// FindChatByTypeAndName retrieves a chat by its type, name, and parent ID.
// This is used for hierarchical discovery of manifest pages.
func FindChatByTypeAndName(db *sql.DB, chatType string, name string, parentID int64) (*Chat, error) {
	query := `SELECT id, uuid, type, visibility, owner, name, parent_id, main_model FROM chats WHERE type = ? AND name = ? AND parent_id = ? AND deleted = 0`
	
	var c Chat
	err := db.QueryRow(query, chatType, name, parentID).Scan(&c.ID, &c.UUID, &c.Type, &c.Visibility, &c.Owner, &c.Name, &c.ParentID, &c.MainModel)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query chat by type and name: %w", err)
	}
	return &c, nil
}

// InsertChat creates a new chat record.
func InsertChat(db *sql.DB, chat *Chat) (int64, error) {
	if chat.UUID == "" {
		chat.UUID = uuid.New().String()
	}

	query := `
		INSERT INTO chats (
			type, deleted, visibility, uuid, owner, name, parent_id, 
			group_id, prompt_id, main_model, protected, is_default_name, 
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	result, err := db.Exec(
		query,
		chat.Type,
		0, // deleted
		chat.Visibility,
		chat.UUID,
		chat.Owner,
		chat.Name,
		chat.ParentID,
		chat.GroupID,
		chat.PromptID,
		chat.MainModel,
		1, // protected
		0, // is_default_name
		now,
		now,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert chat: %w", err)
	}

	return result.LastInsertId()
}

// FindMessageByRoleAndType finds a specific message within a chat.
func FindMessageByRoleAndType(db *sql.DB, chatID int64, role string, msgType string) (*Message, error) {
	query := `SELECT id, chat_id, parent_id, level, message FROM messages WHERE chat_id = ? AND role = ? AND type = ? AND deleted = 0 LIMIT 1`
	
	var msg Message
	err := db.QueryRow(query, chatID, role, msgType).Scan(&msg.ID, &msg.ChatID, &msg.ParentID, &msg.Level, &msg.Message)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query message by role and type: %w", err)
	}
	return &msg, nil
}

// UpdateMessage updates the content of an existing message.
func UpdateMessage(db *sql.DB, id int64, content string) error {
	query := `UPDATE messages SET message = ?, updated_at = ? WHERE id = ?`
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	_, err := db.Exec(query, content, now, id)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}
	return nil
}

// InsertMessage inserts a new message record.
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

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	result, err := db.Exec(
		query,
		msg.Type,
		msg.Deleted,
		msg.Visibility,
		msg.ChatID,
		msg.ParentID,
		msg.Level,
		msg.ChatID,
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

	return result.LastInsertId()
}

// InsertPublishedManifest records a new publication in the index.
func InsertPublishedManifest(db *sql.DB, m *PublishedManifest) (int64, error) {
	query := `
		INSERT INTO published_manifests (
			uuid, owner, repo, branch, database, published_at, 
			root_chat_id, owner_chat_id, repo_chat_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	result, err := db.Exec(
		query,
		m.UUID,
		m.Owner,
		m.Repo,
		m.Branch,
		m.Database,
		now,
		m.RootChatID,
		m.OwnerChatID,
		m.RepoChatID,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert published manifest: %w", err)
	}

	return result.LastInsertId()
}

// DeletePublishedManifest performs a soft delete on a manifest record.
func DeletePublishedManifest(db *sql.DB, uuidStr string) error {
	query := `UPDATE published_manifests SET deleted = 1 WHERE uuid = ?`
	_, err := db.Exec(query, uuidStr)
	if err != nil {
		return fmt.Errorf("failed to delete published manifest: %w", err)
	}
	return nil
}

// GetActiveManifests retrieves active manifests based on owner and repo filters.
func GetActiveManifests(db *sql.DB, owner, repo string) ([]PublishedManifest, error) {
	var query string
	var args []interface{}

	if owner == "" {
		// Root level: Get unique owners
		query = `SELECT DISTINCT owner FROM published_manifests WHERE deleted = 0 ORDER BY owner ASC`
	} else if repo == "" {
		// Owner level: Get unique repos for owner
		query = `SELECT DISTINCT repo FROM published_manifests WHERE owner = ? AND deleted = 0 ORDER BY repo ASC`
		args = append(args, owner)
	} else {
		// Repo level: Get all branches for repo
		query = `SELECT uuid, branch, database, published_at FROM published_manifests WHERE owner = ? AND repo = ? AND deleted = 0 ORDER BY published_at DESC`
		args = append(args, owner, repo)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query active manifests: %w", err)
	}
	defer rows.Close()

	var manifests []PublishedManifest
	for rows.Next() {
		var m PublishedManifest
		if owner == "" {
			err = rows.Scan(&m.Owner)
		} else if repo == "" {
			err = rows.Scan(&m.Repo)
		} else {
			var publishedAtStr string
			err = rows.Scan(&m.UUID, &m.Branch, &m.Database, &publishedAtStr)
			if err == nil {
				m.PublishedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", publishedAtStr)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("failed to scan manifest row: %w", err)
		}
		manifests = append(manifests, m)
	}

	return manifests, nil
}
