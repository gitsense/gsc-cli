/*
 * Component: Settings
 * Block-UUID: 253896fb-1af3-4728-a310-9ccfaa2fe69f
 * Parent-UUID: 3cb885e1-4d2e-4a30-97cc-319fd02e5684
 * Version: 1.2.0
 * Description: Package settings provides global configuration constants for the GSC CLI. Added constants for backup directory, temporary database suffix, and maximum backup retention count.
 * Language: Go
 * Created-at: 2026-02-02T05:47:30.456Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0)
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
