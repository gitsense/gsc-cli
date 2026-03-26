/**
 * Component: Contract Types
 * Block-UUID: e6a0bef5-6d2b-409f-91b4-ebc048eba3cd
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the core data structures for contracts, including the WorkdirEntry and ContractData. Implements backward-compatible unmarshalling for the workdir to workdirs migration.
 * Language: Go
 * Created-at: 2026-03-26T15:15:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package contract

import (
	"encoding/json"
	"time"
)

// WorkdirEntry represents a working directory associated with a contract.
type WorkdirEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	AddedAt time.Time `json:"added_at"`
	Status  string    `json:"status"`
}

// ContractData defines the core schema for a contract, shared between the file system and the database.
type ContractData struct {
	Description string
	ExpiresAt   time.Time
	UUID        string
	Status      string
	Authcode    string

	// Security Fields
	Whitelist   []string
	NoWhitelist bool
	ExecTimeout int

	// Workspace Preferences
	PreferredEditor   string
	PreferredTerminal string
	PreferredReview   string

	// The new array field
	Workdirs []WorkdirEntry `json:"workdirs"`
}

// UnmarshalJSON handles backward compatibility for the workdir -> workdirs migration.
func (c *ContractData) UnmarshalJSON(data []byte) error {
	// Define an alias type to avoid infinite recursion
	type Alias ContractData

	// Define a temporary struct that captures both the old and new formats
	type TempContract struct {
		Workdir    string         `json:"workdir"` // Legacy field
		Workdirs   []WorkdirEntry `json:"workdirs"` // New field
		AliasFields Alias         `json:"-"`        // All other fields
	}

	var temp TempContract
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Copy alias fields
	*c = ContractData(temp.AliasFields)

	// Handle Migration
	if len(temp.Workdirs) > 0 {
		// New format: use the array directly
		c.Workdirs = temp.Workdirs
	} else if temp.Workdir != "" {
		// Legacy format: migrate single string to array
		c.Workdirs = []WorkdirEntry{
			{
				Name:    "primary", // Default name for legacy entries
				Path:    temp.Workdir,
				AddedAt: time.Now(), // We don't have the original added_at, so we use now
				Status:  "active",
			},
		}
	}

	return nil
}
