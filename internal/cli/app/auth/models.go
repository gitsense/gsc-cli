/**
 * Component: Auth Code Models
 * Block-UUID: 44cf2f45-f401-4ec7-b451-a3b4f6f292f4
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the data structures for auth codes and validation results used by the 'gsc app auth' command suite.
 * Language: Go
 * Created-at: 2026-05-20T15:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package auth

import "time"

// AuthCodeData represents the JSON file content for an auth code.
// It is stored in GSC_HOME/data/auth/<code>.json
type AuthCodeData struct {
	Code        string    `json:"code"`
	Permissions []string  `json:"permissions"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// ValidationResult represents the JSON output for the validate command.
// It provides structured data for the backend to parse and display to the user.
type ValidationResult struct {
	Valid              bool      `json:"valid"`
	Code               string    `json:"code"`
	Status             string    `json:"status"`
	ExpiresAt          time.Time `json:"expires_at,omitempty"`
	Permissions        []string  `json:"permissions,omitempty"`
	RequiredPermission string    `json:"required_permission,omitempty"`
	Error              string    `json:"error,omitempty"`
}

// Status constants for ValidationResult
const (
	StatusActive           = "active"
	StatusExpired          = "expired"
	StatusNotFound         = "not_found"
	StatusPermissionDenied = "permission_denied"
)
