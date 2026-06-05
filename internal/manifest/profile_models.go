
/**
 * Component: Profile Models
 * Block Ministers: 1.4.0
 * Parent-UUID: 84e57fb0-d59e-48b8-8063-694bd2504902
 * Version: 1.4.0
 * Description: Defines the Go structs for Context Profiles, which represent named workspaces containing pre-defined configuration values. Added Scope field to GlobalSettings to support Focus Scope configuration within profiles. INTERNAL: These models support a feature currently hidden from the user interface to reduce complexity. The implementation is retained for potential future use.
 * Language: Go
 * Created-at: 2026-02-05T20:06:12.132Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), GLM-4.7 (v1.4.0)
 */


package manifest

// INTERNAL: Profile represents a named context profile.
// A profile is a collection of configuration settings that can be activated
// to switch the "workspace" context for the CLI.
// This feature is currently hidden from the user interface.
type Profile struct {
	Name        string          `json:"name"`        // The unique name of the profile (e.g., "security",", "payments")
	Description string          `json:"description"` // Human-readable description of the profile's purpose
	Aliases     []string        `json:"aliases"`     // Optional list of short aliases for quick access (e.g., ["sec", "pay"])
	Settings    ProfileSettings `json:"settings"`    // The configuration settings for this profile
}

// INTERNAL: ProfileSettings contains the configuration values for a specific profile.
// It is divided into global settings and command-specific settings.
type ProfileSettings struct {
	Global GlobalSettings `json:"global"` // Settings that apply globally (e.g., default database, scope)
	Query  QuerySettings  `json:"query"`  // Settings specific to the 'gsc query' command
	RG     RGSettings     `json:"rg"`     // Settings specific to the 'gsc rg' command
}

// INTERNAL: GlobalSettings contains configuration that applies across multiple commands.
type GlobalSettings struct {
	DefaultDatabase string       `json:"default_database"` // The default database to use for all commands
	Scope           *ScopeConfig `json:"scope"`            // The Focus Scope configuration for this profile
}

// INTERNAL: QuerySettings contains configuration specific to the 'gsc query' command.
type QuerySettings struct {
	DefaultField  string `json:"default_field"`  // The default field to query
	DefaultFormat string `json:"default_format"` // The default output format (table, json)
}

// INTERNAL: RGSettings contains configuration specific to the 'gsc rg' command.
type RGSettings struct {
	DefaultFormat  string   `json:"default_format"`  // The default output format (table, json)
	DefaultContext int      `json:"default_context"` // The default number of context lines to show
	DefaultFields  []string `json:"default_fields"`  // The default metadata fields to include in results
}
