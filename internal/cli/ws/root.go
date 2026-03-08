/*
 * Component: Workspace Root Command
 * Block-UUID: 1585903b-df1f-4278-86a8-485148ca099c
 * Parent-UUID: 0da619d3-c526-44ba-8185-f39c3463a08e
 * Version: 1.6.1
 * Description: Removed unused 'hash' variable declaration in Zsh execution block to fix build error.
 * Language: Go
 * Created-at: 2026-03-07T20:09:32.589Z
 * Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.6.1)
 */


package ws

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

var (
	wsID    string
	wsShell bool
)

// wsCmd represents the base command for workspace management
var wsCmd = &cobra.Command{
	Use:   "ws [hash-position]",
	Short: "Workspace management and entry",
	Long: `The 'ws' command provides tools for interacting with shadow workspaces.
It supports a "Shortcut" mode for quick entry and subcommands for specific actions.`,
	// If no subcommand is provided, run the 'enter' logic (Shortcut Mode)
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			// Shortcut Mode: gsc ws <hash-position>
			// Implies --shell is true
			return handleWorkspaceEntry(args[0], true, "")
		}
		return cmd.Help()
	},
}

// RegisterCommand adds the ws command and its subcommands to the root command
func RegisterCommand(root *cobra.Command) {
	// Add persistent flags (available to all subcommands)
	wsCmd.PersistentFlags().StringVar(&wsID, "id", "", "Workspace hash-position context")
	wsCmd.PersistentFlags().BoolVar(&wsShell, "shell", false, "Keep shell open after action")

	// Register subcommands
	wsCmd.AddCommand(sendCmd)

	root.AddCommand(wsCmd)
}

// handleWorkspaceEntry resolves the workspace and spawns a shell
func handleWorkspaceEntry(input string, keepShell bool, action string) error {
	// 1. Parse Input
	parts := strings.Split(input, "-")
	hash := parts[0]
	position := -1

	if len(parts) > 1 {
		pos, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid position format: %s", parts[1])
		}
		position = pos
	}

	// 2. Locate Workspace Directory
	gscHome, _ := settings.GetGSCHome(false)
	dumpsRoot := filepath.Join(gscHome, settings.DumpsRelPath)

	workspaceRoot, err := findWorkspaceByHash(dumpsRoot, hash)
	if err != nil {
		return err
	}

	// 3. Resolve Target Directory
	targetDir := workspaceRoot // Default to root if no position

	if position >= 0 {
		manifestPath := filepath.Join(workspaceRoot, "workspace.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to read workspace manifest: %w", err)
		}

		var ws contract.ShadowWorkspace
		if err := json.Unmarshal(data, &ws); err != nil {
			return fmt.Errorf("corrupted workspace manifest: %w", err)
		}

		// Find the file entry
		for _, f := range ws.Files {
			if f.Position == position {
				if f.Status == contract.MappedStatusMapped {
					targetDir = filepath.Join(workspaceRoot, "mapped", f.Path)
				} else if f.Path != "" {
					// Unmapped component
					targetDir = filepath.Join(workspaceRoot, "unmapped", "components", f.Path)
				} else {
					// Unmapped snippet
					targetDir = filepath.Join(workspaceRoot, "unmapped", "snippets")
				}
				break
			}
		}

		if targetDir == workspaceRoot {
			return fmt.Errorf("position %d not found in workspace manifest", position)
		}
	}

	// 4. Handle Action (Stub)
	if action != "" {
		fmt.Printf("Executing action: %s\n", action)
		// TODO: Implement action logic (save, undo, diff)
	}

	// 5. Generate Init Script
	// We need the ContractUUID to get the ProjectRoot.
	// We can get it from the workspace.json
	var wsMeta contract.ShadowWorkspace
	data, _ := os.ReadFile(filepath.Join(workspaceRoot, "workspace.json"))
	json.Unmarshal(data, &wsMeta)

	// Fetch contract metadata to get ProjectRoot
	meta, err := contract.GetContract(wsMeta.ContractUUID)
	if err != nil {
		return fmt.Errorf("failed to load contract metadata: %w", err)
	}

	// Calculate mappedDir (parent of workspaceRoot)
	mappedDir := filepath.Dir(workspaceRoot)

	if err := contract.GenerateShellInitScript(mappedDir, meta.ChatID, meta.UUID, meta.Workdir, hash, targetDir); err != nil {
		return fmt.Errorf("failed to generate shell init script: %w", err)
	}

	// 6. Execute Shell
	if keepShell {
		return executeShell(workspaceRoot, targetDir)
	}

	return nil
}

