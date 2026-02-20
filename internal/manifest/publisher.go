/**
 * Component: Manifest Publisher Logic
 * Block-UUID: f47f8f8f-d2fe-407a-82e8-b7fef36b1462
 * Parent-UUID: 1ea963da-4ae6-4722-b602-ece5605e0a52
 * Version: 1.0.4
 * Description: Orchestrates the publishing and unpublishing of intelligence manifests. Manages the hierarchical chat structure, file storage in GSC_HOME, and deterministic Markdown regeneration from the published_manifests index.
 * Language: Go
 * Created-at: 2026-02-20T00:43:51.874Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package manifest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ManifestMeta is a helper struct to extract the database name from the manifest JSON.
type ManifestMeta struct {
	ManifestInfo struct {
		Name string `json:"name"`
	} `json:"manifest_info"`
}

// Publish orchestrates the publication of a manifest to the local GitSense Chat app.
func Publish(manifestPath, owner, repo, branch string) error {
	// 1. Environment Validation
	gscHome, err := settings.GetGSCHome(true)
	if err != nil {
		return err
	}

	// 2. Metadata Extraction
	dbName, err := extractDBName(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest metadata: %w", err)
	}

	// 3. Database Connection
	dbPath := settings.GetChatDatabasePath(gscHome)
	logger.Debug("Connecting to GitSense Chat database", "path", dbPath)
	chatDB, err := db.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.CloseDB(chatDB)

	// 3.5. Schema Initialization
	// Ensure the published_manifests table exists before attempting to insert data.
	if err := db.CreatePublishedManifestsTable(chatDB); err != nil {
		return fmt.Errorf("failed to ensure published_manifests table exists: %w", err)
	}

	// 4. Hierarchy Management (Find or Create Chats)
	rootID, ownerID, repoID, err := ensureHierarchy(chatDB, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to establish chat hierarchy (db: %s): %w", dbPath, err)
	}

	// 5. Identity & Persistence
	manifestUUID := uuid.New().String()
	m := &db.PublishedManifest{
		UUID:        manifestUUID,
		Owner:       owner,
		Repo:        repo,
		Branch:      branch,
		Database:    dbName,
		RootChatID:  sql.NullInt64{Int64: rootID, Valid: true},
		OwnerChatID: sql.NullInt64{Int64: ownerID, Valid: true},
		RepoChatID:  sql.NullInt64{Int64: repoID, Valid: true},
	}

	if _, err := db.InsertPublishedManifest(chatDB, m); err != nil {
		return err
	}

	// 6. File Operation
	storageDir := settings.GetManifestStoragePath(gscHome)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	destPath := filepath.Join(storageDir, manifestUUID+".json")
	if err := copyFile(manifestPath, destPath); err != nil {
		return fmt.Errorf("failed to copy manifest to storage: %w", err)
	}

	// 7. UI Synchronization (Regeneration)
	if err := regenerateUI(chatDB, rootID, ownerID, repoID, owner, repo); err != nil {
		return fmt.Errorf("failed to regenerate chat UI: %w", err)
	}

	logger.Success("Manifest published successfully", "uuid", manifestUUID, "repo", owner+"/"+repo)
	return nil
}

// Unpublish removes a manifest from the index and updates the UI.
func Unpublish(remoteID string) error {
	gscHome, err := settings.GetGSCHome(true)
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
	os.Remove(filepath.Join(storageDir, m.UUID+".json"))

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
	rootMD := buildRootMarkdown(rootManifests)
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

func buildRootMarkdown(owners []db.PublishedManifest) string {
	var sb strings.Builder
	sb.WriteString("# Intelligence Manifests\n\nWelcome to the central index for published intelligence layers.\n\n")
	sb.WriteString("## Repository Owners\n\n")
	if len(owners) == 0 {
		sb.WriteString("No repository owners have published manifests yet.\n")
	} else {
		for _, o := range owners {
			sb.WriteString(fmt.Sprintf("*   [%s](#)\n", o.Owner))
		}
	}
	sb.WriteString(generateStandardFooter())
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
			sb.WriteString(fmt.Sprintf("*   [%s](#)\n", r.Repo))
		}
	}
	sb.WriteString(generateStandardFooter())
	return sb.String()
}

func buildRepoMarkdown(owner, repo string, manifests []db.PublishedManifest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s/%s\n\nIntelligence layers for the %s/%s repository.\n\n", owner, repo, owner, repo))

	sb.WriteString("## Published Manifests\n\n")
	if len(manifests) == 0 {
		sb.WriteString("No active intelligence layers are currently published for this repository.\n")
	} else {
		sb.WriteString("| ID | Branch | Database | Published | Download |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
		for _, m := range manifests {
			shortID := m.UUID[:8]
			published := m.PublishedAt.Format("2006-01-02")
			link := fmt.Sprintf("[Download](/--/manifests/%s/%s/%s)", owner, repo, m.UUID)
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s |\n", shortID, m.Branch, m.Database, published, link))
		}
	}

	sb.WriteString(generateStandardFooter())
	return sb.String()
}

func generateStandardFooter() string {
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

## Learn More

Visit [https://github.com/gitsense/gsc-cli](https://github.com/gitsense/gsc-cli) to learn more about GitSense Chat CLI.
`
}

// --- Utilities ---

func extractDBName(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var meta ManifestMeta
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return "", err
	}
	return meta.ManifestInfo.Name, nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
