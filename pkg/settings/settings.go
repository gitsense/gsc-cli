/**
 * Component: Settings
 * Block-UUID: 0b1b9437-eb33-4875-aa31-dc59d4e857da
 * Parent-UUID: 99d7e8e3-601c-4687-8410-acfc42683306
 * Version: 1.9.0
 * Description: Added DefaultExecTimeout and DefaultSafeSet constants to support the 'gsc contract exec' security framework and command whitelisting.
 * Language: Go
 * Created-at: 2026-02-28T18:01:07.174Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0)
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
// This is a variable so it can be overridden by CLI flags
var GitSenseDir = DefaultGitSenseDir

// RegistryFileName is the name of the file that tracks all manifests
const RegistryFileName = "manifest.json"

// DefaultDBExtension is the file extension for SQLite databases
const DefaultDBExtension = ".db"

// ManifestJSONExtension is the file extension for manifest dump files
const ManifestJSONExtension = ".json"

// BackupsDir is the name of the subdirectory within .gitsense where database backups are stored
const BackupsDir = "backups"

// TempDBSuffix is the suffix used for temporary database files during atomic imports
const TempDBSuffix = ".db.tmp"

// MaxBackups is the maximum number of backup files to retain for a single database
const MaxBackups = 5

// DefaultMaxBridgeSize is the default maximum size (1MB) for bridge output
const DefaultMaxBridgeSize = 1048576

// BridgeCodeLength is the required length of the 6-digit bridge code
const BridgeCodeLength = 6

// RealModelNotes is the internal model name for custom bridge messages
const RealModelNotes = "GitSense Notes"

// BridgeHandshakeDir is the relative path within GSC_HOME for handshake files
const BridgeHandshakeDir = "data/codes"

// ManifestStorageDir is the relative path within GSC_HOME for published manifest files
const ManifestStorageDir = "data/storage/manifests"

// ChatDatabaseRelPath is the relative path within GSC_HOME for the chat database
const ChatDatabaseRelPath = "data/chats.sqlite3"

// ExecOutputsRelPath is the relative path within GSC_HOME for exec command outputs
const ExecOutputsRelPath = "exec/outputs"

// ContractsRelPath is the relative path within GSC_HOME for contract metadata
const ContractsRelPath = "data/contracts"

// ProvenanceFileName is the name of the project-local audit log
const ProvenanceFileName = "provenance.log"

// ContractHandshakeConsumer is the consumer name for contract-related handshakes
const ContractHandshakeConsumer = "gsc-contract"

// DefaultContractTTL is the default time-to-live for a contract (4 hours)
const DefaultContractTTL = 4

// DefaultExecTimeout is the default execution timeout in seconds for contract commands.
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
}

// GetGSCHome resolves the GSC_HOME directory. If required is true, it returns an
// error if the environment variable is not set. If required is false, it falls
// back to the user's home directory .gitsense folder.
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