// findWorkspaceByHash scans the dumps directory for a folder matching the hash
func findWorkspaceByHash(dumpsRoot, hash string) (string, error) {
	entries, err := os.ReadDir(dumpsRoot)
	if err != nil {
		return "", fmt.Errorf("dumps directory not found: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check mapped/<hash>
		mappedPath := filepath.Join(dumpsRoot, entry.Name(), "mapped", hash)
		if info, err := os.Stat(mappedPath); err == nil && info.IsDir() {
			return mappedPath, nil
		}
	}

	return "", fmt.Errorf("workspace with hash '%s' not found", hash)
}

// executeShell spawns a sub-shell in the target directory
func executeShell(workspaceRoot, targetDir string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "powershell"
		} else {
			shell = "/bin/bash"
		}
	}

	fmt.Printf("Entering workspace: %s\n", filepath.Base(workspaceRoot))
	fmt.Printf("Location: %s\n", targetDir)
	fmt.Println("Type 'exit' to return to your project.")

	// Calculate mappedDir (parent of workspaceRoot)
	mappedDir := filepath.Dir(workspaceRoot)

	if runtime.GOOS == "windows" {
		// Windows: Spawn PowerShell
		initScript := filepath.Join(mappedDir, ".gsc-init.ps1")
		cmd := exec.Command(shell, "-NoExit", "-Command", fmt.Sprintf(". \"%s\"", initScript))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		// Unix: Use syscall.Exec to replace the Go process
		// Detect if the shell is Zsh
		isZsh := strings.HasSuffix(filepath.Base(shell), "zsh")

		var args []string
		var env []string

		if isZsh {
			// ==========================================
			// ZSH STRATEGY: ZDOTDIR + Loader Script
			// ==========================================
			
			// 1. Load Metadata for Template Substitution
			manifestPath := filepath.Join(workspaceRoot, "workspace.json")
			manifestData, err := os.ReadFile(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to read workspace manifest: %w", err)
			}
			var ws contract.ShadowWorkspace
			if err := json.Unmarshal(manifestData, &ws); err != nil {
				return fmt.Errorf("corrupted workspace manifest: %w", err)
			}

			meta, err := contract.GetContract(ws.ContractUUID)
			if err != nil {
				return fmt.Errorf("failed to load contract metadata: %w", err)
			}

			// 2. Load the Zsh Template
			gscHome, _ := settings.GetGSCHome(false)
			templatePath := filepath.Join(gscHome, "data", "templates", "shells", "ws", runtime.GOOS, "init.zsh")
			
			templateContent, err := os.ReadFile(templatePath)
			if err != nil {
				return fmt.Errorf("failed to read zsh init template (ensure templates are bootstrapped): %w", err)
			}

			// 3. Substitute Variables
			replacements := map[string]string{
				"{{GSC_CHAT_ID}}":        fmt.Sprintf("%d", meta.ChatID),
				"{{GSC_PROJECT_ROOT}}":   meta.Workdir,
				"{{GSC_CONTRACT_UUID}}":  meta.UUID,
				"{{GSC_SCRIPTS_DIR}}":   mappedDir,
				"{{TARGET_DIR}}":         targetDir,
			}

			processedContent := string(templateContent)
			for key, val := range replacements {
				processedContent = strings.ReplaceAll(processedContent, key, val)
			}

			// 4. Write .gsc-init.zsh
			zshInitPath := filepath.Join(mappedDir, ".gsc-init.zsh")
			if err := os.WriteFile(zshInitPath, []byte(processedContent), 0755); err != nil {
				return fmt.Errorf("failed to write .gsc-init.zsh: %w", err)
			}

			// 5. Write .zshrc (The Loader)
			// This file sources the user's real .zshrc first, then our init script.
			zshrcPath := filepath.Join(mappedDir, ".zshrc")
			zshrcContent := fmt.Sprintf(`# GitSense Workspace Loader
# This file is loaded by Zsh because ZDOTDIR is set to this directory.

# 1. Source the user's original .zshrc to preserve their theme and plugins
if [[ -f "$HOME/.zshrc" ]]; then
    source "$HOME/.zshrc"
fi

# 2. Source the GitSense workspace init script
# This sets the prompt, aliases, and environment variables
if [[ -f "%s" ]]; then
    source "%s"
fi
`, zshInitPath, zshInitPath)

			if err := os.WriteFile(zshrcPath, []byte(zshrcContent), 0644); err != nil {
				return fmt.Errorf("failed to write workspace .zshrc: %w", err)
			}

			// 6. Set Environment Variables
			env = os.Environ()
			env = append(env, fmt.Sprintf("ZDOTDIR=%s", mappedDir))

			// 7. Execute Zsh
			args = []string{shell}

		} else {
			// ==========================================
			// BASH STRATEGY: --rcfile
			// ==========================================
			initScript := filepath.Join(mappedDir, ".gsc-init.sh")
			args = []string{shell, "--rcfile", initScript}
			env = os.Environ()
		}

		binary, err := exec.LookPath(shell)
		if err != nil {
			return fmt.Errorf("shell not found: %w", err)
		}

		return syscall.Exec(binary, args, env)
	}
}
