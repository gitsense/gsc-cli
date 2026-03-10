/**
 * Component: Workspace Fuzzy Find Command
 * Block-UUID: 7e6eb55b-e9d0-4078-8c8e-f88b74a14311
 * Parent-UUID: 062cb964-d634-4ea3-94f1-389f1a325ec3
 * Version: 1.3.0
 * Description: Implements the 'gsc ws ffp' command to fuzzy find files in the project root and perform actions like diff, edit, or copy.
 * Language: Go
 * Created-at: 2026-03-09T23:51:58.549Z
 * Authors: GLM-4.4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.2.1), GLM-4.7 (v1.2.2), GLM-4.7 (v1.3.0)
 */


package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	ffpDiff     bool
	ffpOpen     bool
	ffpFrom     string
	ffpCopy     bool
	ffpRelative bool
	ffpPath     bool
)

// ffpCmd represents the 'gsc ws ffp' command
var ffpCmd = &cobra.Command{
	Use:   "ffp [flags]",
	Short: "Fuzzy find files in the project root",
	Long: `Fuzzy find files in the real project root while inside a shadow workspace.
Supports actions like diffing against the generated code, editing, or copying paths.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleFFP()
	},
}

func init() {
	ffpCmd.Flags().BoolVar(&ffpDiff, "diff", false, "Diff the selected file against the generated code in the workspace")
	ffpCmd.Flags().StringVar(&ffpFrom, "from", "", "The source file for diff (defaults to 'generated.*' in CWD)")
	ffpCmd.Flags().BoolVar(&ffpOpen, "open", false, "Open the selected file in the preferred editor")
	ffpCmd.Flags().BoolVar(&ffpCopy, "copy", false, "Copy the content of the selected file to the clipboard")
	ffpCmd.Flags().BoolVar(&ffpRelative, "relative", false, "Copy the relative path to the clipboard")
	ffpCmd.Flags().BoolVar(&ffpPath, "path", false, "Copy the absolute path to the clipboard (default)")
}

func handleFFP() error {
	// 1. Context Validation
	projectRoot := os.Getenv("GSC_PROJECT_ROOT")
	if projectRoot == "" {
		return fmt.Errorf("GSC_PROJECT_ROOT environment variable is not set. Are you in a GitSense workspace?")
	}

	// 2. Dependency Check: fzf
	if _, err := exec.LookPath("fzf"); err != nil {
		return fmt.Errorf("fzf is not installed or not in PATH. Please install fzf to use this command.")
	}

	// 3. Dependency Check: bat (optional)
	hasBat := false
	if _, err := exec.LookPath("bat"); err == nil {
		hasBat = true
	}

	// 4. Construct fzf command
	// We use 'find' to list files recursively.
	var findCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("find"); err != nil {
			return fmt.Errorf("this command currently requires 'find' (Unix) or a Git Bash environment on Windows")
		}
		findCmd = exec.Command("find", ".", "-type", "d", "-name", ".git", "-prune", "-o", "-type", "f", "-print")
	} else {
		findCmd = exec.Command("find", ".", "-type", "d", "-name", ".git", "-prune", "-o", "-type", "f", "-print")
	}

	// Set working directory to project root
	findCmd.Dir = projectRoot

	// Capture find output
	findOutput, err := findCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list files in project root: %w", err)
	}

	// 5. Execute fzf
	// We must prepend the project root to the preview command because fzf runs
	// in the shadow workspace (CWD), but the file paths are relative to the project root.
	rootPrefix := fmt.Sprintf("\"%s/\"", projectRoot)
	previewCmd := fmt.Sprintf("head -20 %s{}", rootPrefix)
	if hasBat {
		previewCmd = fmt.Sprintf("bat --color=always %s{} 2>/dev/null || head -20 %s{}", rootPrefix, rootPrefix)
	}

	fzfArgs := []string{
		"--preview", previewCmd,
		"--preview-window", "right:60%",
		"--select-1",
		"--exit-0",
	}

	fzfCmd := exec.Command("fzf", fzfArgs...)
	fzfCmd.Stdin = bytes.NewReader(findOutput)

	// Run fzf and capture output
	selectedFileRel, err := fzfCmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 130 {
			fmt.Println("Selection cancelled.")
			return nil
		}
		return fmt.Errorf("fzf execution failed: %w", err)
	}

	selectedFileRelStr := string(selectedFileRel)
	// Trim whitespace/newlines (fzf adds a trailing newline)
	selectedFileRelStr = strings.TrimSpace(selectedFileRelStr)
	selectedFileRelStr = filepath.Clean(selectedFileRelStr)

	// 6. Construct Absolute Path
	absPath := filepath.Join(projectRoot, selectedFileRelStr)

	// 7. Handle Actions
	if ffpDiff {
		return handleDiffAction(absPath, selectedFileRelStr)
	}

	if ffpOpen {
		return handleOpenAction(absPath)
	}

	if ffpCopy {
		return handleCopyContentAction(absPath)
	}

	if ffpRelative {
		return copyToClipboard(selectedFileRelStr)
	}

	// Default: Copy Absolute Path
	if err := copyToClipboard(absPath); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}

	fmt.Printf("Copied: %s\n", absPath)
	return nil
}

// handleDiffAction executes the diff tool
func handleDiffAction(projectFileAbs, projectFileRel string) error {
	// Resolve 'from' file
	fromFile := ffpFrom
	if fromFile == "" {
		// Default to generated.* in CWD
		matches, err := filepath.Glob("generated.*")
		if err != nil {
			return fmt.Errorf("failed to glob for generated files: %w", err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("no 'generated.*' file found in current directory. Please specify --from.")
		}
		if len(matches) > 1 {
			return fmt.Errorf("multiple 'generated.*' files found in current directory. Please specify --from.")
		}
		fromFile = matches[0]
	}

	// Resolve Diff Tool
	diffTool := "diff" // Default
	// Try to load contract to get PreferredReview
	contractUUID := os.Getenv("GSC_CONTRACT_UUID")
	if contractUUID != "" {
		// We need to load the contract. 
		// To avoid circular dependency or complex imports, we can read the JSON directly
		// or rely on the contract package if available. 
		// For simplicity in this file, we'll try to read the contract JSON.
		contractDir, _ := filepath.Abs(filepath.Join(os.Getenv("HOME"), ".gitsense", "contracts"))
		contractPath := filepath.Join(contractDir, contractUUID+".json")
		
		data, err := os.ReadFile(contractPath)
		if err == nil {
			var meta struct {
				PreferredReview string `json:"preferred_review"`
			}
			if json.Unmarshal(data, &meta) == nil && meta.PreferredReview != "" {
				diffTool = meta.PreferredReview
			}
		}
	}

	// Execute Diff
	cmd := exec.Command(diffTool, fromFile, projectFileAbs)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	err := cmd.Run()
	if err != nil {
		// diff returns 1 if files differ, 2 if there's an error.
		// We only want to fail on 2.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil // Differences found, but command succeeded
			}
		}
		return err // Propagate real errors (exit code 2 or other)
	}
	return nil
}

// handleOpenAction opens the file in the preferred editor
func handleOpenAction(absPath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // Default
	}
	
	cmd := exec.Command(editor, absPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// handleCopyContentAction reads the file and copies content to clipboard
func handleCopyContentAction(absPath string) error {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	if err := copyToClipboard(string(content)); err != nil {
		return fmt.Errorf("failed to copy content to clipboard: %w", err)
	}
	
	fmt.Printf("Copied content of: %s\n", absPath)
	return nil
}

// copyToClipboard copies the given text to the system clipboard
func copyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard utility found. Please install xclip or xsel.")
		}
	case "windows":
		if _, err := exec.LookPath("powershell"); err == nil {
			cmd = exec.Command("powershell", "-command", fmt.Sprintf("Set-Clipboard -Value \"%s\"", text))
		} else {
			return fmt.Errorf("clipboard utility not found on Windows.")
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	cmd.Stdin = bytes.NewReader([]byte(text))
	return cmd.Run()
}
