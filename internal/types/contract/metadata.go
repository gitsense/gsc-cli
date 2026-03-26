/**
 * Component: Contract Types
 * Block-UUID: a5116024-48a9-4153-b3c3-30707648be78
 * Parent-UUID: 57aa5d85-9a96-486f-a76a-ccf62f9ad4e6
 * Version: 1.4.0
 * Description: Defines the core data structures for contracts, including the WorkdirEntry and ContractData. Implements backward-compatible unmarshalling for the workdir to workdirs migration.
 * Language: Go
 * Created-at: 2026-03-26T17:04:38.216Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0)
 */


package contract

import (
	"time"
	"encoding/json"
)

// ContractStatus defines the lifecycle state of a contract.
type ContractStatus string

const (
	ContractActive    ContractStatus = "active"
	ContractCancelled ContractStatus = "cancelled"
	ContractExpired   ContractStatus = "expired"
	ContractDone      ContractStatus = "done"
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
	Status      ContractStatus
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
		Workdir  string         `json:"workdir"`
		Workdirs []WorkdirEntry `json:"workdirs"`
		*Alias                 // Embed pointer to receiver
	}

	temp := &TempContract{ Alias: (*Alias)(c) }
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Handle Migration
	// Note: temp.Workdirs is actually pointing to c.Workdirs because of the embedding
	if len(temp.Workdirs) > 0 {
		// New format: array is populated in temp.Workdirs due to shadowing
		// We must explicitly copy it to c.Workdirs
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
