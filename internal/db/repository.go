/*
 * Component: Repository Database Operations
 * Block-UUID: 00e98fcc-74a9-4d3d-8ce4-3d839562f78b
 * Parent-UUID: 3d7bb017-e95a-46f0-8c76-9f63c81da7b1
 * Version: 1.2.1
 * Description: Provides shared methods for resolving repository and branch identifiers (GroupID, RefChatID) from the database. v1.2.1: Added debug logging to GetRefChatID to diagnose unexpected 'branch not found' errors.
 * Language: Go
 * Created-at: 2026-05-15T15:31:22.931Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1)
 */


package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gitsense/gsc-cli/pkg/logger" // Import logger
)

// GetGroupID retrieves the group ID for a specific repository (owner/repo).
// This is used as a fallback when the state file is missing or invalid.
func GetGroupID(db *sql.DB, owner, repo string) (int64, error) {
	repoPath := fmt.Sprintf("%s/%s", owner, repo)
	
	var groupID int64
	query := `SELECT id FROM groups WHERE name = ? AND type = 'git-repo' AND deleted = 0`
	
	err := db.QueryRow(query, repoPath).Scan(&groupID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("repository '%s' not found in database", repoPath)
		}
		return 0, fmt.Errorf("database error while looking up repository: %w", err)
	}

	return groupID, nil
}

// GetRefChatID retrieves the chat ID for a specific branch within a group.
func GetRefChatID(db *sql.DB, groupID int64, branch string) (int64, error) {
	var refID int64
	query := `SELECT id FROM chats WHERE type = 'git-ref' AND name = ? AND group_id = ? AND deleted = 0`
	
	logger.Debug("Executing GetRefChatID query", "query", query, "branch", branch, "groupID", groupID)

	err := db.QueryRow(query, branch, groupID).Scan(&refID)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debug("GetRefChatID: branch not found", "branch", branch, "groupID", groupID)
			return 0, fmt.Errorf("branch '%s' not found in database for group %d", branch, groupID)
		}
		logger.Error("GetRefChatID: database error", "error", err, "branch", branch, "groupID", groupID)
		return 0, fmt.Errorf("failed to resolve refChatId: %w", err)
	}

	logger.Debug("GetRefChatID: resolved refID", "refID", refID, "branch", branch, "groupID", groupID)
	return refID, nil
}

// GetRefCommitHash retrieves the commit hash stored in the meta field for a git-ref chat.
// This checks the GitSense Chat Application database, not shadow repositories.
// The meta field contains JSON with the commit hash for the imported branch.
func GetRefCommitHash(db *sql.DB, refChatID int64) (string, error) {
	var meta sql.NullString
	query := `SELECT meta FROM chats WHERE id = ? AND type = 'git-ref' AND deleted = 0`
	
	err := db.QueryRow(query, refChatID).Scan(&meta)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("git-ref chat %d not found in database", refChatID)
		}
		return "", fmt.Errorf("failed to query meta field: %w", err)
	}

	if !meta.Valid {
		return "", fmt.Errorf("meta field is NULL for git-ref chat %d", refChatID)
	}

	// Parse the meta JSON to extract the commit hash
	// The meta field structure is: {"commit": {"hash": "...", "treeHash": "..."}, ...}
	type RefMeta struct {
		Commit struct {
			Hash string `json:"hash"`
		} `json:"commit"`
	}
	var refMeta RefMeta
	if err := json.Unmarshal([]byte(meta.String), &refMeta); err != nil {
		return "", fmt.Errorf("failed to parse meta JSON: %w", err)
	}

	if refMeta.Commit.Hash == "" {
		return "", fmt.Errorf("commit hash not found in meta field for git-ref chat %d", refChatID)
	}

	return refMeta.Commit.Hash, nil
}
