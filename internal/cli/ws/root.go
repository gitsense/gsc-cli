/*
 * Component: Workspace Root Command
 * Block-UUID: 1f3d22cb-8990-4d52-98b4-2ce05ccc22a2
 * Parent-UUID: 0f18e2bf-d323-4a03-a562-c421305e2258
 * Version: 1.2.0
 * Description: Added PersistentPreRunE to enforce GSC_HOME requirement for all ws subcommands, ensuring the workspace environment is correctly configured before execution.
 * Language: Go
 * Created-at: 2026-03-07T02:50:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
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
	Short: "Shadow workspace management and entry",
	Long: `The 'ws' command provides tools for interacting with shadow workspaces.
It supports a "Shortcut" mode for quick entry and subcommands for specific actions.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Enforce GSC_HOME requirement
		// This ensures that the web app's data directory is used for dumps and events
		if _, err := settings.GetGSCHome(true); err != nil {
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
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

	if err := contract.GenerateShellInitScript(workspaceRoot, meta.ChatID, meta.UUID, meta.Workdir, hash, targetDir); err != nil {
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

	initScript := filepath.Join(workspaceRoot, ".gsc-init.sh")
	if runtime.GOOS == "windows" {
		initScript = filepath.Join(workspaceRoot, ".gsc-init.ps1")
	}

	if runtime.GOOS == "windows" {
		// Windows: Spawn PowerShell
		cmd := exec.Command(shell, "-NoExit", "-Command", fmt.Sprintf(". \"%s\"", initScript))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		// Unix: Use syscall.Exec to replace the Go process
		args := []string{shell, "--rcfile", initScript}

		binary, err := exec.LookPath(shell)
		if err != nil {
			return fmt.Errorf("shell not found: %w", err)
		}

		return syscall.Exec(binary, args, os.Environ())
	}
}
