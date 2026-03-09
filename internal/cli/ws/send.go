/*
 * Component: Workspace Send Command
 * Block-UUID: f4ff7005-b6dd-4609-81f4-d022d19cd041
 * Parent-UUID: 2f9978b9-6ccc-43f5-b0f2-8356ec9d644e
 * Version: 1.3.0
 * Description: Added support for message manipulation operations (replace, insert before, insert after) using workspace context and updated payload structure.
 * Language: Go
 * Created-at: 2026-03-07T04:45:31.000Z
 * Authors: Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.1.1), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

var (
	sendFile          string
	sendMdBefore      string
	sendMdAfter       string
	sendWrap          string
	sendVisibility    string
	sendForce         bool
	sendNoConfirmation bool
	
	// New flags for message manipulation
	sendReplace      bool
	sendInsertBefore bool
	sendInsertAfter  bool
)

// sendCmd represents the 'gsc ws send' command
var sendCmd = &cobra.Command{
	Use:   "send [text]",
	Short: "Send a message from the terminal to the chat",
	Long: `Sends a message to the active chat session via the contract events database.
Supports piping, file input, markdown formatting, and message manipulation operations.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleSend(args)
	},
}

func init() {
	sendCmd.Flags().StringVar(&sendFile, "file", "", "Read content from a file")
	sendCmd.Flags().StringVar(&sendMdBefore, "md-before", "", "Prepend Markdown text")
	sendCmd.Flags().StringVar(&sendMdAfter, "md-after", "", "Append Markdown text")
	sendCmd.Flags().StringVar(&sendWrap, "wrap", "", "Wrap output in a code block (e.g., 'bash', 'python')")
	sendCmd.Flags().StringVar(&sendVisibility, "visibility", "human-public", "Message visibility: 'human-public' or 'human-only'")
	sendCmd.Flags().BoolVar(&sendForce, "force", false, "Skip confirmation for large files")
	sendCmd.Flags().BoolVar(&sendNoConfirmation, "no-chat-confirmation", false, "Bypass the UI confirmation modal")

	// Register new manipulation flags
	sendCmd.Flags().BoolVar(&sendReplace, "replace-message", false, "Replace the current workspace message")
	sendCmd.Flags().BoolVar(&sendInsertBefore, "insert-before-message", false, "Insert before the current workspace message")
	sendCmd.Flags().BoolVar(&sendInsertAfter, "insert-after-message", false, "Insert after the current workspace message")
}

func handleSend(args []string) error {
	// 1. Context Validation
	contractUUID := os.Getenv("GSC_CONTRACT_UUID")
	if contractUUID == "" {
		return fmt.Errorf("not in a GitSense workspace. GSC_CONTRACT_UUID environment variable not set.")
	}

	chatIDStr := os.Getenv("GSC_CHAT_ID")
	if chatIDStr == "" {
		return fmt.Errorf("not in a GitSense workspace. GSC_CHAT_ID environment variable not set.")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid GSC_CHAT_ID environment variable: %w", err)
	}

	// 2. Flag Validation (Mutual Exclusion)
	operationFlags := []bool{sendReplace, sendInsertBefore, sendInsertAfter}
	activeFlags := 0
	for _, flag := range operationFlags {
		if flag {
			activeFlags++
		}
	}
	if activeFlags > 1 {
		return fmt.Errorf("only one message manipulation flag can be used at a time")
	}

	// 3. Context Resolution (Reference Message ID)
	var referenceMessageID int64
	if activeFlags > 0 {
		// We need to read workspace.json to find the reference message ID
		workspacePath := "workspace.json"
		if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
			return fmt.Errorf("workspace.json not found. Cannot determine reference message ID for operation.")
		}

		data, err := os.ReadFile(workspacePath)
		if err != nil {
			return fmt.Errorf("failed to read workspace.json: %w", err)
		}

		var ws contract.ShadowWorkspace
		if err := json.Unmarshal(data, &ws); err != nil {
			return fmt.Errorf("failed to parse workspace.json: %w", err)
		}

		referenceMessageID = ws.MessageID
	}

	// 4. Input Resolution
	var content string

	// Check for Pipe
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		content = string(data)
	} else if sendFile != "" {
		// Read from File
		content, err = readFileContent(sendFile)
		if err != nil {
			return err
		}
	} else if len(args) > 0 {
		// Read from Argument
		content = args[0]
	} else {
		return fmt.Errorf("no input provided. Use pipe, --file, or provide text argument.")
	}

	// Conflict Check: Pipe and File
	if (stat.Mode() & os.ModeCharDevice) == 0 && sendFile != "" {
		return fmt.Errorf("cannot use both pipe and --file")
	}

	// 5. Formatting
	// Wrap content first
	if sendWrap != "" {
		content = fmt.Sprintf("```%s\n%s\n```", sendWrap, content)
	}

	// Add before/after
	finalMessage := content
	if sendMdBefore != "" {
		finalMessage = sendMdBefore + "\n\n" + finalMessage
	}
	if sendMdAfter != "" {
		finalMessage = finalMessage + "\n\n" + sendMdAfter
	}

	// 6. Payload Construction
	payload := contract.ChatMessagePayload{
		Text:               finalMessage,
		Type:               "regular",
		Visibility:         sendVisibility,
		NoConfirmation:     sendNoConfirmation,
		ReferenceMessageID: referenceMessageID,
		Replace:            sendReplace,
		InsertBefore:       sendInsertBefore,
		InsertAfter:        sendInsertAfter,
	}

	// 7. Expiration Calculation (1 minute)
	expiresAt := time.Now().Add(1 * time.Minute)

	// 8. Database Insertion
	if err := contract.InsertEvent(contractUUID, chatID, "chat_message", payload, "terminal", expiresAt); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 9. Feedback
	fmt.Printf("✓ Message queued for chat %d\n", chatID)
	if sendNoConfirmation {
		fmt.Printf("! Message will be added to chat automatically.\n")
	} else {
		fmt.Printf("! You have 60 seconds to confirm this message in the Web UI before it expires.\n")
	}
	return nil
}

// readFileContent reads a file and performs validation checks
func readFileContent(path string) (string, error) {
	// Check existence
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// Size Check
	if info.Size() > settings.DefaultMaxSendSize {
		sizeMB := float64(info.Size()) / 1024 / 1024
		fmt.Printf("Warning: File is %.2f MB. Large messages may be truncated by the AI.\n", sizeMB)

		if !sendForce {
			confirm := false
			prompt := &survey.Confirm{
				Message: "Do you want to continue?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
				fmt.Println("Send cancelled.")
				return "", fmt.Errorf("send cancelled by user")
			}
		}
	}

	// Binary Check
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return "", fmt.Errorf("failed to read file header: %w", err)
	}

	if n > 0 && bytes.Contains(buf[:n], []byte{0}) {
		return "", fmt.Errorf("binary files are not supported")
	}

	// Read full content
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file content: %w", err)
	}

	return string(content), nil
}
