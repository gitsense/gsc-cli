/**
 * Component: Contract Dump Orchestrator
 * Block-UUID: a581ceef-fb80-49fc-a379-47f2e74cc356
 * Parent-UUID: 184e6b17-4620-4e07-9c6a-a9dcd749dda4
 * Version: 2.24.0
 * Description: Implemented Registry-First workspace strategy. Calculates composite hash (ContractUUID + MessageHash) for unique workspace IDs and updates the ContractMetadata JSON registry.
 * Language: Go
 * Created-at: 2026-03-10T00:43:35.105Z
 * Authors: GLM-4.7 (v2.23.0), GLM-4.7 (v2.24.0)
 */


package contract

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ExecuteDump coordinates the full dump process for a given contract.
// It supports 'tree', 'merged', and 'mapped' strategies.
func ExecuteDump(contractUUID string, writer DumpWriter, outputDir string, includeSystem bool, trim bool, dumpType string, sortMode string, debugPatch bool, messageID int64, validate bool, activeChatID int64) (*MappedDumpResult, error) {
	// 1. Initialize Output (Skip if validating to avoid deleting the workspace)
	if !validate {
		if err := writer.Prepare(outputDir); err != nil {
			return nil, fmt.Errorf("failed to prepare output directory: %w", err)
		}
	}

	// 2. Open Database
	gscHome, _ := settings.GetGSCHome(false)
	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return nil, err
	}
	defer sqliteDB.Close()

	// 3. Find all chats associated with this contract
	query := `
		SELECT 
			id, uuid, name, type 
		FROM 
			chats 
		WHERE id IN (
			SELECT chat_id FROM messages WHERE type = 'gsc-cli-contract' AND json_extract(meta, '$.contract_uuid') = ? AND deleted = 0
		)`
		
	rows, err := sqliteDB.Query(query, contractUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chats for contract: %w", err)
	}
	defer rows.Close()

	var chats []db.Chat
	for rows.Next() {
		var c db.Chat
		if err := rows.Scan(&c.ID, &c.UUID, &c.Name, &c.Type); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}

	if len(chats) == 0 {
		return nil, fmt.Errorf("no chats found for contract %s", contractUUID)
	}

	// ==========================================
	// STRATEGY SELECTION
	// ==========================================
	if dumpType == "mapped" {
		return executeMappedDump(contractUUID, chats, sqliteDB, writer, outputDir, includeSystem, trim, debugPatch, messageID, validate, activeChatID)
	}

	if dumpType == "merged" {
		// "text" in UI usually means the merged/squashed view
		if dumpType == "text" {
			dumpType = "merged"
		}

		// Default sort mode to recency if not specified
		if sortMode == "" {
			sortMode = settings.SortRecency
		}

		// Merged dump doesn't return MappedDumpResult
		err := executeMergedDump(chats, sqliteDB, writer, outputDir, includeSystem, trim, sortMode, make(map[string]string), debugPatch)
		return nil, err
	}

	// ==========================================
	// LEGACY 'TREE' STRATEGY
	// ==========================================
	return nil, executeTreeDump(chats, sqliteDB, writer, outputDir, includeSystem, trim, debugPatch)
}
