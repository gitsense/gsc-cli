/*
 * Component: Import Git State Management
 * Block-UUID: 786eb968-3de7-4ffd-aa4f-3d9fd5eb3206
 * Parent-UUID: 66c0263f-23d0-4ea2-b18c-d036eaba6808
 * Version: 1.6.0
 * Description: Handles the persistence and retrieval of import state (.gitsense/import-git.json) and checks for .gitignore compliance. Phase 2: Added ShadowPath to BranchState and updated SaveState signature to support shadow repo tracking. Phase 3: Added GroupID to ImportState to support stable repository identification across re-imports. v1.4.0: Fixed SaveState to always update GroupID when loading an existing state file, ensuring old state files are corrected with the current database GroupID. v1.5.0: Added RebuildState struct and Rebuild field to BranchState to consolidate rebuild state into import-git.json. Added SaveRebuildState and ClearRebuildState helpers for atomic rebuild state updates. Updated SaveState to preserve in-progress rebuild state. v1.6.0: Added clearShadowFromState helper to consolidate state file manipulation logic.
 * Language: Go
 * Created-at: 2026-05-13T18:19:30.456Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
 */


package importgit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	stateFileName    = "import-git.json"
	gitsenseDirName  = ".gitsense"
	stateFileVersion = "1"
)

// ImportState represents the structure of the import state file
type ImportState struct {
	Version  string                 `json:"version"`
	Owner    string                 `json:"owner"`
	Repo     string                 `json:"repo"`
	GroupID  int64                  `json:"group_id"` // Phase 3: Added for stable repo identification
	Branches map[string]BranchState `json:"branches"`
}

// BranchState holds state information for a specific branch
type BranchState struct {
	LastImport string        `json:"last_import"`
	RefChatID  int64         `json:"ref_chat_id"`
	Shadow     bool          `json:"shadow"`
	ShadowPath string        `json:"shadow_path,omitempty"` // Phase 2: Path to shadow repo
	Flags      ImportFlags   `json:"flags"`
	Rebuild    *RebuildState `json:"rebuild,omitempty"` // v1.5.0: Non-nil only during an active rebuild
}

// ImportFlags captures the flags used during the import
type ImportFlags struct {
	MaxSize       int    `json:"max_size"`
	Include       string `json:"include"`
	Exclude       string `json:"exclude"`
	IncludeBinary bool   `json:"include_binary"`
}

// RebuildState represents an in-progress rebuild, embedded in BranchState.
// v1.5.0: Moved from rebuild.go to consolidate rebuild state into import-git.json.
type RebuildState struct {
	RefChatID        int64             `json:"ref_chat_id"` // Pre-rebuild ref chat ID for analysis dump/restore
	StartedAt        string            `json:"started_at"`
	AnalysisDumpFile string            `json:"analysis_dump_file"`
	HasAnalysis      bool              `json:"has_analysis"`
	Checkpoints      map[string]string `json:"checkpoints"`
}

// LoadState reads the import state file from the given git repository path
func LoadState(gitPath string) (*ImportState, error) {
	stateFilePath := filepath.Join(gitPath, gitsenseDirName, stateFileName)

	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return nil, err
	}

	var state ImportState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state file: %w", err)
	}

	return &state, nil
}

// SaveState writes the import state file to the given git repository path
func SaveState(gitPath, owner, repo, branch string, refChatID, groupID int64, flags ImportFlags, isShadow bool, shadowPath string) error {
	// Ensure .gitsense directory exists
	gitsenseDir := filepath.Join(gitPath, gitsenseDirName)
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return fmt.Errorf("failed to create .gitsense directory: %w", err)
	}

	stateFilePath := filepath.Join(gitsenseDir, stateFileName)

	// Load existing state if any to preserve other branches
	var state ImportState
	if _, err := os.Stat(stateFilePath); err == nil {
		existingState, err := LoadState(gitPath)
		if err != nil {
			return fmt.Errorf("failed to load existing state for update: %w", err)
		}
		state = *existingState
		
		// v1.4.0: Always update GroupID to the current value from the database
		// This ensures that old state files (with GroupID=0) are corrected
		state.GroupID = groupID
	} else {
		// Initialize new state
		state = ImportState{
			Version:  stateFileVersion,
			Owner:    owner,
			Repo:     repo,
			GroupID:  groupID, // Phase 3: Initialize GroupID
			Branches: make(map[string]BranchState),
		}
	}

	// v1.5.0: Preserve any in-progress rebuild state so SaveState doesn't erase it mid-rebuild.
	var existingRebuild *RebuildState
	if existing, ok := state.Branches[branch]; ok {
		existingRebuild = existing.Rebuild
	}

	// Update or add the branch state
	state.Branches[branch] = BranchState{
		LastImport: time.Now().Format(time.RFC3339),
		RefChatID:  refChatID,
		Shadow:     isShadow, // Phase 2: Use parameter
		ShadowPath: shadowPath, // Phase 2: Use parameter
		Flags:      flags,
		Rebuild:    existingRebuild, // v1.5.0: Preserve rebuild state
	}

	// Marshal and write
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(stateFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// SaveRebuildState atomically updates only the Rebuild field in the branch's entry.
// v1.5.0: Added to support rebuild state consolidation without disturbing other branch state.
func SaveRebuildState(gitPath, branch string, rs *RebuildState) error {
	importState, err := LoadState(gitPath)
	if err != nil {
		return fmt.Errorf("failed to load state for rebuild update: %w", err)
	}
	bs, ok := importState.Branches[branch]
	if !ok {
		return fmt.Errorf("branch %q not found in import state", branch)
	}
	bs.Rebuild = rs
	importState.Branches[branch] = bs

	stateFilePath := filepath.Join(gitPath, gitsenseDirName, stateFileName)
	data, err := json.MarshalIndent(importState, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}
	return os.WriteFile(stateFilePath, data, 0644)
}

// ClearRebuildState sets the Rebuild field to nil on completion.
// v1.5.0: Added to clean up rebuild state after successful completion.
func ClearRebuildState(gitPath, branch string) error {
	return SaveRebuildState(gitPath, branch, nil)
}

// clearShadowFromState removes shadow metadata from the state file
// v1.6.0: Moved from command.go to consolidate state file manipulation logic.
func clearShadowFromState(gitPath, branch string) error {
	state, err := LoadState(gitPath)
	if err != nil {
		return err // State might not exist, which is fine
	}
	
	if bs, ok := state.Branches[branch]; ok {
		bs.Shadow = false
		bs.ShadowPath = ""
		state.Branches[branch] = bs
		
		// Re-save state
		// Note: We need to reconstruct the flags from state or just save what we have
		// For simplicity, we just update the branch state
		gitsenseDir := filepath.Join(gitPath, gitsenseDirName)
		stateFilePath := filepath.Join(gitsenseDir, stateFileName)
		
		data, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(stateFilePath, data, 0644)
	}
	
	return nil
}

// CheckGitIgnore checks if a specific file is ignored by git in the given repository
func CheckGitIgnore(gitPath, relFilePath string) (bool, error) {
	// git check-ignore -q <path>
	// Exit code 0 means ignored
	// Exit code 1 means not ignored
	cmd := exec.Command("git", "check-ignore", "-q", relFilePath)
	cmd.Dir = gitPath
	err := cmd.Run()
	
	if err == nil {
		return true, nil // Ignored
	}
	
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil // Not ignored
	}
	
	return false, err // Error running command
}
