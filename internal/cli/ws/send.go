/*
 * Component: Workspace Send Command
 * Block-UUID: ec9c7ec0-ff89-4935-813c-a4e04057a251
 * Parent-UUID: edadae4a-1aff-4395-aa91-a656cde83c70
 * Version: 1.0.1
 * Description: Fixed build error by replacing strings.Builder.ReadFrom with io.ReadAll for reading from stdin.
 * Language: Go
 * Created-at: 2026-03-07T03:32:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1)
 */


package ws

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

var (
	sendFile      string
	sendMdBefore  string
	sendMdAfter   string
	sendWrap      string
	sendVisibility string
	sendForce     bool
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
}

func handleSend(args []string) error {
	// 1. Context Validation
	contractUUID := os.Getenv("GSC_CONTRACT_UUID")
	if contractUUID == "" {
		return fmt.Errorf("not in a GitSense workspace. GSC_CONTRACT_UUID environment variable not set.")
	}

	// 2. Input Resolution
	var content string
	var err error

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
		Text:       finalMessage,
		Type:       "regular",
		Visibility: sendVisibility,
	}

	// 5. Database Insertion
	if err := contract.InsertEvent(contractUUID, "chat_message", payload, "terminal"); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 6. Feedback
	fmt.Printf("✓ Message queued for chat (Type: %s)\n", payload.Type)
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
