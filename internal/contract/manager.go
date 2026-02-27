/**
 * Component: Contract Manager
 * Block-UUID: bc99e7a5-7dcf-40fd-9103-7cabb3d42a64
 * Parent-UUID: 37c524cf-fb0b-4180-90ba-e296e23fa3c4
 * Version: 1.3.2
 * Description: Improved CreateContract logic to prevent multiple active contracts for the same workspace and implemented chat idempotency by using UpsertContractMessage.
 * Language: Go
 * Created-at: 2026-02-27T17:01:48.830Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), Gemini 3 Flash (v1.0.7), GLM-4.7 (v1.0.8), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/google/uuid"
)

// CreateContract initializes a new traceability contract using a valid handshake code.
// It validates the workdir, persists the contract metadata, and inserts the contract message into the chat.
func CreateContract(code string, description string, authcode string, workdir string) (*ContractMetadata, error) {
	// 1. Resolve GSC_HOME and Load Handshake
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	h, err := bridge.LoadHandshake(gscHome, code, bridge.StageExecution)
	if err != nil {
		return nil, fmt.Errorf("failed to load handshake: %w", err)
	}

	// 2. Validate Workdir and Check for Existing Active Contracts
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for workdir: %w", err)
	}

	if _, err := git.FindGitRootFrom(absWorkdir); err != nil {
		return nil, fmt.Errorf("invalid workdir: %w", err)
	}

	// Check if an active contract already exists for this workspace
	if existing, err := GetContractByWorkdir(absWorkdir); err == nil && existing != nil {
		return nil, fmt.Errorf("an active contract already exists for this workspace: %s. Please cancel it before creating a new one", existing.UUID)
	}

	// Claim the handshake immediately to prevent double-use
	if err := h.UpdateStatus("running", nil); err != nil {
		return nil, fmt.Errorf("failed to claim handshake: %w", err)
	}

	// 3. Generate Metadata
	now := time.Now()
	meta := &ContractMetadata{
		UUID:        uuid.New().String(),
		Authcode:    authcode,
		Description: description,
		Workdir:     absWorkdir,
		ChatID:      h.ChatID,
		ContractMessageID: 0, // Will be set after DB insertion
		ChatUUID:    h.ChatUUID,
		Status:      ContractActive,
		CreatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(settings.DefaultContractTTL) * time.Hour),
	}

	// 4. Persist JSON
	if err := saveContractMetadata(meta); err != nil {
		return nil, fmt.Errorf("failed to save contract metadata: %w", err)
	}

	// 5. Upsert DB Message (Idempotency)
	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		// Rollback: Remove the JSON file if DB connection fails
		_ = os.Remove(getContractPath(meta.UUID))
		return nil, fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      string(meta.Status),
	}

	// Use UpsertContractMessage to ensure only one contract message exists per chat
	contractMsgID, err := db.UpsertContractMessage(sqliteDB, h.ChatID, dbData)
	if err != nil {
		// Rollback: Remove the JSON file if DB insertion fails
		_ = os.Remove(getContractPath(meta.UUID))
		// Mark handshake as failed if DB insertion fails
		_ = h.UpdateStatus("error", &bridge.Error{Code: "CONTRACT_FAILED", Message: err.Error()})
		return nil, fmt.Errorf("failed to upsert contract message: %w", err)
	}

	meta.ContractMessageID = contractMsgID

	// Persist the updated metadata (specifically the ContractMessageID) to disk
	if err := saveContractMetadata(meta); err != nil {
		// Log warning but don't fail the contract creation, as the DB record exists.
		logger.Warning("Failed to update contract metadata file with message ID", "error", err)
	}

	logger.Info("Contract created successfully", "uuid", meta.UUID, "expires_at", meta.ExpiresAt)

	// Mark handshake as successfully consumed
	if err := h.UpdateStatus("success", nil); err != nil {
		logger.Warning("Failed to mark handshake as consumed", "error", err)
	}
	return meta, nil
}

// ListContracts retrieves all contracts from the global storage.
// It performs a lazy expiration check, updating the status in memory if a contract has expired.
func ListContracts() ([]ContractMetadata, error) {
	contractDir, err := manifest.ResolveGlobalContractDir()
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(contractDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan contracts directory: %w", err)
	}

	var contracts []ContractMetadata
	now := time.Now()

	for _, file := range files {
		meta, err := loadContractMetadata(file)
		if err != nil {
			logger.Warning("Failed to load contract file", "file", file, "error", err)
			continue
		}

		// Lazy Expiration Check
		if meta.Status == ContractActive && now.After(meta.ExpiresAt) {
			meta.Status = ContractExpired
			// Note: We do not persist this change to disk here to keep ListContracts read-only.
			// The status will be updated if the user attempts to renew or if we add a cleanup job.
		}

		contracts = append(contracts, *meta)
	}

	return contracts, nil
}

// GetContract retrieves a specific contract by its UUID.
func GetContract(uuid string) (*ContractMetadata, error) {
	path := getContractPath(uuid)
	return loadContractMetadata(path)
}

// GetContractByWorkdir retrieves the active contract for a specific working directory.
// It returns an error if no active contract is found or if multiple active contracts exist for the same directory.
func GetContractByWorkdir(workdir string) (*ContractMetadata, error) {
	// Resolve to absolute path to ensure consistency
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	contracts, err := ListContracts()
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}

	var matches []ContractMetadata
	for _, c := range contracts {
		if c.Status == ContractActive && c.Workdir == absWorkdir {
			matches = append(matches, c)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no active contract found for directory: %s", absWorkdir)
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple active contracts found for directory: %s. Please specify a UUID", absWorkdir)
	}

	return &matches[0], nil
}

// GetContractInfo retrieves contract information for the 'info' command.
// It applies sanitization rules if requested (e.g., converting absolute paths to relative).
func GetContractInfo(uuid string, sanitize bool) (*ContractInfoResult, error) {
	meta, err := GetContract(uuid)
	if err != nil {
		return nil, err
	}

	result := &ContractInfoResult{
		UUID:        meta.UUID,
		Description: meta.Description,
		Status:      string(meta.Status),
		CreatedAt:   meta.CreatedAt,
		ExpiresAt:   meta.ExpiresAt,
		Authcode:    meta.Authcode,
		Workdir:     meta.Workdir,
	}

	if sanitize {
		// Try to make the workdir relative to the current working directory
		cwd, err := os.Getwd()
		if err == nil {
			relPath, err := filepath.Rel(cwd, meta.Workdir)
			if err == nil {
				result.Workdir = relPath
			} else {
				// Fallback to basename if relative path calculation fails
				result.Workdir = filepath.Base(meta.Workdir)
			}
		} else {
			// Fallback to basename if CWD is unavailable
			result.Workdir = filepath.Base(meta.Workdir)
		}
		
		// Mask authcode if sanitizing output
		result.Authcode = "****"
	}

	return result, nil
}

// CancelContract terminates a contract immediately.
// It updates the local JSON file and the corresponding message in the Chat database.
func CancelContract(uuid string) error {
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status == ContractCancelled {
		return fmt.Errorf("contract %s is already cancelled", uuid)
	}

	// Update Status
	meta.Status = ContractCancelled

	// Persist JSON
	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to update contract metadata: %w", err)
	}

	// Update DB Message
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      string(meta.Status),
	}

	if err := db.UpdateContractMessage(sqliteDB, meta.ContractMessageID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	logger.Info("Contract cancelled", "uuid", uuid)
	return nil
}

// RenewContract extends the expiration time of an active or expired contract.
// It updates the local JSON file and the corresponding message in the Chat database.
func RenewContract(uuid string, hours int) error {
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status == ContractCancelled {
		return fmt.Errorf("cannot renew a cancelled contract")
	}

	// Calculate new expiration
	duration := time.Duration(hours) * time.Hour
	newStart := time.Now()
	if meta.ExpiresAt.After(newStart) {
		newStart = meta.ExpiresAt
	}
	meta.ExpiresAt = newStart.Add(duration)

	// Update Status if it was expired
	if meta.Status == ContractExpired {
		meta.Status = ContractActive
	}

	// Persist JSON
	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to update contract metadata: %w", err)
	}

	// Update DB Message
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      string(meta.Status),
	}

	if err := db.UpdateContractMessage(sqliteDB, meta.ContractMessageID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	logger.Info("Contract renewed", "uuid", uuid, "new_expires_at", meta.ExpiresAt)
	return nil
}

// saveContractMetadata performs an atomic write of the contract metadata to disk.
func saveContractMetadata(meta *ContractMetadata) error {
	contractDir, err := manifest.ResolveGlobalContractDir()
	if err != nil {
		return err
	}

	path := filepath.Join(contractDir, meta.UUID+".json")
	tmpPath := path + ".tmp"

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contract metadata: %w", err)
	}

	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp contract file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename contract file: %w", err)
	}

	return nil
}

// loadContractMetadata reads and unmarshals a contract metadata file.
func loadContractMetadata(path string) (*ContractMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract file: %w", err)
	}

	var meta ContractMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract metadata: %w", err)
	}

	return &meta, nil
}

// getContractPath returns the absolute path to a contract file.
func getContractPath(uuid string) string {
	contractDir, _ := manifest.ResolveGlobalContractDir()
	return filepath.Join(contractDir, uuid+".json")
}

// FormatContractInfo formats the output for the 'contract info' command.
func FormatContractInfo(info *ContractInfoResult, format string) string {
	if format == "json" {
		bytes, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Sprintf("Error formatting JSON: %v\n", err)
		}
		return string(bytes)
	}

	var sb strings.Builder
	sb.WriteString("Contract Information:\n")
	sb.WriteString(fmt.Sprintf("  UUID:         %s\n", info.UUID))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", info.Status))
	sb.WriteString(fmt.Sprintf("  Description:  %s\n", info.Description))
	sb.WriteString(fmt.Sprintf("  Authcode:     %s\n", info.Authcode))
	sb.WriteString(fmt.Sprintf("  Workdir:      %s\n", info.Workdir))
	sb.WriteString(fmt.Sprintf("  Created At:   %s\n", info.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Expires At:   %s\n", info.ExpiresAt.Format(time.RFC3339)))
	
	return sb.String()
}

// FormatContractTest formats the output for the 'contract test' command.
func FormatContractTest(result *ContractTestResult, format string) string {
	if format == "json" {
		bytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Sprintf("Error formatting JSON: %v\n", err)
		}
		return string(bytes)
	}

	var sb strings.Builder

	// 1. Contract Info Section
	sb.WriteString("Contract Information:\n")
	sb.WriteString(fmt.Sprintf("  UUID:         %s\n", result.UUID))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", result.Status))
	sb.WriteString(fmt.Sprintf("  Expires At:   %s\n", result.ExpiresAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Workdir:      %s\n", result.Workdir))
	sb.WriteString("\n")

	// 2. Test Result Section
	sb.WriteString("Contract Test Result:\n")
	statusStr := "Success"
	if !result.Success {
		statusStr = "Failed"
	}
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", statusStr))
	
	if result.ErrorCode != "" {
		sb.WriteString(fmt.Sprintf("  Error Code:   %s\n", result.ErrorCode))
	}
	
	sb.WriteString(fmt.Sprintf("  Message:      %s\n", result.Message))
	sb.WriteString("\n")

	// 3. File Details (if applicable)
	if result.RelativePath != "" {
		sb.WriteString(fmt.Sprintf("  File:         %s\n", result.RelativePath))
	}
	
	if result.BlockUUID != "" {
		sb.WriteString(fmt.Sprintf("  Block-UUID: %s\n", result.BlockUUID))
	}

	if result.ParentUUID != "" {
		sb.WriteString(fmt.Sprintf("  Parent-UUID: %s\n", result.ParentUUID))
	}

	if result.RelativePath != "" || result.BlockUUID != "" {
		sb.WriteString("\n")
	}

	// 4. Diff Section (if applicable)
	if result.DiffUnified != "" {
		sb.WriteString("Diff:\n")
		sb.WriteString(result.DiffUnified)
	}

	return sb.String()
}
