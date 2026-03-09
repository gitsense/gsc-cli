/*
 * Component: Workspace Message Command
 * Block-UUID: 1038bb06-f215-48c1-93bf-11af64c19e94
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc ws message' command to display the current message from the database and compare it with the workspace snapshot.
 * Language: Go
 * Created-at: 2026-03-09T15:50:38.435Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package ws

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

// messageCmd represents the 'gsc ws message' command
var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Display the current message from the chat",
	Long: `Displays the content of the message associated with this workspace from the database.
It also checks if the workspace snapshot is stale compared to the live chat.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleMessage()
	},
}

func init() {
	wsCmd.AddCommand(messageCmd)
}

func handleMessage() error {
	// 1. Read Workspace Manifest
	workspacePath := "workspace.json"
	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace.json not found. Are you in a GitSense Chat workspace?")
	}

	data, err := os.ReadFile(workspacePath)
	if err != nil {
		return fmt.Errorf("failed to read workspace.json: %w", err)
	}

	var ws contract.ShadowWorkspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return fmt.Errorf("failed to parse workspace.json: %w", err)
	}

	// 2. Open Database
	gscHome, _ := settings.GetGSCHome(false)
	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return err
	}
	defer sqliteDB.Close()

	// 3. Fetch Message from DB
	msg, err := db.GetMessage(sqliteDB, ws.MessageID)
	if err != nil {
		return fmt.Errorf("failed to fetch message %d from database: %w", ws.MessageID, err)
	}

	if !msg.Message.Valid {
		return fmt.Errorf("message content is empty in database")
	}

	// 4. Calculate Hash of DB Message
	dbHash := calculateMessageHash(*msg)

	// 5. Compare with Workspace Hash
	// Note: ws.Hash is the Workspace ID (Composite Hash), not the Message Hash.
	// We need to compare the content. 
	// The workspace.json stores the MessageID. We can check if the message content matches.
	// However, the user wants to know if the workspace *reflects* the message.
	// Since the workspace is a dump, we can't easily re-hash the dump without re-reading all files.
	// A simpler proxy: Check if the message content in DB matches the message.md in the workspace (if it exists).
	
	// Read local message.md if it exists
	localMsgPath := filepath.Join(filepath.Dir(workspacePath), "message.md")
	localHash := ""
	if _, err := os.Stat(localMsgPath); err == nil {
		localContent, _ := os.ReadFile(localMsgPath)
		localHash = calculateStringHash(string(localContent))
	}

	// 6. Display
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Message ID: %d\n", ws.MessageID)
	fmt.Printf("Role: %s\n", msg.Role)
	fmt.Printf("Created At: %s\n", msg.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println("--------------------------------------------------")
	
	if localHash != "" && localHash != dbHash {
		fmt.Println("WARNING: Workspace message.md is stale compared to the database.")
		fmt.Println("The user may have updated the message since this workspace was created.")
		fmt.Println("--------------------------------------------------")
	}

	fmt.Println(msg.Message.String)
	fmt.Println("--------------------------------------------------")

	return nil
}

// calculateMessageHash generates a hash for a db.Message
func calculateMessageHash(msg db.Message) string {
	return calculateStringHash(msg.Message.String)
}

// calculateStringHash generates a SHA256 hash of a string
func calculateStringHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))[:8]
}
