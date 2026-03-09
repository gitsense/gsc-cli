/**
 * Component: Workspace Root Command
 * Block-UUID: d896e65b-aaff-4e9c-9e96-a00daa37ac0a
 * Parent-UUID: 0dd61126-bd98-41a6-b7fb-db43b862f34e
 * Version: 1.9.0
 * Description: Fixed compiler error by removing unused 'entry' variable in handleWorkspaceEntry.
 * Language: Go
 * Created-at: 2026-03-09T15:59:35.864Z
 * Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.6.2), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.8.1), GLM-4.7 (v1.9.0)
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
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

var (
	wsID    string
	wsShell bool
)

// wsCmd represents the base command for workspace management
var wsCmd = &cobra.Command{
	Use:   "ws [workspace-id]",
	Short: "Workspace management and entry",
	Long: `The 'ws' command provides tools for interacting with shadow workspaces.
It supports a "Shortcut" mode for quick entry and subcommands for specific actions.`,
	// If no subcommand is provided, run the 'enter' logic (Shortcut Mode)
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			// Shortcut Mode: gsc ws <workspace-id>
			// Implies --shell is true
			return handleWorkspaceEntry(args[0], true, "")
		}
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Enforce GSC_HOME requirement
		if _, err := settings.GetGSCHome(true); err != nil {
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
}

// RegisterCommand adds the ws command and its subcommands to the root command
func RegisterCommand(root *cobra.Command) {
	wsCmd.PersistentFlags().StringVar(&wsID, "id", "", "Workspace hash-position context")
	wsCmd.PersistentFlags().BoolVar(&wsShell, "shell", false, "Keep shell open after action")

	wsCmd.AddCommand(sendCmd)
	wsCmd.AddCommand(ffpCmd)
	root.AddCommand(wsCmd)
}

// handleWorkspaceEntry resolves the workspace and spawns a shell
func handleWorkspaceEntry(input string, keepShell bool, action string) error {
	// 1. Parse Input
	parts := strings.Split(input, "-")
	workspaceID := parts[0] // This is the Composite Hash (Workspace ID)
	position := -1

	if len(parts) > 1 {
		pos, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid position format: %s", parts[1])
		}
		position = pos
	}

	// 2. Locate Workspace via Registry
	// We scan contract JSON files to find which contract owns this workspace ID
	meta, _, err := findWorkspaceByID(workspaceID)
	if err != nil {
		return err
	}

	// 3. Resolve Target Directory
	gscHome, _ := settings.GetGSCHome(false)
	dumpsRoot := filepath.Join(gscHome, settings.DumpsRelPath)
	
	// Construct path directly: dumps/<contract-uuid>/mapped/<workspace-id>
	workspaceRoot := filepath.Join(dumpsRoot, meta.UUID, "mapped", workspaceID)
	
	targetDir := workspaceRoot
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

		for _, f := range ws.Files {
			if f.Position == position {
				if f.Status == contract.MappedStatusMapped {
					targetDir = filepath.Join(workspaceRoot, "mapped", f.Path)
				} else if f.Path != "" {
					targetDir = filepath.Join(workspaceRoot, "unmapped", "components", f.Path)
				} else {
					targetDir = filepath.Join(workspaceRoot, "unmapped", "snippets")
				}
				break
			}
		}
	}

	// 4. Execute Shell
	if keepShell {
		return executeShell(workspaceRoot, targetDir, meta)
	}

	return nil
}

// findWorkspaceByID scans the contract registry (JSON files) to find the workspace.
// This implements the "Registry-First" strategy.
func findWorkspaceByID(workspaceID string) (*contract.ContractMetadata, contract.WorkspaceEntry, error) {
	contractDir, err := manifest.ResolveGlobalContractDir()
	if err != nil {
		return nil, contract.WorkspaceEntry{}, fmt.Errorf("failed to resolve contract directory: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(contractDir, "*.json"))
	if err != nil {
		return nil, contract.WorkspaceEntry{}, fmt.Errorf("failed to scan contracts directory: %w", err)
	}

	for _, file := range files {
		// Load contract metadata
		// We use the internal loadContractMetadata from the contract package
		// Since it's not exported, we have to duplicate the logic or rely on GetContract
		// GetContract is safer and handles path resolution.
		
		// Extract UUID from filename
		uuid := filepath.Base(file)
		uuid = strings.TrimSuffix(uuid, ".json")
		
		meta, err := contract.GetContract(uuid)
		if err != nil {
			// Skip corrupt/unreadable contracts
			continue
		}

		// Check Workspaces map
		if meta.Workspaces != nil {
			if entry, exists := meta.Workspaces[workspaceID]; exists {
				return meta, entry, nil
			}
		}
	}

	return nil, contract.WorkspaceEntry{}, fmt.Errorf("workspace '%s' not found in any active contract", workspaceID)
}

// executeShell spawns a sub-shell in the target directory
func executeShell(workspaceRoot, targetDir string, meta *contract.ContractMetadata) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			shell = "powershell"
		} else {
			shell = "/bin/bash"
		}
	}

	// 1. Load Workspace Metadata
	manifestPath := filepath.Join(workspaceRoot, "workspace.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read workspace manifest: %w", err)
	}
	var ws contract.ShadowWorkspace
	if err := json.Unmarshal(manifestData, &ws); err != nil {
		return fmt.Errorf("corrupted workspace manifest: %w", err)
	}

	// 2. Prepare Template Replacements
	mappedDir := filepath.Dir(workspaceRoot)
	replacements := map[string]string{
		"{{GSC_CHAT_ID}}":       fmt.Sprintf("%d", meta.ChatID),
		"{{GSC_PROJECT_ROOT}}":  meta.Workdir,
		"{{GSC_CONTRACT_UUID}}": meta.UUID,
		"{{GSC_SCRIPTS_DIR}}":   mappedDir,
		"{{TARGET_DIR}}":        targetDir,
	}

	// 3. Process Shell Template
	shellName := filepath.Base(shell)
	ext := "sh"
	if strings.HasSuffix(shellName, "zsh") {
		ext = "zsh"
	} else if strings.HasSuffix(shellName, "powershell") || strings.HasSuffix(shellName, "pwsh") {
		ext = "ps1"
	}

	gscHome, _ := settings.GetGSCHome(false)
	templatePath := filepath.Join(gscHome, "data", "templates", "shells", "ws", runtime.GOOS, "init."+ext)
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read shell init template: %w", err)
	}

	processedContent := string(templateContent)
	for key, val := range replacements {
		processedContent = strings.ReplaceAll(processedContent, key, val)
	}

	// 5. Write Init Script and Prepare Execution
	fmt.Printf("Entering workspace: %s\n", filepath.Base(workspaceRoot))
	fmt.Printf("Location: %s\n", targetDir)
	fmt.Println("Type 'exit' to return to your project.")

	var args []string
	var env []string = os.Environ()

	if ext == "ps1" {
		// Windows/PowerShell Strategy
		initScript := filepath.Join(mappedDir, ".gsc-init.ps1")
		if err := os.WriteFile(initScript, []byte(processedContent), 0755); err != nil {
			return fmt.Errorf("failed to write .gsc-init.ps1: %w", err)
		}
		args = []string{shell, "-NoExit", "-Command", fmt.Sprintf(". \"%s\"", initScript)}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else if ext == "zsh" {
		// Zsh Strategy: Use ZDOTDIR to point to our generated .zshrc
		zshrcPath := filepath.Join(mappedDir, ".zshrc")
		if err := os.WriteFile(zshrcPath, []byte(processedContent), 0644); err != nil {
			return fmt.Errorf("failed to write workspace .zshrc: %w", err)
		}
		env = append(env, fmt.Sprintf("ZDOTDIR=%s", mappedDir))
		args = []string{shell}
	} else {
		// Bash Strategy: Use --rcfile
		initScript := filepath.Join(mappedDir, ".gsc-init.sh")
		if err := os.WriteFile(initScript, []byte(processedContent), 0755); err != nil {
			return fmt.Errorf("failed to write .gsc-init.sh: %w", err)
		}
		args = []string{shell, "--rcfile", initScript}
	}

	binary, err := exec.LookPath(shell)
	if err != nil {
		return fmt.Errorf("shell not found: %w", err)
	}

	return syscall.Exec(binary, args, env)
}
