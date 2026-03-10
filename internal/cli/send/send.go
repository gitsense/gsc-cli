/**
 * Component: Shared Send Logic
 * Block-UUID: 4816865e-1c8d-47ce-b972-10e9906052a5
 * Parent-UUID: 7b5bb7ce-58bf-4437-85ce-8978102e84d3
 * Version: 1.1.0
 * Description: Core logic for processing and sending chat messages from the CLI, shared between 'ws send' and 'contract send'.
 * Language: Go
 * Created-at: 2026-03-10T22:33:03.945Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package send

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// Options encapsulates all possible configurations for a send operation.
type Options struct {
	ContractUUID   string
	ChatID         int64
	Text           string // From command line arguments
	File           string // Path to a file to read
	MdBefore       string // Markdown to prepend
	MdAfter        string // Markdown to append
	Wrap           string // Language for code block wrapping
	Visibility     string // "human-public" or "human-only"
	NoSizeLimit    bool   // Skip confirmation for large files
	NoConfirmation bool   // Bypass the UI confirmation modal

	// Manipulation (Workspace specific)
	ReferenceMessageID int64
	Replace            bool
	InsertBefore       bool
	InsertAfter        bool
}

// Perform executes the shared logic for processing input and queuing a chat message event.
func Perform(opts Options) error {
	// 1. Input Resolution
	var content string
	stat, _ := os.Stdin.Stat()

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		if opts.File != "" {
			return fmt.Errorf("cannot use both pipe and --file")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		content = string(data)
	} else if opts.File != "" {
		var err error
		content, err = readFileContent(opts.File, opts.NoSizeLimit)
		if err != nil {
			return err
		}
	} else if opts.Text != "" {
		content = opts.Text
	} else {
		return fmt.Errorf("no input provided. Use pipe, --file, or provide text argument.")
	}

	// 2. Formatting
	if opts.Wrap != "" {
		content = fmt.Sprintf("```%s\n%s\n```", opts.Wrap, content)
	}

	finalMessage := content
	if opts.MdBefore != "" {
		finalMessage = opts.MdBefore + "\n\n" + finalMessage
	}
	if opts.MdAfter != "" {
		finalMessage = finalMessage + "\n\n" + opts.MdAfter
	}

	// 3. Payload Construction
	payload := contract.ChatMessagePayload{
		Text:               finalMessage,
		Type:               "regular",
		Visibility:         opts.Visibility,
		NoConfirmation:     opts.NoConfirmation,
		ReferenceMessageID: opts.ReferenceMessageID,
		Replace:            opts.Replace,
		InsertBefore:       opts.InsertBefore,
		InsertAfter:        opts.InsertAfter,
	}

	// 4. Database Insertion (1 minute expiration)
	expiresAt := time.Now().Add(1 * time.Minute)
	if err := contract.InsertEvent(opts.ContractUUID, opts.ChatID, "chat_message", payload, "terminal", expiresAt); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// 5. Feedback
	fmt.Printf("✓ Message queued for chat %d\n", opts.ChatID)
	if opts.NoConfirmation {
		fmt.Printf("! Message will be added to chat automatically.\n")
	} else {
		fmt.Printf("! You have 60 seconds to confirm this message in the Web UI before it expires.\n")
	}

	return nil
}

// readFileContent reads a file and performs validation checks (size and binary check).
func readFileContent(path string, noSizeLimit bool) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// Size Check
	if info.Size() > settings.DefaultMaxSendSize {
		sizeMB := float64(info.Size()) / 1024 / 1024
		fmt.Printf("Warning: File is %.2f MB. Large messages may be truncated by the AI.\n", sizeMB)

		if !noSizeLimit {
			confirm := false
			prompt := &survey.Confirm{
				Message: "Do you want to continue?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &confirm); err != nil || !confirm {
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
