/*
 * Component: Workspace Entry Command
 * Block-UUID: 3452c929-8e33-4c6e-91db-fc27e973895a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc ws' command using hash-position syntax to enter shadow workspaces without a registry.
 * Language: Go
 * Created-at: 2026-03-06T18:15:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package cli

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

var wsCmd = &cobra.Command{
	Use:   "ws [hash-position]",
	Short: "Enter a shadow workspace using its hash and position",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := args[0]
		return handleWorkspaceEntry(input)
	},
}

func handleWorkspaceEntry(input string) error {
	// 1. Parse Input
	// Format: <hash> or <hash>-<position>
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

	// 4. Execute Shell
	return executeShell(workspaceRoot, targetDir)
}

// findWorkspaceByHash scans the dumps directory for a folder matching the hash
func findWorkspaceByHash(dumpsRoot, hash string) (string, error) {
	// Iterate through contract UUIDs
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

	// Prepare Environment
	// We pass the target directory so the init script knows where to cd
	env := append(os.Environ(), fmt.Sprintf("GSC_TARGET_DIR=%s", targetDir))

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
		cmd.Env = env
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

		return syscall.Exec(binary, args, env)
	}
}
