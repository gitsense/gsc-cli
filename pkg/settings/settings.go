/*
 * Component: Settings and Configuration Manager
 * Block-UUID: dc816fe6-6737-44e7-acab-d916ca78482d
 * Parent-UUID: d72cd4fa-0f8c-4c55-9326-69ab43b2164f
 * Version: 3.1.0
 * Description: Added SortRecency, SortPopularity, and SortChronological constants to support the new 'merged' dump type sorting strategies.
 * Language: Go
 * Created-at: 2026-03-02T07:50:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), Gemini 3 Flash (v1.10.0), GLM-4.7 (v2.0.0), GLM-4.7 (v3.0.0), Gemini 3 Flash (v3.1.0)
 */


package settings

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
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

// TemplateConfig represents the structure of the templates.<os>.json file
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
// It attempts to read from the embedded filesystem first to bootstrap the user's local config.
func LoadTemplates() error {
	gscHome, err := GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	templatesDir := filepath.Join(gscHome, "data", "templates")

	// 1. Ensure templates directory exists
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	// 2. Bootstrap README.md if missing
	readmePath := filepath.Join(templatesDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		logger.Info("Bootstrapping README.md")
		if err := copyEmbeddedFile("templates/README.md", readmePath); err != nil {
			logger.Warning("Failed to write templates README.md", "error", err)
		}
	}

	// 3. Handle OS-specific JSON file
	osName := runtime.GOOS
	jsonFileName := fmt.Sprintf("templates.%s.json", osName)
	embeddedJsonPath := fmt.Sprintf("templates/%s", jsonFileName)
	localJsonPath := filepath.Join(templatesDir, jsonFileName)

	var config TemplateConfig

	// Check if local file exists
	if _, err := os.Stat(localJsonPath); os.IsNotExist(err) {
		// File doesn't exist, bootstrap it from embedded filesystem
		logger.Info("Bootstrapping template configuration", "file", jsonFileName)
		
		// Read from embedded FS
		data, err := templateFS.ReadFile(embeddedJsonPath)
		if err != nil {
			return fmt.Errorf("failed to read embedded template file %s: %w", embeddedJsonPath, err)
		}

		// Write to local FS
		if err := os.WriteFile(localJsonPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write template file: %w", err)
		}

		// Parse the data we just wrote
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse bootstrapped template file: %w", err)
		}
	} else {
		// File exists, load it from local FS (user might have customized it)
		data, err := os.ReadFile(localJsonPath)
		if err != nil {
			return fmt.Errorf("failed to read local template file: %w", err)
		}

		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse local template file: %w", err)
		}
	}

	// 4. Populate global maps
	if config.Editors != nil {
		DefaultEditorTemplates = config.Editors
	}
	if config.Terminals != nil {
		DefaultTerminalTemplates = config.Terminals
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
