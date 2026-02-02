/*
 * Component: Settings
 * Block-UUID: 3cb885e1-4d2e-4a30-97cc-319fd02e5684
 * Parent-UUID: df6117f7-d19c-4339-b421-f9cf8b123d71
 * Version: 1.1.0
 * Description: Package settings provides global configuration constants for the GSC CLI.
 * Language: Go
 * Created-at: 2026-02-02T05:47:30.456Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0)
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
