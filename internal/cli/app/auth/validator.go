/**
 * Component: Auth Code Validator
 * Block-UUID: 5ac70cbf-dee1-4970-8a2b-901eec73c80f
 * Parent-UUID: 1215c942-9e1d-4cca-85de-8984372203f7
 * Version: 1.0.1
 * Description: Implements the validation logic for auth codes, checking existence, expiry, and permissions.
 * Language: Go
 * Created-at: 2026-05-21T01:29:17.210Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 2.5 Flash Lite (v1.0.1)
 */


package auth

import (
	"errors"
	"fmt"
	"time"
)

// ValidateCode checks if an auth code is valid, not expired, and has the required permission.
func ValidateCode(code string, requiredPermission string) (*ValidationResult, error) {
	// 1. Load Code
	data, err := LoadCode(code)
	if err != nil {
		if errors.Is(err, ErrAuthCodeNotFound) {
			// File not found
			return &ValidationResult{
				Valid: false,
				Code:  code,
				Status: StatusNotFound,
				Error: "invalid or not found",
			}, nil
		}
		// Unexpected error (e.g., permission denied, corrupt JSON)
		return nil, fmt.Errorf("failed to load auth code: %w", err)
	}

	// 2. Check Expiry
	if time.Now().After(data.ExpiresAt) {
		secondsExpired := int(time.Since(data.ExpiresAt).Seconds())
		return &ValidationResult{
			Valid:       false,
			Code:        code,
			Status:      StatusExpired,
			ExpiresAt:   data.ExpiresAt,
			Permissions: data.Permissions,
			Error:       fmt.Sprintf("code expired %d seconds ago", secondsExpired),
		}, nil
	}

	// 3. Check Permissions (if required)
	if requiredPermission != "" {
		hasPermission := false
		for _, p := range data.Permissions {
			if p == requiredPermission {
				hasPermission = true
				break
			}
		}
		if !hasPermission {
			return &ValidationResult{
				Valid:              false,
				Code:               code,
				Status:             StatusPermissionDenied,
				RequiredPermission: requiredPermission,
				Permissions:        data.Permissions,
				Error:              "missing required permission",
			}, nil
		}
	}

	// 4. Success
	return &ValidationResult{
		Valid:       true,
		Code:        code,
		Status:      StatusActive,
		ExpiresAt:   data.ExpiresAt,
		Permissions: data.Permissions,
	}, nil
}
