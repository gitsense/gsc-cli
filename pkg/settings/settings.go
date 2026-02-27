/**
 * Component: Settings
 * Block-UUID: 428e959b-88e8-4988-9b3b-726902e42896
 * Parent-UUID: 5f19ef23-e1c7-455f-aeec-046dbbcbcba8
 * Version: 1.7.0
 * Description: Added constants for the Contract and Provenance systems, including relative paths and default TTL.
 * Language: Go
 * Created-at: 2026-02-26T18:21:22.185Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0)
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
