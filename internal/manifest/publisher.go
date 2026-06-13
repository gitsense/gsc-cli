/**
 * Component: Manifest Publisher Logic
 * Block-UUID: 7bd569fe-5483-45e7-b7ff-babfcab68ea1
 * Parent-UUID: f0a30ed0-79d1-4379-8429-8f430bb9c06b
 * Version: 1.11.0
 * Description: Orchestrates the publishing and unpublishing of intelligence manifests. Updated file storage to organize manifests by owner and repo subdirectories. Added JSON output for web UI integration. Added support for --format flag to switch between human-readable text and JSON output. Added pointer file creation for clean URL support (database_name.json points to latest manifest).
 * Language: Go
 * Created-at: 2026-05-10T01:08:10.254Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0)
 */


package manifest

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ManifestJSON represents the full structure of a manifest file for unmarshaling.
type ManifestJSON struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	Manifest      struct {
		Name        string   `json:"name"`
		DatabaseName string  `json:"database_name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	} `json:"manifest"`
	Repositories []struct {
		Ref  string `json:"ref"`
		Name string `json:"name"`
	} `json:"repositories"`
	Branches []struct {
		Ref  string `json:"ref"`
		Name string `json:"name"`
	} `json:"branches"`
}

// Publish orchestrates the publication of a manifest to the local GitSense Chat app.
func Publish(manifestPath, owner, repo, branch, format string) error {
	isJSON := format == "json"

	// Helper to print JSON error to stderr if in JSON mode
	printJSONError := func(err error) {
		if isJSON {
			errorOutput := map[string]string{
				"status": "error",
				"error":  err.Error(),
			}
			jsonBytes, _ := json.Marshal(errorOutput)
			fmt.Fprintln(os.Stderr, string(jsonBytes))
		}
	}

	// 1. Environment Validation
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		printJSONError(err)
		return err
	}

	// 2. Metadata Extraction & Hash Calculation
	fileBytes, manifestData, hash, err := extractManifestProperties(manifestPath, owner, repo, branch)
	if err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to read manifest metadata: %w", err)
	}

	// Validate that we actually found a database name
	if manifestData.Manifest.DatabaseName == "" {
		err := fmt.Errorf("manifest metadata is invalid: 'manifest.database_name' is missing or empty in %s", manifestPath)
		printJSONError(err)
		return err
	}

	// 3. Database Connection
	dbPath := settings.GetChatDatabasePath(gscHome)
	if !isJSON {
		logger.Debug("Connecting to GitSense Chat database", "path", dbPath)
	}
	chatDB, err := db.OpenDB(dbPath)
	if err != nil {
		printJSONError(err)
		return err
	}
	defer db.CloseDB(chatDB)

	// 3.5. Schema Initialization
	// Ensure the published_manifests table exists before attempting to insert data.
	if err := db.CreatePublishedManifestsTable(chatDB); err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to ensure published_manifests table exists: %w", err)
	}

	// 4. Duplicate Detection (The "Bump" Logic)
	existing, err := db.FindManifestByHash(chatDB, hash)
	if err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to check for existing manifest: %w", err)
	}

	if existing != nil {
		// BUMP: Manifest exists, just update the timestamp and regenerate UI
		if !isJSON {
			logger.Info("Manifest already exists. Bumping timestamp...", "uuid", existing.UUID)
		}
		
		if err := db.UpdateManifestTimestamp(chatDB, existing.ID); err != nil {
			printJSONError(err)
			return fmt.Errorf("failed to bump manifest timestamp: %w", err)
		}

		// Regenerate UI for the existing hierarchy
		if err := regenerateUI(chatDB, existing.RootChatID.Int64, existing.OwnerChatID.Int64, existing.RepoChatID.Int64, existing.Owner, existing.Repo); err != nil {
			printJSONError(err)
			return fmt.Errorf("failed to regenerate chat UI after bump: %w", err)
		}

		// Output JSON result for Web UI (Bump Scenario)
		var repoUUID string
		err = chatDB.QueryRow("SELECT uuid FROM chats WHERE id = ?", existing.RepoChatID.Int64).Scan(&repoUUID)
		if err != nil {
			printJSONError(err)
			return fmt.Errorf("failed to retrieve repo chat UUID: %w", err)
		}

		if isJSON {
			resultJSON := fmt.Sprintf(`{"status":"success","repoUUID":"%s","manifestUUID":"%s"}`, repoUUID, existing.UUID)
			fmt.Println(resultJSON)
		} else {
			fmt.Println("Manifest bumped successfully", "uuid", existing.UUID, "repo", owner+"/"+repo)
		}
		return nil
	}

	// 5. Hierarchy Management (Find or Create Chats)
	rootID, ownerID, repoID, err := ensureHierarchy(chatDB, owner, repo)
	if err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to establish chat hierarchy (db: %s): %w", dbPath, err)
	}

	// 6. Identity & Persistence
	manifestUUID := uuid.New().String()
	
	// Prepare JSON fields for storage
	tagsJSON, _ := json.Marshal(manifestData.Manifest.Tags)
	reposJSON, _ := json.Marshal(manifestData.Repositories)
	branchesJSON, _ := json.Marshal(manifestData.Branches)
	
	// Parse generated_at
	var generatedAt time.Time
	if manifestData.GeneratedAt != "" {
		generatedAt, _ = time.Parse(time.RFC3339, manifestData.GeneratedAt)
	}

	m := &db.PublishedManifest{
		UUID:                manifestUUID,
		Owner:               owner,
		Repo:                repo,
		Branch:              branch,
		Database:            manifestData.Manifest.DatabaseName,
		SchemaVersion:       manifestData.SchemaVersion,
		GeneratedAt:         generatedAt,
		ManifestName:        manifestData.Manifest.Name,
		ManifestDescription: manifestData.Manifest.Description,
		ManifestTags:        string(tagsJSON),
		Repositories:        string(reposJSON),
		Branches:            string(branchesJSON),
		Hash:                hash,
		RootChatID:          sql.NullInt64{Int64: rootID, Valid: true},
		OwnerChatID:         sql.NullInt64{Int64: ownerID, Valid: true},
		RepoChatID:          sql.NullInt64{Int64: repoID, Valid: true},
	}

	if _, err := db.InsertPublishedManifest(chatDB, m); err != nil {
		printJSONError(err)
		return err
	}

	// 7. File Operation
	storageDir := settings.GetManifestStoragePath(gscHome)
	
	// Create owner/repo subdirectory structure
	ownerRepoDir := filepath.Join(storageDir, owner, repo)
	if err := os.MkdirAll(ownerRepoDir, 0755); err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to create owner/repo directory: %w", err)
	}

	destPath := filepath.Join(ownerRepoDir, manifestUUID+".json")
	if err := os.WriteFile(destPath, fileBytes, 0644); err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to write manifest to storage: %w", err)
	}

	// 7.5. Create/update pointer file for clean URLs
	// This allows users to import manifests using clean URLs like:
	// gsc manifest import https://chat.gitsense.com/--/manifests/BurntSushi/ripgrep/code-intent
	pointerFilename := manifestData.Manifest.DatabaseName + ".json"
	pointerPath := filepath.Join(ownerRepoDir, pointerFilename)

	pointerData := map[string]interface{}{
		"manifest_uuid":  manifestUUID,
		"manifest_name":  manifestData.Manifest.Name,
		"database_name":  manifestData.Manifest.DatabaseName,
		"published_at":   time.Now().Format(time.RFC3339),
	}

	pointerBytes, err := json.MarshalIndent(pointerData, "", "  ")
	if err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to marshal pointer file: %w", err)
	}

	if err := os.WriteFile(pointerPath, pointerBytes, 0644); err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to write pointer file: %w", err)
	}

	// 8. UI Synchronization (Regeneration)
	if err := regenerateUI(chatDB, rootID, ownerID, repoID, owner, repo); err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to regenerate chat UI: %w", err)
	}

	// 9. Output Result for Web UI
	// Fetch the UUID of the repo chat so the web UI can link to it
	var repoUUID string
	err = chatDB.QueryRow("SELECT uuid FROM chats WHERE id = ?", repoID).Scan(&repoUUID)
	if err != nil {
		printJSONError(err)
		return fmt.Errorf("failed to retrieve repo chat UUID: %w", err)
	}

	if isJSON {
		// Output JSON result to stdout for the Node wrapper to parse
		resultJSON := fmt.Sprintf(`{"status":"success","repoUUID":"%s","manifestUUID":"%s"}`, repoUUID, manifestUUID)
		fmt.Println(resultJSON)
	} else {
		logger.Success("Manifest published successfully", "uuid", manifestUUID, "repo", owner+"/"+repo)
	}
	
	return nil
}

// DeleteByOwnerRepo unpublishes all manifests for a specific owner/repo combination.
// ManifestGroupSummary holds a manifest name and the number of versions published under it.
type ManifestGroupSummary struct {
	Name  string
	Count int
}

// RepoDeleteSummary holds a repo name and the number of manifests published in it.
type RepoDeleteSummary struct {
	Repo  string
	Count int
}

// SummarizeOwnerRepoDelete returns the total count and per-name breakdown for an owner/repo delete.
func SummarizeOwnerRepoDelete(owner, repo string) (int, []ManifestGroupSummary, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return 0, nil, err
	}
	chatDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return 0, nil, err
	}
	defer db.CloseDB(chatDB)

	manifests, err := db.GetActiveManifests(chatDB, owner, repo)
	if err != nil {
		return 0, nil, err
	}

	counts := map[string]int{}
	var order []string
	for _, m := range manifests {
		if _, seen := counts[m.ManifestName]; !seen {
			order = append(order, m.ManifestName)
		}
		counts[m.ManifestName]++
	}

	groups := make([]ManifestGroupSummary, 0, len(order))
	total := 0
	for _, name := range order {
		c := counts[name]
		groups = append(groups, ManifestGroupSummary{Name: name, Count: c})
		total += c
	}
	return total, groups, nil
}

// SummarizeOwnerDelete returns the total count and per-repo breakdown for an owner-wide delete.
func SummarizeOwnerDelete(owner string) (int, []RepoDeleteSummary, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return 0, nil, err
	}
	chatDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return 0, nil, err
	}
	defer db.CloseDB(chatDB)

	repos, err := db.GetActiveManifests(chatDB, owner, "")
	if err != nil {
		return 0, nil, err
	}

	summaries := make([]RepoDeleteSummary, 0, len(repos))
	total := 0
	for _, r := range repos {
		summaries = append(summaries, RepoDeleteSummary{Repo: r.Repo, Count: r.ManifestCount})
		total += r.ManifestCount
	}
	return total, summaries, nil
}

func DeleteByOwnerRepo(owner, repo string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return err
	}

	dbPath := settings.GetChatDatabasePath(gscHome)
	chatDB, err := db.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.CloseDB(chatDB)

	count, err := db.DeletePublishedManifestsByOwnerRepo(chatDB, owner, repo)
	if err != nil {
		return err
	}

	rootChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-root", "Intelligence Manifests", 0)
	if err != nil {
		return fmt.Errorf("failed to find root chat: %w", err)
	}
	if rootChat != nil {
		ownerChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-owner", owner, rootChat.ID)
		if err != nil {
			return fmt.Errorf("failed to find owner chat: %w", err)
		}
		if ownerChat != nil {
			repoChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-repo", repo, ownerChat.ID)
			if err != nil {
				return fmt.Errorf("failed to find repo chat: %w", err)
			}
			if repoChat != nil {
				if err := db.DeleteChatAndDescendants(chatDB, repoChat.ID); err != nil {
					return fmt.Errorf("failed to delete repo chat hierarchy: %w", err)
				}
			}
		}
	}

	deleteStorageDir(filepath.Join(settings.GetManifestStoragePath(gscHome), owner, repo))

	if err := regenerateAfterRepoDelete(chatDB, owner); err != nil {
		return fmt.Errorf("failed to regenerate UI: %w", err)
	}

	logger.Success("Manifests unpublished", "owner", owner, "repo", repo, "count", count)
	return nil
}

// DeleteByOwner unpublishes all manifests for an owner (all repos).
func DeleteByOwner(owner string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return err
	}

	dbPath := settings.GetChatDatabasePath(gscHome)
	chatDB, err := db.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.CloseDB(chatDB)

	count, err := db.DeletePublishedManifestsByOwner(chatDB, owner)
	if err != nil {
		return err
	}

	rootChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-root", "Intelligence Manifests", 0)
	if err != nil {
		return fmt.Errorf("failed to find root chat: %w", err)
	}
	if rootChat != nil {
		ownerChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-owner", owner, rootChat.ID)
		if err != nil {
			return fmt.Errorf("failed to find owner chat: %w", err)
		}
		if ownerChat != nil {
			if err := db.DeleteChatAndDescendants(chatDB, ownerChat.ID); err != nil {
				return fmt.Errorf("failed to delete owner chat hierarchy: %w", err)
			}
		}
	}

	deleteStorageDir(filepath.Join(settings.GetManifestStoragePath(gscHome), owner))

	if err := regenerateAfterOwnerDelete(chatDB); err != nil {
		return fmt.Errorf("failed to regenerate root UI: %w", err)
	}

	logger.Success("Manifests unpublished", "owner", owner, "count", count)
	return nil
}

// deleteStorageDir removes a directory and all its contents, ignoring not-found errors.
func deleteStorageDir(path string) {
	if err := os.RemoveAll(path); err != nil {
		logger.Warning("Failed to delete storage directory", "path", path, "error", err)
	}
}

// regenerateAfterRepoDelete regenerates the owner and root UIs after a repo has been deleted.
func regenerateAfterRepoDelete(chatDB *sql.DB, owner string) error {
	rootChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-root", "Intelligence Manifests", 0)
	if err != nil || rootChat == nil {
		return nil
	}

	ownerChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-owner", owner, rootChat.ID)
	if err != nil || ownerChat == nil {
		return nil
	}

	ownerManifests, _ := db.GetActiveManifests(chatDB, owner, "")
	ownerMD := buildOwnerMarkdown(owner, ownerManifests)
	if err := ensureMessages(chatDB, ownerChat.ID, owner, ownerMD); err != nil {
		return err
	}
	rootManifests, _ := db.GetActiveManifests(chatDB, "", "")
	recentManifests, _ := db.GetGlobalRecentManifests(chatDB, 5)
	rootMD := buildRootMarkdown(rootManifests, recentManifests)
	return ensureMessages(chatDB, rootChat.ID, "Intelligence Manifests", rootMD)
}

// regenerateAfterOwnerDelete regenerates the root UI after an entire owner has been deleted.
func regenerateAfterOwnerDelete(chatDB *sql.DB) error {
	rootChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-root", "Intelligence Manifests", 0)
	if err != nil || rootChat == nil {
		return nil
	}

	rootManifests, _ := db.GetActiveManifests(chatDB, "", "")
	recentManifests, _ := db.GetGlobalRecentManifests(chatDB, 5)
	rootMD := buildRootMarkdown(rootManifests, recentManifests)
	return ensureMessages(chatDB, rootChat.ID, "Intelligence Manifests", rootMD)
}

// Unpublish removes a manifest from the index and updates the UI.
func Unpublish(remoteID string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return err
	}

	dbPath := settings.GetChatDatabasePath(gscHome)
	chatDB, err := db.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.CloseDB(chatDB)

	// 1. Lookup manifest to get chat IDs for regeneration
	// Note: We use a LIKE query to support short IDs
	query := `SELECT uuid, root_chat_id, owner_chat_id, repo_chat_id, owner, repo FROM published_manifests WHERE uuid LIKE ? AND deleted = 0`
	var m db.PublishedManifest
	err = chatDB.QueryRow(query, remoteID+"%").Scan(&m.UUID, &m.RootChatID, &m.OwnerChatID, &m.RepoChatID, &m.Owner, &m.Repo)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("manifest with ID %s not found", remoteID)
		}
		return err
	}

	// 2. Soft Delete in DB
	if err := db.DeletePublishedManifest(chatDB, m.UUID); err != nil {
		return err
	}

	// 3. Delete File
	storageDir := settings.GetManifestStoragePath(gscHome)
	
	// Construct path: data/storage/manifests/owner/repo/uuid.json
	manifestPath := filepath.Join(storageDir, m.Owner, m.Repo, m.UUID+".json")
	os.Remove(manifestPath)

	// 4. UI Synchronization
	if err := regenerateUI(chatDB, m.RootChatID.Int64, m.OwnerChatID.Int64, m.RepoChatID.Int64, m.Owner, m.Repo); err != nil {
		return err
	}

	logger.Success("Manifest unpublished successfully", "uuid", m.UUID)
	return nil
}

// ensureHierarchy ensures the Root, Owner, and Repo chats exist.
func ensureHierarchy(chatDB *sql.DB, owner, repo string) (int64, int64, int64, error) {
	// Root Level
	root, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-root", "Intelligence Manifests", 0)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to find root chat: %w", err)
	}
	var rootID int64
	if root == nil {
		// Ensure we have a valid Group and Prompt before creating the chat
		groupID, err := getOrCreateDefaultGroup(chatDB)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get default group: %w", err)
		}
		promptID, err := getOrCreateDefaultPrompt(chatDB)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get default prompt: %w", err)
		}

		rootID, err = db.InsertChat(chatDB, &db.Chat{
			Type:       "intelligence-manifests-root",
			Name:       "Intelligence Manifests",
			Visibility: "public",
			MainModel:  settings.RealModelNotes,
			ParentID:   0,
			GroupID:    groupID,
			PromptID:   promptID,
		})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to insert root chat: %w", err)
		}
	} else {
		rootID = root.ID
	}

	// Owner Level
	ownerChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-owner", owner, rootID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to find owner chat: %w", err)
	}
	var ownerID int64
	if ownerChat == nil {
		// Reuse the same group and prompt for the hierarchy
		groupID, err := getOrCreateDefaultGroup(chatDB)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get default group: %w", err)
		}
		promptID, err := getOrCreateDefaultPrompt(chatDB)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get default prompt: %w", err)
		}

		ownerID, err = db.InsertChat(chatDB, &db.Chat{
			Type:       "intelligence-manifests-owner",
			Name:       owner,
			ParentID:   rootID,
			Visibility: "public",
			MainModel:  settings.RealModelNotes,
			GroupID:    groupID,
			PromptID:   promptID,
		})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to insert owner chat: %w", err)
		}
	} else {
		ownerID = ownerChat.ID
	}

	// Repo Level
	repoChat, err := db.FindChatByTypeAndName(chatDB, "intelligence-manifests-repo", repo, ownerID)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to find repo chat: %w", err)
	}
	var repoID int64
	if repoChat == nil {
		// Reuse the same group and prompt for the hierarchy
		groupID, err := getOrCreateDefaultGroup(chatDB)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get default group: %w", err)
		}
		promptID, err := getOrCreateDefaultPrompt(chatDB)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to get default prompt: %w", err)
		}

		repoID, err = db.InsertChat(chatDB, &db.Chat{
			Type:       "intelligence-manifests-repo",
			Name:       repo,
			ParentID:   ownerID,
			Visibility: "public",
			MainModel:  settings.RealModelNotes,
			GroupID:    groupID,
			PromptID:   promptID,
		})
		if err != nil {
			return 0, 0, 0, fmt.Errorf("failed to insert repo chat: %w", err)
		}
	} else {
		repoID = repoChat.ID
	}

	return rootID, ownerID, repoID, nil
}

// getOrCreateDefaultGroup ensures a default group exists for manifest chats.
func getOrCreateDefaultGroup(chatDB *sql.DB) (int64, error) {
	group, err := db.FindGroupByTypeAndName(chatDB, "regular", "Intelligence Manifests")
	if err != nil {
		return 0, err
	}
	if group != nil {
		return group.ID, nil
	}
	return db.InsertGroup(chatDB, &db.Group{
		Type: "regular",
		Name: "Intelligence Manifests",
	})
}

// getOrCreateDefaultPrompt ensures a default prompt exists for manifest chats.
func getOrCreateDefaultPrompt(chatDB *sql.DB) (int64, error) {
	prompt, err := db.FindPromptByTypeAndName(chatDB, "system", "Manifest Viewer")
	if err != nil {
		return 0, err
	}
	if prompt != nil {
		return prompt.ID, nil
	}
	return db.InsertPrompt(chatDB, &db.Prompt{
		Type: "system",
		Name: "Manifest Viewer",
	})
}

// regenerateUI rebuilds the Markdown for all three levels of the hierarchy.
func regenerateUI(chatDB *sql.DB, rootID, ownerID, repoID int64, owner, repo string) error {
	// 1. Repo Level
	repoManifests, _ := db.GetActiveManifests(chatDB, owner, repo)
	repoMD := buildRepoMarkdown(owner, repo, repoManifests)
	if err := ensureMessages(chatDB, repoID, owner+"/"+repo, repoMD); err != nil {
		return err
	}

	// 2. Owner Level
	ownerManifests, _ := db.GetActiveManifests(chatDB, owner, "")
	ownerMD := buildOwnerMarkdown(owner, ownerManifests)
	if err := ensureMessages(chatDB, ownerID, owner, ownerMD); err != nil {
		return err
	}

	// 3. Root Level
	rootManifests, _ := db.GetActiveManifests(chatDB, "", "")
	recentManifests, _ := db.GetGlobalRecentManifests(chatDB, 5)
	rootMD := buildRootMarkdown(rootManifests, recentManifests)
	if err := ensureMessages(chatDB, rootID, "Intelligence Manifests", rootMD); err != nil {
		return err
	}

	return nil
}

// ensureMessages upserts the System/Assistant message pair for a chat.
func ensureMessages(chatDB *sql.DB, chatID int64, contextName, content string) error {
	// 1. Ensure System Message (The Anchor)
	sysMsg, err := db.FindMessageByRoleAndType(chatDB, chatID, "system", "")
	if err != nil {
		return err
	}
	var sysID int64
	if sysMsg == nil {
		sysID, err = db.InsertMessage(chatDB, &db.Message{
			ChatID:     chatID,
			Role:       "system",
			ParentID:   0,
			Level:      0,
			Visibility: "public",
			Message:    sql.NullString{String: fmt.Sprintf("You are the intelligence manifest viewer for %s.", contextName), Valid: true},
		})
	} else {
		sysID = sysMsg.ID
	}

	// 2. Upsert Assistant Message (The View)
	astMsg, err := db.FindMessageByRoleAndType(chatDB, chatID, "assistant", "intelligence-manifest")
	if err != nil {
		return err
	}

	if astMsg == nil {
		_, err = db.InsertMessage(chatDB, &db.Message{
			ChatID:     chatID,
			Role:       "assistant",
			Type:       "intelligence-manifest",
			ParentID:   sysID,
			Level:      1,
			Visibility: "public",
			Temperature: sql.NullFloat64{Float64: 0.0, Valid: true},
			Message:    sql.NullString{String: content, Valid: true},
		})
	} else {
		err = db.UpdateMessage(chatDB, astMsg.ID, content)
	}

	return err
}

// --- Markdown Builders ---

func buildRootMarkdown(owners []db.PublishedManifest, recent []db.PublishedManifest) string {
	var sb strings.Builder
	sb.WriteString("# Intelligence Manifests\n\nWelcome to the central index for published intelligence layers.\n\n")
	
	//// Recently Published Section
	//sb.WriteString(generateRecentlyPublishedTable(recent))
	//sb.WriteString("\n")

	sb.WriteString("## Owners\n\n")
	if len(owners) == 0 {
		sb.WriteString("No repository owners have published manifests yet.\n")
	} else {
		for _, o := range owners {
			sb.WriteString(fmt.Sprintf("*   [%s (%d)](#)\n", o.Owner, o.ManifestCount))
		}
	}
	
	sb.WriteString(generateLearnMoreSection())
	return sb.String()
}

func buildOwnerMarkdown(owner string, repos []db.PublishedManifest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\nIntelligence layers for repositories owned by %s.\n\n", owner, owner))
	sb.WriteString("## Repositories\n\n")
	if len(repos) == 0 {
		sb.WriteString("No repositories for this owner have published manifests yet.\n")
	} else {
		for _, r := range repos {
			sb.WriteString(fmt.Sprintf("*   [%s](#) (%d)\n", r.Repo, r.ManifestCount))
		}
	}
	
	sb.WriteString(generateLearnMoreSection())
	return sb.String()
}

func buildRepoMarkdown(owner, repo string, manifests []db.PublishedManifest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s / %s\n\nIntelligence layers for the %s/%s repository.\n\n", owner, repo, owner, repo))

	if len(manifests) == 0 {
		sb.WriteString("No active intelligence layers are currently published for this repository.\n")
	} else {
		type manifestGroup struct {
			name        string
			database    string
			description string
			entries     []db.PublishedManifest
		}
		var groupOrder []string
		groups := map[string]*manifestGroup{}

		for _, m := range manifests {
			key := m.ManifestName + "\x00" + m.Database
			if _, exists := groups[key]; !exists {
				groupOrder = append(groupOrder, key)
				groups[key] = &manifestGroup{
					name:        m.ManifestName,
					database:    m.Database,
					description: m.ManifestDescription,
				}
			}
			groups[key].entries = append(groups[key].entries, m)
		}

		for i, key := range groupOrder {
			g := groups[key]
			sb.WriteString(fmt.Sprintf("## %s (%s)\n\n", g.name, g.database))
			if g.description != "" {
				sb.WriteString(g.description + "\n\n")
			}
			sb.WriteString("| ID | Branch | Published | Download | URL |\n")
			sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
			for _, m := range g.entries {
				shortID := m.UUID[:8]
				published := m.PublishedAt.Format("2006-01-02 15:04:05")
				link := fmt.Sprintf("[Download](/--/manifests/%s/%s/%s.json)", owner, repo, m.UUID)
				copyLink := "[Copy](#)"
				sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n", shortID, m.Branch, published, link, copyLink))
			}
			if i < len(groupOrder)-1 {
				sb.WriteString("\n---\n\n")
			}
		}
	}

	sb.WriteString(generateHowToUseSection())
	sb.WriteString(generateLearnMoreSection())
	return sb.String()
}

func generateHowToUseSection() string {
	return `

## How to Use

To add this intelligence layer to your repository:

1. Download the manifest file using the link above.
2. Navigate to your local repository.
3. Switch to the branch that matches the manifest (e.g., 'git checkout main').
4. Run the following command:

` + "```bash" + `
gsc manifest import <path-to-manifest-file>
` + "```" + `
`
}

func generateLearnMoreSection() string {
	return `

## Learn More

Visit [https://github.com/gitsense/gsc-cli](https://github.com/gitsense/gsc-cli) to learn more about GitSense Chat CLI and the intelligence layer.
`
}

func generateRecentlyPublishedTable(manifests []db.PublishedManifest) string {
	if len(manifests) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Recently Published\n\n")
	sb.WriteString("| Repository | Manifest | Published |\n")
	sb.WriteString("| :--- | :--- | :--- |\n")
	for _, m := range manifests {
		published := m.PublishedAt.Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("| %s/%s | %s | %s |\n", m.Owner, m.Repo, m.ManifestName, published))
	}
	return sb.String()
}

// --- Utilities ---

// extractManifestProperties reads the manifest file, calculates the hash, and extracts metadata.
func extractManifestProperties(path, owner, repo, branch string) ([]byte, *ManifestJSON, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	defer file.Close()

	// Read file content for hashing
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, "", err
	}

	// Calculate Hash: SHA256(file_content + owner + repo + branch)
	hashInput := string(fileBytes) + owner + repo + branch
	hashBytes := sha256.Sum256([]byte(hashInput))
	hash := hex.EncodeToString(hashBytes[:])

	// Unmarshal JSON to extract properties
	var data ManifestJSON
	if err := json.Unmarshal(fileBytes, &data); err != nil {
		return nil, nil, "", err
	}

	return fileBytes, &data, hash, nil
}
