/*
 * Component: Settings and Configuration Manager
 * Block-UUID: 19000c50-ba9d-488f-9377-6bf34a43eff2
 * Parent-UUID: 67e4640f-8a9e-400f-a443-894b5330c837
 * Version: 3.4.0
 * Description: Refactored LoadTemplates to load command templates from the 'templates/commands/' subdirectory with simplified OS-specific filenames (e.g., 'darwin.json').
 * Language: Go
 * Created-at: 2026-03-06T01:50:18.037Z
 * Authors: GLM-4.7 (v1.0.0), ..., Gemini 3 Flash (v3.2.0), Gemini 3 Flash (v3.3.0), GLM-4.7 (v3.4.0)
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

// DumpsRelPath is the relative path within GSC_HOME for contract dumps
const DumpsRelPath = "data/dumps"

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
