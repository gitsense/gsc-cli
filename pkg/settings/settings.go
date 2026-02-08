/**
 * Component: Settings
 * Block-UUID: 63907862-1f4b-484c-ac50-593567964dd5
 * Parent-UUID: 253896fb-1af3-4728-a310-9ccfaa2fe69f
 * Version: 1.3.0
 * Description: Package settings provides global configuration constants for the GSC CLI. Added constants for backup directory, temporary database suffix, and maximum backup retention count.
 * Language: Go
 * Created-at: 2026-02-08T06:37:58.511Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0)
 */


package settings

// DefaultGitSenseDir is the default name of the directory where GitSense stores its data
const DefaultGitSenseDir = ".gitsense"

// GitSenseDir is the name of the directory where GitSense stores its data
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

