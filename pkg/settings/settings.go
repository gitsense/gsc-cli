/**
 * Component: Settings
 * Block-UUID: 55237783-18fc-4d69-988c-32f446ac1455
 * Parent-UUID: 0b1b9437-eb33-4875-aa31-dc59d4e857da
 * Version: 1.10.0
 * Description: Added ReviewStagingRelPath and command templates for editors and terminals. Updated DefaultSafeSet to include 'open', 'osascript', 'zed', and 'code' to support the context-aware launcher and review features.
 * Language: Go
 * Created-at: 2026-02-28T18:01:07.174Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), Gemini 3 Flash (v1.10.0)
 */


package settings

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultGitSenseDir is the default name of the directory where GitSense Chat stores its data
const DefaultGitSenseDir = ".gitsense"

// GitSenseDir is the name of the directory where GitSense Chat stores its data
var GitSenseDir = DefaultGitSenseDir

const RegistryFileName = "manifest.json"
const DefaultDBExtension = ".db"
const ManifestJSONExtension = ".json"
const BackupsDir = "backups"
const TempDBSuffix = ".db.tmp"
const MaxBackups = 5
const DefaultMaxBridgeSize = 1048576
const BridgeCodeLength = 6
const RealModelNotes = "GitSense Notes"
const BridgeHandshakeDir = "data/codes"
const ManifestStorageDir = "data/storage/manifests"
const ChatDatabaseRelPath = "data/chats.sqlite3"
const ExecOutputsRelPath = "exec/outputs"
const ContractsRelPath = "data/contracts"

// ReviewStagingRelPath is the relative path within GSC_HOME for temporary review files
const ReviewStagingRelPath = "data/review"

const ProvenanceFileName = "provenance.log"
const ContractHandshakeConsumer = "gsc-contract"
const DefaultContractTTL = 4
const DefaultExecTimeout = 60

// DefaultSafeSet is the list of commands allowed by default when no whitelist is specified.
var DefaultSafeSet = []string{
	// Discovery
	"gsc", "cat", "ls", "find", "tree", "stat", "du", "wc",
	// Search
	"grep", "rg", "awk", "sed",
	// Sampling
	"head", "tail",
	// Version Control
	"git",
	// Build & Runtime
	"npm", "make", "go", "cargo", "python", "pip", "mvn", "gradle",
	// Network
	"curl", "wget",
	// System Context
	"pwd", "date", "whoami", "env",
	// Launchers & Editors
	"open", "osascript", "zed", "code", "vim", "vi",
}

// DefaultEditorTemplates maps editor names to their launch command templates.
// %s is replaced with the file path.
var DefaultEditorTemplates = map[string]string{
	"zed":       "zed %s",
	"code":      "code %s",
	"vim":       "vim %s",
	"vim-iterm2": "osascript -e 'tell application \"iTerm\" to create window with default profile command \"vim %s\"'",
}

// DefaultTerminalTemplates maps terminal names to their launch command templates.
// %s is replaced with the directory path (usually ".").
var DefaultTerminalTemplates = map[string]string{
	"iterm2":       "open -a iTerm %s",
	"terminal.app": "open -a Terminal %s",
}

// GetGSCHome resolves the GSC_HOME directory.
func GetGSCHome(required bool) (string, error) {
	gscHome := os.Getenv("GSC_HOME")
	if gscHome != "" {
		return gscHome, nil
	}

	if required {
		return "", fmt.Errorf("GSC_HOME environment variable is not set")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	return filepath.Join(homeDir, DefaultGitSenseDir), nil
}

// GetChatDatabasePath returns the absolute path to the GitSense Chat database.
func GetChatDatabasePath(gscHome string) string {
	return filepath.Join(gscHome, ChatDatabaseRelPath)
}

// GetManifestStoragePath returns the absolute path to the manifest storage directory.
func GetManifestStoragePath(gscHome string) string {
	return filepath.Join(gscHome, ManifestStorageDir)
}

// GetExecOutputsDir returns the absolute path to the exec outputs directory.
func GetExecOutputsDir() (string, error) {
	gscHome, err := GetGSCHome(false)
	if err != nil {
		return "", fmt.Errorf("failed to resolve GSC_HOME for exec outputs: %w", err)
	}
	return filepath.Join(gscHome, ExecOutputsRelPath), nil
}

// GetReviewStagingDir returns the absolute path to the review staging directory.
func GetReviewStagingDir() (string, error) {
	gscHome, err := GetGSCHome(false)
	if err != nil {
		return "", fmt.Errorf("failed to resolve GSC_HOME for review staging: %w", err)
	}
	return filepath.Join(gscHome, ReviewStagingRelPath), nil
}
