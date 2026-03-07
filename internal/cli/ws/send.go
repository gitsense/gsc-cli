/*
 * Component: Workspace Send Command
 * Block-UUID: 2f9978b9-6ccc-43f5-b0f2-8356ec9d644e
 * Parent-UUID: 79695097-4c2a-4e29-b0e9-4bc23b4f4d21
 * Version: 1.2.0
 * Description: Added --no-chat-confirmation flag to allow bypassing the UI confirmation modal for automated workflows.
 * Language: Go
 * Created-at: 2026-03-07T04:45:31.000Z
 * Authors: Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.1.1), GLM-4.7 (v1.2.0)
 */


package ws

import (
	"bytes"
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
)

// sendCmd represents the 'gsc ws send' command
var sendCmd = &cobra.Command{
	Use:   "send [text]",
	Short: "Send a message from the terminal to the chat",
	Long: `Sends a message to the active chat session via the contract events database.
Supports piping, file input, and markdown formatting.`,
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

	// 2. Input Resolution
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

	// 3. Formatting
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

	// 4. Payload Construction
	payload := contract.ChatMessagePayload{
		Text:           finalMessage,
		Type:           "regular",
		Visibility:     sendVisibility,
		NoConfirmation: sendNoConfirmation,
	}

	// 5. Expiration Calculation (1 minute)
	expiresAt := time.Now().Add(1 * time.Minute)

	// 6. Database Insertion
	if err := contract.InsertEvent(contractUUID, chatID, "chat_message", payload, "terminal", expiresAt); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 7. Feedback
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
