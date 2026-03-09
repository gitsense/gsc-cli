/*
 * Component: Workspace Block Navigation Command
 * Block-UUID: f719ead5-7449-4a57-90bd-2f0c2945898e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc ws block' command for navigating between code blocks in a shadow workspace.
 * Language: Go
 * Created-at: 2026-03-09T17:29:51.605Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/spf13/cobra"
)

// blockCmd represents the 'gsc ws block' command
var blockCmd = &cobra.Command{
	Use:   "block [index|list|next|prev|root]",
	Short: "Navigate between code blocks in the workspace",
	Long: `Provides the path to a specific code block directory for shell navigation.
If no argument is provided, it jumps to the first block (if only one exists) or launches a picker.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleBlock(args)
	},
}

func init() {
	wsCmd.AddCommand(blockCmd)
}

func handleBlock(args []string) error {
	// 1. Locate Workspace Root and Manifest
	wsRoot, manifest, err := findWorkspaceManifest()
	if err != nil {
		return err
	}

	if len(manifest.Files) == 0 {
		return fmt.Errorf("no code blocks found in this workspace")
	}

	// 2. Determine Intent
	intent := ""
	if len(args) > 0 {
		intent = strings.ToLower(args[0])
	}

	// 3. Handle Intent
	switch intent {
	case "root":
		fmt.Println(wsRoot)
		return nil

	case "list":
		return runBlockPicker(wsRoot, manifest)

	case "next", "prev":
		return handleRelativeJump(wsRoot, manifest, intent == "next")

	case "":
		// Smart Default: 1 block -> jump, >1 block -> picker
		if len(manifest.Files) == 1 {
			fmt.Println(resolveBlockPath(wsRoot, manifest.Files[0]))
			return nil
		}
		return runBlockPicker(wsRoot, manifest)

	default:
		// Check if it's a numeric index
		if idx, err := strconv.Atoi(intent); err == nil {
			// We support 1-based indexing for users, but 0-based internally
			targetIdx := idx - 1
			if targetIdx < 0 || targetIdx >= len(manifest.Files) {
				return fmt.Errorf("invalid block index: %d (total blocks: %d)", idx, len(manifest.Files))
			}
			fmt.Println(resolveBlockPath(wsRoot, manifest.Files[targetIdx]))
			return nil
		}
		return fmt.Errorf("unknown block command: %s", intent)
	}
}

// findWorkspaceManifest walks up from PWD to find workspace.json
func findWorkspaceManifest() (string, *contract.ShadowWorkspace, error) {
	curr, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}

	for {
		manifestPath := filepath.Join(curr, "workspace.json")
		if _, err := os.Stat(manifestPath); err == nil {
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				return "", nil, fmt.Errorf("failed to read manifest: %w", err)
			}

			var ws contract.ShadowWorkspace
			if err := json.Unmarshal(data, &ws); err != nil {
				return "", nil, fmt.Errorf("corrupted manifest: %w", err)
			}
			return curr, &ws, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return "", nil, fmt.Errorf("not inside a GitSense workspace (workspace.json not found)")
}

// resolveBlockPath returns the absolute path to the directory containing the block
func resolveBlockPath(wsRoot string, file contract.MappedFileEntry) string {
	if file.Status == contract.MappedStatusMapped {
		return filepath.Join(wsRoot, "mapped", file.Path)
	}
	
	// Unmapped logic
	if file.Path != "" {
		// Components are in unmapped/components/<sanitized_name>
		return filepath.Join(wsRoot, "unmapped", "components", file.Path)
	}
	
	// Snippets are in unmapped/snippets
	return filepath.Join(wsRoot, "unmapped", "snippets")
}

// runBlockPicker launches fzf to select a block
func runBlockPicker(wsRoot string, manifest *contract.ShadowWorkspace) error {
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf is required for the block picker. Please install fzf.")
	}

	// Prepare fzf input
	var input bytes.Buffer
	currDir, _ := os.Getwd()

	for i, f := range manifest.Files {
		prefix := "  "
		blockPath := resolveBlockPath(wsRoot, f)
		if blockPath == currDir {
			prefix = "* "
		}

		label := f.Path
		if label == "" {
			label = "Snippet"
		}

		line := fmt.Sprintf("%s[%d] %-8s %s\n", prefix, i+1, f.Status, label)
		input.WriteString(line)
	}

	// Execute fzf
	// We use --with-nth to hide the selection markers if needed, but simple is better for now
	cmd := exec.Command("fzf", "--header", "Select a block to jump to:", "--reverse", "--height", "40%")
	cmd.Stdin = &input
	cmd.Stderr = os.Stderr // Ensure UI goes to stderr

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return nil // Cancelled
		}
		return err
	}

	// Parse selection (e.g., "  [1] mapped   src/main.go")
	line := string(output)
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start == -1 || end == -1 {
		return fmt.Errorf("failed to parse fzf selection")
	}

	idxStr := line[start+1 : end]
	idx, _ := strconv.Atoi(idxStr)
	
	fmt.Println(resolveBlockPath(wsRoot, manifest.Files[idx-1]))
	return nil
}

// handleRelativeJump calculates the next or previous block relative to PWD
func handleRelativeJump(wsRoot string, manifest *contract.ShadowWorkspace, next bool) error {
	currDir, _ := os.Getwd()
	currIdx := -1

	for i, f := range manifest.Files {
		if resolveBlockPath(wsRoot, f) == currDir {
			currIdx = i
			break
		}
	}

	if currIdx == -1 {
		// If not in a block dir, 'next' goes to 0, 'prev' goes to last
		if next {
			fmt.Println(resolveBlockPath(wsRoot, manifest.Files[0]))
		} else {
			fmt.Println(resolveBlockPath(wsRoot, manifest.Files[len(manifest.Files)-1]))
		}
		return nil
	}

	targetIdx := currIdx
	if next {
		targetIdx = (currIdx + 1) % len(manifest.Files)
	} else {
		targetIdx = (currIdx - 1 + len(manifest.Files)) % len(manifest.Files)
	}

	fmt.Println(resolveBlockPath(wsRoot, manifest.Files[targetIdx]))
	return nil
}
