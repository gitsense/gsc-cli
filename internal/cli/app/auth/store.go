/**
 * Component: Auth Code Store
 * Block-UUID: a4ec42e7-9241-4ec5-a99a-bb9badc6d62a
 * Parent-UUID: 5a5fd4d1-4693-4cd9-be5d-3140a0c941e6
 * Version: 1.0.2
 * Description: Handles file I/O operations for auth codes, including generation, atomic saving, loading, and deletion.
 * Language: Go
 * Created-at: 2026-05-21T01:28:17.441Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 2.5 Flash Lite (v1.0.2)
 */


package auth

import (
	"crypto/rand"
	"errors"
	"encoding/json"
	"math"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var ErrAuthCodeNotFound = errors.New("auth code not found")

// GenerateRandomCode generates a random 6-digit numeric string.
func GenerateRandomCode() (string, error) {
	max := big.NewInt(int64(math.Pow10(settings.AuthCodeLength)))
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate random code: %w", err)
	}
	
	// Format as 6-digit string with leading zeros
	return fmt.Sprintf("%06d", n.Int64()), nil
} 
// SaveCode creates an auth code JSON file atomically.
func SaveCode(code string, permissions []string, ttlMinutes int) error {
	// 1. Resolve GSC_HOME
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// 2. Ensure directory exists
	authDir := filepath.Join(gscHome, settings.AuthCodesRelPath)
	if err := os.MkdirAll(authDir, 0700); err != nil {
		return fmt.Errorf("failed to create auth directory: %w", err)
	}

	// 3. Prepare Data
	now := time.Now()
	data := AuthCodeData{
		Code:        code,
		Permissions: permissions,
		ExpiresAt:   now.Add(time.Duration(ttlMinutes) * time.Minute),
		CreatedAt:   now,
	}

	// 4. Marshal JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth code data: %w", err)
	}

	// 5. Atomic Write
	filePath := filepath.Join(authDir, code+".json")
	tmpPath := filePath + ".tmp"

	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write temp auth code file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to rename auth code file: %w", err)
	}

	logger.Info("Auth code saved", "code", code, "expires_at", data.ExpiresAt)
	return nil
}

// LoadCode reads and parses an auth code JSON file.
func LoadCode(code string) (*AuthCodeData, error) {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	filePath := filepath.Join(gscHome, settings.AuthCodesRelPath, code+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrAuthCodeNotFound
		}
		return nil, fmt.Errorf("failed to read auth code file: %w", err)
	}

	var authData AuthCodeData
	if err := json.Unmarshal(data, &authData); err != nil {
		return nil, fmt.Errorf("failed to parse auth code file: %w", err)
	}

	return &authData, nil
}

// DeleteCode removes an auth code JSON file.
func DeleteCode(code string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	filePath := filepath.Join(gscHome, settings.AuthCodesRelPath, code+".json")

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete auth code file: %w", err)
	}

	logger.Info("Auth code deleted", "code", code)
	return nil
}
