/*
 * Component: Settings
 * Block-UUID: df6117f7-d19c-4339-b421-f9cf8b123d71
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Package settings provides global configuration constants for the GSC CLI.
 * Language: Go
 * Created-at: 2026-02-02T05:47:30.456Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package settings

// GitSenseDir is the name of the directory where GitSense stores its data
const GitSenseDir = ".gitsense"

// RegistryFileName is the name of the file that tracks all manifests
const RegistryFileName = "manifest.json"

// DefaultDBExtension is the file extension for SQLite databases
const DefaultDBExtension = ".db"

// ManifestJSONExtension is the file extension for manifest dump files
const ManifestJSONExtension = ".json"
