/**
 * Component: Chat Database Operations
 * Block-UUID: 696b977f-81a2-44b9-8feb-f82a939b0275
 * Parent-UUID: be880f6c-ea9c-4b6f-ac21-ed5a975c4f15
 * Version: 1.7.0
 * Description: Expanded library methods for hierarchical chat management. Updated GetActiveManifests to support repository counts at the Owner level to display the number of manifests per repository.
 * Language: Go
 * Created-at: 2026-02-20T04:31:47.873Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), GLM-4.7 (v1.7.0)
 */


package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
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

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	
	logger.Debug("InsertChat result", "id", id)
	return id, nil
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
	
	truncatedContent := truncateString(content, 100)
	logger.Debug("UpdateMessage", "query", query, "args", []interface{}{truncatedContent, now, id})
	
	result, err := db.Exec(query, content, now, id)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	logger.Debug("UpdateMessage result", "rows_affected", rowsAffected)
	return nil
}

// InsertMessage inserts a new message record.
func InsertMessage(db *sql.DB, msg *Message) (int64, error) {
	query := `
		INSERT INTO messages (
			type, deleted, visibility, chat_id, parent_id, level, 
			model, 
			real_model, temperature, role, message, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, 
			(SELECT main_model FROM chats WHERE id = ?), 
			?, ?, ?, ?, ?, ?
		)`

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	// Prepare args for logging (truncate message content)
	msgContent := ""
	if msg.Message.Valid {
		msgContent = truncateString(msg.Message.String, 100)
	}
	logger.Debug("InsertMessage", "query", query, "args", []interface{}{msg.Type, msg.Deleted, msg.Visibility, msg.ChatID, msg.ParentID, msg.Level, msg.ChatID, msg.RealModel, msg.Temperature, msg.Role, msgContent, now, now})

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

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	
	logger.Debug("InsertMessage result", "id", id)
	return id, nil
}

