/**
 * Component: Contract Manager
 * Block-UUID: d188da85-f7eb-4503-afcc-86dee9d5e8c1
 * Parent-UUID: b2a5d928-1eac-4baf-9818-d0235f41c8e6
 * Version: 1.0.4
 * Description: Added GetContractByWorkdir to support the 'status' command. This function retrieves the active contract for a specific working directory, handling edge cases for zero or multiple matches.
 * Language: Go
 * Created-at: 2026-02-26T05:53:17.561Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
func CreateContract(code string, description string, workdir string) (*ContractMetadata, error) {
	// 1. Resolve GSC_HOME and Load Handshake
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	h, err := bridge.LoadHandshake(gscHome, code, bridge.StageExecution)
	if err != nil {
		return nil, fmt.Errorf("failed to load handshake: %w", err)
	}

	// Claim the handshake immediately to prevent double-use
	if err := h.UpdateStatus("running", nil); err != nil {
		return nil, fmt.Errorf("failed to claim handshake: %w", err)
	}

	// 2. Validate Workdir
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for workdir: %w", err)
	}

	if _, err := git.FindGitRootFrom(absWorkdir); err != nil {
		return nil, fmt.Errorf("invalid workdir: %w", err)
	}

	// 3. Generate Metadata
	now := time.Now()
	meta := &ContractMetadata{
		UUID:        uuid.New().String(),
		Description: description,
		Workdir:     absWorkdir,
		ChatID:      h.ChatID,
		ContractMessageID: 0, // Will be set after DB insertion
		ChatUUID:    h.ChatUUID,
		Status:      ContractActive,
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}

	// 4. Persist JSON
	if err := saveContractMetadata(meta); err != nil {
		return nil, fmt.Errorf("failed to save contract metadata: %w", err)
	}

	// 5. Insert DB Message
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

	contractMsgID, err := db.InsertContractWithAnchor(sqliteDB, h.ChatID, dbData)
	if err != nil {
		// Rollback: Remove the JSON file if DB insertion fails
		_ = os.Remove(getContractPath(meta.UUID))
		// Mark handshake as failed if DB insertion fails
		_ = h.UpdateStatus("error", &bridge.Error{Code: "CONTRACT_FAILED", Message: err.Error()})
		return nil, fmt.Errorf("failed to insert contract message: %w", err)
	}

	meta.ContractMessageID = contractMsgID
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
	// We assume ResolveGlobalContractDir succeeds or we handle the error in the caller.
	// For simplicity here, we construct the path assuming the dir exists.
	gscHome, _ := settings.GetGSCHome(false)
	return filepath.Join(gscHome, settings.ContractsRelPath, uuid+".json")
}
