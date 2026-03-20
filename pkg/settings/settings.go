/**
 * Component: Settings and Configuration Manager
 * Block-UUID: cce037b3-0d6f-4527-96da-e6cb45cefa57
 * Parent-UUID: 36aae72e-ef71-45d0-ae8b-0df66b659311
 * Version: 3.12.0
 * Description: Added Docker-related constants for container management and context tracking to support the gsc docker command suite.
 * Language: Go
 * Created-at: 2026-03-20T16:07:59.833Z
 * Authors: GLM-4.7 (v3.5.0), GLM-4.7 (v3.6.0), GLM-4.7 (v3.7.0), Gemini 3 Flash (v3.8.0), Gemini 3 Flash (v3.9.0), GLM-4.7 (v3.10.0), Gemini 3 Flash (v3.11.0), Gemini 3 Flash (v3.12.0)
 */


package settings

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/gitsense/gsc-cli/pkg/logger"
)

//go:embed templates/*
var templateFS embed.FS

// DefaultGitSenseDir is the default name of the directory where GitSense Chat stores its data
const DefaultGitSenseDir = ".gitsense"

// GitSenseDir is the name of the directory where GitSense Chat stores its data
var GitSenseDir = DefaultGitSenseDir

// DockerRootPrefix is the unique root path used in Docker environments to signal 
// that paths require translation when accessed from a host machine.
const DockerRootPrefix = "/gsc-docker-app"

// DockerContextFileName is the hidden file used to track the active Docker proxy context.
const DockerContextFileName = ".gsc-docker-context.json"

// Docker Defaults
const DefaultContainerName = "gitsense-chat"
const DefaultImageName = "gitsense/chat"
const DefaultAppPort = "3357"

// DockerDataDirRelPath is the relative path within GSC_HOME for Docker-specific persistent data.
// The 'docker/' prefix ensures isolation from native 'app/' installations.
const DockerDataDirRelPath = "docker/data"

// DockerReposDirRelPath is the relative path within GSC_HOME for the default Docker repository sandbox.
// This sibling directory to 'data' ensures consistent path translation and isolation.
const DockerReposDirRelPath = "docker/repos"

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
const ExecOutputsRelPath = "data/exec/outputs"
const ContractsRelPath = "data/contracts"

// HomesRelPath is the relative path within GSC_HOME for contract homes
const HomesRelPath = "data/homes"

// ReviewStagingRelPath is the relative path within GSC_HOME for temporary review files
const ReviewStagingRelPath = "data/review"

const ProvenanceFileName = "provenance.log"
const ContractHandshakeConsumer = "gsc-contract"
const DefaultContractTTL = 4
const DefaultExecTimeout = 60

// Sort Modes for the 'merged' dump type
const SortRecency = "recency"
const SortPopularity = "popularity"
const SortChronological = "chronological"

// DefaultMaxSendSize is the default size limit (in bytes) for the 'gsc ws send' command
// before a warning is triggered. Default is 500KB.
const DefaultMaxSendSize = 500 * 1024

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
	"cd", "pwd", "date", "whoami", "env", "echo",
	// File Manipulation
	"mkdir", "touch", "cp", "mv",
	// Launchers & Editors
	"open", "osascript", "zed", "code", "vim", "vi",
}

// DefaultEditorTemplates maps editor aliases to their command templates.
// This is populated by LoadTemplates() at runtime.
var DefaultEditorTemplates = make(map[string]string)

// DefaultTerminalTemplates maps terminal aliases to their command templates.
// This is populated by LoadTemplates() at runtime.
var DefaultTerminalTemplates = make(map[string]string)

// TemplateConfig represents the structure of the commands.<os>.json file
type TemplateConfig struct {
	Editors   map[string]string `json:"editors"`
	Terminals map[string]string `json:"terminals"`
}