// InsertPublishedManifest records a new publication in the index.
func InsertPublishedManifest(db *sql.DB, m *PublishedManifest) (int64, error) {
	query := `
		INSERT INTO published_manifests (
			uuid, owner, repo, branch, database, schema_version, generated_at, 
			manifest_name, manifest_description, manifest_tags, repositories, branches, 
			hash, published_at, root_chat_id, owner_chat_id, repo_chat_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	result, err := db.Exec(
		query,
		m.UUID,
		m.Owner,
		m.Repo,
		m.Branch,
		m.Database,
		m.SchemaVersion,
		m.GeneratedAt.Format("2006-01-02T15:04:05.000Z"),
		m.ManifestName,
		m.ManifestDescription,
		m.ManifestTags,
		m.Repositories,
		m.Branches,
		m.Hash,
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

// FindManifestByHash retrieves a manifest by its content hash.
func FindManifestByHash(db *sql.DB, hash string) (*PublishedManifest, error) {
	query := `SELECT id, uuid, owner, repo, branch, root_chat_id, owner_chat_id, repo_chat_id FROM published_manifests WHERE hash = ? AND deleted = 0`
	
	var m PublishedManifest
	err := db.QueryRow(query, hash).Scan(&m.ID, &m.UUID, &m.Owner, &m.Repo, &m.Branch, &m.RootChatID, &m.OwnerChatID, &m.RepoChatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query manifest by hash: %w", err)
	}
	return &m, nil
}

// UpdateManifestTimestamp updates the published_at timestamp for an existing manifest (the "bump" logic).
func UpdateManifestTimestamp(db *sql.DB, id int64) error {
	query := `UPDATE published_manifests SET published_at = ? WHERE id = ?`
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	
	result, err := db.Exec(query, now, id)
	if err != nil {
		return fmt.Errorf("failed to update manifest timestamp: %w", err)
	}
	
	rowsAffected, _ := result.RowsAffected()
	logger.Debug("UpdateManifestTimestamp result", "rows_affected", rowsAffected)
	return nil
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
		// Root level: Get unique owners and their manifest counts
		query = `SELECT owner, COUNT(*) as count FROM published_manifests WHERE deleted = 0 GROUP BY owner ORDER BY owner ASC`
	} else if repo == "" {
		// Owner level: Get unique repos for owner and their manifest counts
		query = `SELECT repo, COUNT(*) as count FROM published_manifests WHERE owner = ? AND deleted = 0 GROUP BY repo ORDER BY repo ASC`
		args = append(args, owner)
	} else {
		// Repo level: Get all branches for repo
		query = `SELECT uuid, branch, database, manifest_name, published_at FROM published_manifests WHERE owner = ? AND repo = ? AND deleted = 0 ORDER BY published_at DESC`
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
			err = rows.Scan(&m.Owner, &m.ManifestCount)
		} else if repo == "" {
			err = rows.Scan(&m.Repo, &m.ManifestCount)
		} else {
			var publishedAtStr string
			err = rows.Scan(&m.UUID, &m.Branch, &m.Database, &m.ManifestName, &publishedAtStr)
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

// GetGlobalRecentManifests retrieves the most recently published manifests across all repositories.
func GetGlobalRecentManifests(db *sql.DB, limit int) ([]PublishedManifest, error) {
	query := `
		SELECT uuid, owner, repo, manifest_name, published_at 
		FROM published_manifests 
		WHERE deleted = 0 
		ORDER BY published_at DESC 
		LIMIT ?`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query global recent manifests: %w", err)
	}
	defer rows.Close()

	var manifests []PublishedManifest
	for rows.Next() {
		var m PublishedManifest
		var publishedAtStr string
		if err := rows.Scan(&m.UUID, &m.Owner, &m.Repo, &m.ManifestName, &publishedAtStr); err != nil {
			return nil, err
		}
		m.PublishedAt, _ = time.Parse("2006-01-02T15:04:05.000Z", publishedAtStr)
		manifests = append(manifests, m)
	}
	return manifests, nil
}

// Group represents a record in the 'groups' table.
type Group struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Prompt represents a record in the 'prompts' table.
type Prompt struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// FindGroupByTypeAndName retrieves a group by type and name.
func FindGroupByTypeAndName(db *sql.DB, groupType string, name string) (*Group, error) {
	query := `SELECT id, type, name, created_at, updated_at FROM groups WHERE type = ? AND name = ?`
	var g Group
	err := db.QueryRow(query, groupType, name).Scan(&g.ID, &g.Type, &g.Name, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query group: %w", err)
	}
	return &g, nil
}

// InsertGroup creates a new group record.
func InsertGroup(db *sql.DB, group *Group) (int64, error) {
	query := `INSERT INTO groups (type, name, created_at, updated_at) VALUES (?, ?, ?, ?)`
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	result, err := db.Exec(query, group.Type, group.Name, now, now)
	if err != nil {
		return 0, fmt.Errorf("failed to insert group: %w", err)
	}
	return result.LastInsertId()
}

// FindPromptByTypeAndName retrieves a prompt by type and name.
func FindPromptByTypeAndName(db *sql.DB, promptType string, name string) (*Prompt, error) {
	query := `SELECT id, type, name, created_at, updated_at FROM prompts WHERE type = ? AND name = ?`
	var p Prompt
	err := db.QueryRow(query, promptType, name).Scan(&p.ID, &p.Type, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query prompt: %w", err)
	}
	return &p, nil
}

// InsertPrompt creates a new prompt record.
func InsertPrompt(db *sql.DB, prompt *Prompt) (int64, error) {
	query := `INSERT INTO prompts (type, name, prompt, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	// We use a generic system prompt text for the manifest viewer
	promptText := "You are a helpful assistant for viewing intelligence manifests."
	result, err := db.Exec(query, prompt.Type, prompt.Name, promptText, now, now)
	if err != nil {
		return 0, fmt.Errorf("failed to insert prompt: %w", err)
	}
	return result.LastInsertId()
}

// truncateString truncates a string to a maximum length, appending "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
