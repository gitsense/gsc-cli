/*
 * Component: Profile Models
 * Block-UUID: f0b6dfdc-3c3a-433a-bb20-32c1b40b3160
 * Parent-UUID: be2507ae-bb5c-4b4f-91bf-b6ec5c4fa5ba
 * Version: 1.1.0
 * Description: Defines the Go structs for Context Profiles, which represent named workspaces containing pre-defined configuration values. Added Aliases field to support ergonomic profile switching.
 * Language: Go
 * Created-at: 2026-02-03T01:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package manifest

// Profile represents a named context profile.
// A profile is a collection of configuration settings that can be activated
// to switch the "workspace" context for the CLI.
type Profile struct {
	Name        string          `json:"name"`        // The unique name of the profile (e.g., "security", "payments")
	Description string          `json:"description"` // Human-readable description of the profile's purpose
	Aliases     []string        `json:"aliases"`     // Optional list of short aliases for quick access (e.g., ["sec", "pay"])
	Settings    ProfileSettings `json:"settings"`    // The configuration settings for this profile
}

// ProfileSettings contains the configuration values for a specific profile.
// It is divided into global settings and command-specific settings.
type ProfileSettings struct {
	Global GlobalSettings `json:"global"` // Settings that apply globally (e.g., default database)
	Query  QuerySettings  `json:"query"`  // Settings specific to the 'gsc query' command
	RG     RGSettings     `json:"rg"`     // Settings specific to the 'gsc rg' command
}

// GlobalSettings contains configuration that applies across multiple commands.
type GlobalSettings struct {
	DefaultDatabase string `json:"default_database"` // The default database to use for all commands
}

// QuerySettings contains configuration specific to the 'gsc query' command.
type QuerySettings struct {
	DefaultField  string `json:"default_field"`  // The default field to query
	DefaultFormat string `json:"default_format"` // The default output format (table, json)
}

// RGSettings contains configuration specific to the 'gsc rg' command.
type RGSettings struct {
	DefaultFormat  string `json:"default_format"`  // The default output format (table, json)
	DefaultContext int    `json:"default_context"` // The default number of context lines to show
}