func init() {
	// Load templates from embedded files or local JSON files on package initialization
	if err := LoadTemplates(); err != nil {
		logger.Warning("Failed to load templates from configuration, falling back to internal defaults", "error", err)
		// Fallback to hardcoded defaults if loading fails
		loadHardcodedDefaults()
	}
}

// LoadTemplates initializes the editor and terminal templates.
// It recursively bootstraps the templates directory from the embedded filesystem.
func LoadTemplates() error {
	gscHome, err := GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	localTemplatesDir := filepath.Join(gscHome, "data", "templates")

	// 1. Recursively bootstrap the entire templates directory
	// This ensures help/, shells/, and commands/ subdirectories are created and populated.
	if err := bootstrapTemplatesRecursive("templates", localTemplatesDir); err != nil {
		return fmt.Errorf("failed to bootstrap templates: %w", err)
	}

	// 2. Load the OS-specific JSON configuration from the 'commands' subdirectory
	osName := runtime.GOOS
	// New path: templates/commands/<os>.json
	jsonFileName := fmt.Sprintf("%s.json", osName)
	localJsonPath := filepath.Join(localTemplatesDir, "commands", jsonFileName)

	data, err := os.ReadFile(localJsonPath)
	if err != nil {
		return fmt.Errorf("failed to read local command template file %s: %w", localJsonPath, err)
	}

	var config TemplateConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse local command template file: %w", err)
	}

	// 3. Populate global maps
	if config.Editors != nil {
		DefaultEditorTemplates = config.Editors
	}
	if config.Terminals != nil {
		DefaultTerminalTemplates = config.Terminals
	}

	return nil
}

// bootstrapTemplatesRecursive walks the embedded filesystem and copies missing files to the local path.
func bootstrapTemplatesRecursive(srcDir, destDir string) error {
	entries, err := templateFS.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// Ensure local directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := path.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			// Recurse into subdirectory
			if err := bootstrapTemplatesRecursive(srcPath, destPath); err != nil {
				return err
			}
		} else {
			// Copy file if it doesn't exist locally
			if _, err := os.Stat(destPath); os.IsNotExist(err) {
				logger.Info("Bootstrapping template file", "file", entry.Name(), "path", destPath)
				if err := copyEmbeddedFile(srcPath, destPath); err != nil {
					logger.Warning("Failed to bootstrap template file", "file", entry.Name(), "error", err)
				}
			}
		}
	}

	return nil
}

// copyEmbeddedFile reads a file from the embedded filesystem and writes it to the local path.
func copyEmbeddedFile(embedPath, localPath string) error {
	data, err := templateFS.ReadFile(embedPath)
	if err != nil {
		return err
	}
	return os.WriteFile(localPath, data, 0644)
}

// loadHardcodedDefaults sets the global maps to safe fallback values if JSON loading fails.
func loadHardcodedDefaults() {
	logger.Info("Loading hardcoded default templates")

	// Common defaults that work across platforms
	DefaultEditorTemplates = map[string]string{
		"vim":  "vim %s",
		"nano": "nano %s",
		"code": "code %s", // VS Code
	}

	DefaultTerminalTemplates = map[string]string{
		"bash": "bash -c 'cd %s && exec bash'",
	}
}

// GetGSCHome resolves the GSC_HOME directory.
// If create is true, the directory will be created if it doesn't exist.
func GetGSCHome(required bool) (string, error) {
	logger.Debug("GetGSCHome called", "required", required)
	gscHome := os.Getenv("GSC_HOME")
	logger.Debug("GSC_HOME env var check", "value", gscHome)

	if gscHome != "" {
		logger.Debug("Returning GSC_HOME from env", "path", gscHome)
		return gscHome, nil
	}

	if required {
		logger.Debug("GSC_HOME required but not set, returning error")
		return "", fmt.Errorf("GSC_HOME environment variable is not set")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	fallbackPath := filepath.Join(homeDir, DefaultGitSenseDir)
	logger.Debug("Returning fallback path", "path", fallbackPath)
	return fallbackPath, nil
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
