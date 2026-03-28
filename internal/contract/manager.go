/**
 * Component: Contract Manager
 * Block-UUID: f8f9b4cf-51c6-4b44-b563-766dd0ccf11b
 * Parent-UUID: 8edc98d0-beec-47aa-bb62-74a7d55c0994
 * Version: 1.24.5
 * Description: Add workdir management methods (AddWorkdir, RemoveWorkdir, SetPrimaryWorkdir) with conflict validation and event notifications.
 * Language: Go
 * Created-at: 2026-03-28T16:24:31.160Z
 * Authors: GLM-4.7 (v1.24.0), GLM-4.7 (v1.24.1), GLM-4.7 (v1.24.2), GLM-4.7 (v1.24.3), GLM-4.7 (v1.24.4), GLM-4.7 (v1.24.5)
 */


package contract

import (
	typescontract "github.com/gitsense/gsc-cli/internal/types/contract"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/bridge"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// CreateContract initializes a new traceability contract using a valid handshake code.
// It validates the workdir, persists the contract metadata, and inserts the contract message into the chat.
func CreateContract(code string, description string, authcode string, workdir string, whitelist []string, noWhitelist bool, execTimeout int, preferredEditor string, preferredTerminal string, preferredReview string) (*ContractMetadata, error) {
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
		return nil, fmt.Errorf("invalid workdir: not a git repository (directory must contain a .git folder)")
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
	
	expiresAt := now.Add(time.Duration(settings.DefaultContractTTL) * time.Hour)

	// Apply DefaultSafeSet if no whitelist is provided and restrictions are enabled
	effectiveWhitelist := whitelist
	if (effectiveWhitelist == nil || len(effectiveWhitelist) == 0) && !noWhitelist {
		effectiveWhitelist = settings.DefaultSafeSet
	}
	
	meta := &ContractMetadata{
		ContractData: typescontract.ContractData{
			UUID:        uuid.New().String(),
			Workdirs: []typescontract.WorkdirEntry{
				{Name: "primary", Path: absWorkdir, AddedAt: now, Status: "active"},
			},
			Authcode:    authcode,
			Description: description,
			Status:      typescontract.ContractActive,
			ExpiresAt:   expiresAt,
			Whitelist:   effectiveWhitelist,
			NoWhitelist: noWhitelist,
			ExecTimeout: execTimeout,
			PreferredEditor:   preferredEditor,
			PreferredTerminal: preferredTerminal,
			PreferredReview:   preferredReview,
		},
		ChatID:            h.ChatID,
		ContractMessageID: 0, // Will be set after DB insertion
		ChatUUID:          h.ChatUUID,
		CreatedAt:         now,
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

	dbData := db.ContractMessageData{ContractData: meta.ContractData}

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

	// 6. Initialize Events Database (Skeleton)
	if err := initEventsDB(meta.UUID); err != nil {
		logger.Warning("Failed to initialize events database", "error", err)
	}

	// Mark handshake as successfully consumed
	if err := h.UpdateStatus("success", nil); err != nil {
		logger.Warning("Failed to mark handshake as consumed", "error", err)
	}
	return meta, nil
}

// initEventsDB creates the skeleton structure for the contract-level messaging database.
func initEventsDB(uuid string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	dbPath := filepath.Join(gscHome, settings.HomesRelPath, uuid, "events.sqlite3")
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("failed to create events DB directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open events DB: %w", err)
	}
	defer db.Close()

	// Create schema
	if _, err := db.Exec(EventsDBSchema); err != nil {
		return fmt.Errorf("failed to create events table: %w", err)
	}

	logger.Info("Events database initialized", "uuid", uuid, "path", dbPath)
	return nil
}

// ListContracts retrieves all contracts from the global storage.
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
		if meta.Status == typescontract.ContractActive && now.After(meta.ExpiresAt) {
			meta.Status = typescontract.ContractExpired
			logger.Debug("Contract marked as expired due to time", "uuid", meta.UUID, "expires_at", meta.ExpiresAt)
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
func GetContractByWorkdir(workdir string) (*ContractMetadata, error) {
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	logger.Debug("GetContractByWorkdir called", "target_workdir", absWorkdir)
	contracts, err := ListContracts()
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}

	var matches []ContractMetadata
	for _, c := range contracts {
		logger.Debug("Evaluating contract", "uuid", c.UUID, "status", c.Status)
		if c.Status == typescontract.ContractActive {
			logger.Debug("Contract is active, checking workdirs", "uuid", c.UUID, "workdir_count", len(c.Workdirs))
			// Check if the current directory is inside or equal to any of the contract's workdirs
			for _, w := range c.Workdirs {
				logger.Debug("Comparing against workdir", "contract_uuid", c.UUID, "workdir_path", w.Path)
				rel, err := filepath.Rel(w.Path, absWorkdir)
				if err != nil {
					// Paths are on different drives or invalid, skip this workdir
					logger.Debug("Path comparison error (different drives?)", "error", err)
					continue
				}
				logger.Debug("Relative path calculated", "contract_uuid", c.UUID, "workdir_path", w.Path, "relative_path", rel)
				// If the relative path does not start with "..", we are inside the workdir
				if !strings.HasPrefix(rel, "..") {
					logger.Debug("Match found!", "contract_uuid", c.UUID, "workdir_path", w.Path)
					matches = append(matches, c)
					break // Found a match for this contract, stop checking other workdirs
				}
			}
		} else {
			logger.Debug("Contract skipped (not active)", "uuid", c.UUID, "status", c.Status)
		}
	}

	if len(matches) == 0 {
		logger.Debug("No matches found", "target_workdir", absWorkdir)
		return nil, fmt.Errorf("no active contract found for directory: %s", absWorkdir)
	}

	if len(matches) > 1 {
		logger.Debug("Multiple matches found", "target_workdir", absWorkdir, "count", len(matches))
		return nil, fmt.Errorf("multiple active contracts found for directory: %s. Please specify a UUID", absWorkdir)
	}

	logger.Debug("Returning single match", "uuid", matches[0].UUID)
	return &matches[0], nil
}

// GetContractInfo retrieves contract information for the 'info' command.
func GetContractInfo(uuid string, sanitize bool) (*ContractInfoResult, error) {
	meta, err := GetContract(uuid)
	if err != nil {
		return nil, err
	}

	status := string(meta.Status)
	if meta.Status != typescontract.ContractCancelled && meta.Status == typescontract.ContractActive && time.Now().After(meta.ExpiresAt) {
		status = string(typescontract.ContractExpired)
	}

	result := &ContractInfoResult{
		UUID:           meta.UUID,
		PrimaryWorkdir: meta.Workdirs[0].Path,
		Workdirs:       meta.Workdirs,
		Description:    meta.Description,
		Status:         status,
		CreatedAt:      meta.CreatedAt,
		ExpiresAt:      meta.ExpiresAt,
		Authcode:       meta.Authcode,
		ExecTimeout:    meta.ExecTimeout,
		Whitelist:      meta.Whitelist,
		NoWhitelist:    meta.NoWhitelist,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if sanitize {
		cwd, err := os.Getwd()
		if err == nil {
			relPath, err := filepath.Rel(cwd, meta.Workdirs[0].Path)
			if err == nil {
				result.PrimaryWorkdir = relPath
			} else {
				result.PrimaryWorkdir = filepath.Base(meta.Workdirs[0].Path)
			}
		} else {
			result.PrimaryWorkdir = filepath.Base(meta.Workdirs[0].Path)
		}
		result.Authcode = "****"
	}

	return result, nil
}

// CancelContract terminates a contract immediately.
func CancelContract(uuid string) error {
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status == typescontract.ContractCancelled {
		return fmt.Errorf("contract %s is already cancelled", uuid)
	}

	meta.Status = typescontract.ContractCancelled

	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to update contract metadata: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	// Notify Frontend via Event
	payload := ContractChangePayload{
		Status: string(typescontract.ContractCancelled),
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Contract cancelled", "uuid", uuid)
	return nil
}

// CompleteContract marks a contract as finished/done.
// This state prevents further edits but preserves the contract for historical reference and dumping.
func CompleteContract(uuid string) error {
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status == typescontract.ContractDone {
		return fmt.Errorf("contract %s is already marked as done", uuid)
	}

	meta.Status = typescontract.ContractDone

	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to update contract metadata: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	// Notify Frontend via Event
	payload := ContractChangePayload{
		Status: string(typescontract.ContractDone),
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Contract marked as done", "uuid", uuid)
	return nil
}

// RenewContract extends the expiration time of an active or expired contract.
func RenewContract(uuid string, hours int) error {
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status == typescontract.ContractCancelled {
		return fmt.Errorf("cannot renew a cancelled contract")
	}

	duration := time.Duration(hours) * time.Hour
	newStart := time.Now()
	if meta.ExpiresAt.After(newStart) {
		newStart = meta.ExpiresAt
	}
	meta.ExpiresAt = newStart.Add(duration)

	if meta.Status == typescontract.ContractExpired || meta.Status == typescontract.ContractDone {
		meta.Status = typescontract.ContractActive
	}

	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to update contract metadata: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	// Notify Frontend via Event
	payload := ContractChangePayload{
		Status:    string(typescontract.ContractActive),
		ExpiresAt: meta.ExpiresAt.Format(time.RFC3339),
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Contract renewed", "uuid", uuid, "new_expires_at", meta.ExpiresAt)
	return nil
}

// DeleteContract removes a contract.
func DeleteContract(uuid string) error {
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	// Confirmation Prompt
	confirm := false
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Are you sure you want to delete contract '%s' and all associated data (including shadow workspaces)?", meta.UUID),
		Default: false,
	}
	if err := survey.AskOne(prompt, &confirm); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	if !confirm {
		fmt.Println("Delete operation cancelled.")
		return nil
	}

	path := getContractPath(uuid)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete contract file: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}
	dbData.Status = "deleted"

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		logger.Warning("Failed to update contract message in database", "error", err)
	}

	// Notify Frontend via Event
	payload := ContractChangePayload{
		Status: "deleted",
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Contract deleted", "uuid", uuid)
	return nil
}

// AddWorkdir adds a new secondary working directory to an active contract.
// Validates path is a git repository, checks for duplicates, and enforces conflict constraints.
func AddWorkdir(uuid, path, name string) error {
	// 1. Load contract
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status != typescontract.ContractActive {
		return fmt.Errorf("contract must be active to add workdirs. Current status: %s", meta.Status)
	}

	// 2. Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// 3. Validate path is a git repository
	if _, err := git.FindGitRootFrom(absPath); err != nil {
		return fmt.Errorf("invalid workdir: %w", err)
	}

	// 4. Default name to basename if not provided
	if name == "" {
		name = filepath.Base(absPath)
	}

	// 5. Check for duplicates (by path)
	for _, w := range meta.Workdirs {
		if w.Path == absPath {
			return fmt.Errorf("workdir already exists in contract: %s", absPath)
		}
	}

	// 6. Check for duplicates (by name)
	for _, w := range meta.Workdirs {
		if w.Name == name {
			return fmt.Errorf("workdir with name '%s' already exists in contract", name)
		}
	}

	// 7. Check if path is already primary of another active contract
	contracts, err := ListContracts()
	if err != nil {
		return fmt.Errorf("failed to list contracts for conflict check: %w", err)
	}
	for _, other := range contracts {
		if other.UUID != meta.UUID && other.Status == typescontract.ContractActive {
			if len(other.Workdirs) > 0 && other.Workdirs[0].Path == absPath {
				return fmt.Errorf("directory is already primary in another active contract: %s", other.UUID)
			}
		}
	}

	// 8. Append new workdir
	meta.Workdirs = append(meta.Workdirs, typescontract.WorkdirEntry{
		Name:    name,
		Path:    absPath,
		AddedAt: time.Now(),
		Status:  "active",
	})

	// 9. Persist changes
	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to save contract metadata: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}
	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	// 10. Notify Frontend via Event
	payload := ContractChangePayload{
		Status: string(typescontract.ContractActive),
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Workdir added to contract", "uuid", uuid, "name", name, "path", absPath)
	return nil
}

// RemoveWorkdir removes a secondary working directory from an active contract.
// Prevents removal of the primary workdir (index 0).
func RemoveWorkdir(uuid, name string) error {
	// 1. Load contract
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status != typescontract.ContractActive {
		return fmt.Errorf("contract must be active to remove workdirs. Current status: %s", meta.Status)
	}

	// 2. Find workdir by name
	foundIndex := -1
	for i, w := range meta.Workdirs {
		if w.Name == name {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		var names []string
		for i, w := range meta.Workdirs {
			if i == 0 {
				names = append(names, w.Name+" (primary)")
			} else {
				names = append(names, w.Name)
			}
		}
		return fmt.Errorf("workdir '%s' not found. Available: %v", name, names)
	}

	// 3. Prevent removal of primary workdir
	if foundIndex == 0 {
		return fmt.Errorf("cannot remove primary workdir. Use set-primary-workdir to change the primary first")
	}

	// 4. Remove from slice
	meta.Workdirs = append(meta.Workdirs[:foundIndex], meta.Workdirs[foundIndex+1:]...)

	// 5. Persist changes
	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to save contract metadata: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}
	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	// 6. Notify Frontend via Event
	payload := ContractChangePayload{
		Status: string(typescontract.ContractActive),
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Workdir removed from contract", "uuid", uuid, "name", name)
	return nil
}

// SetPrimaryWorkdir changes the primary workdir by swapping with the workdir at index 0.
// Validates no conflicts with other active contracts.
func SetPrimaryWorkdir(uuid, name string) error {
	// 1. Load contract
	meta, err := GetContract(uuid)
	if err != nil {
		return err
	}

	if meta.Status != typescontract.ContractActive {
		return fmt.Errorf("contract must be active to change primary workdir. Current status: %s", meta.Status)
	}

	// 2. Find workdir by name
	foundIndex := -1
	for i, w := range meta.Workdirs {
		if w.Name == name {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		var names []string
		for i, w := range meta.Workdirs {
			if i == 0 {
				names = append(names, w.Name+" (primary)")
			} else {
				names = append(names, w.Name)
			}
		}
		return fmt.Errorf("workdir '%s' not found. Available: %v", name, names)
	}

	// 3. Check if already primary
	if foundIndex == 0 {
		logger.Info("Workdir is already primary", "uuid", uuid, "name", name)
		return nil
	}

	// 4. Check if target path is already primary of another active contract
	targetPath := meta.Workdirs[foundIndex].Path
	contracts, err := ListContracts()
	if err != nil {
		return fmt.Errorf("failed to list contracts for conflict check: %w", err)
	}
	for _, other := range contracts {
		if other.UUID != meta.UUID && other.Status == typescontract.ContractActive {
			if len(other.Workdirs) > 0 && other.Workdirs[0].Path == targetPath {
				return fmt.Errorf("cannot swap: directory is already primary in another active contract: %s", other.UUID)
			}
		}
	}

	// 5. Perform swap
	meta.Workdirs[foundIndex], meta.Workdirs[0] = meta.Workdirs[0], meta.Workdirs[foundIndex]

	// 6. Persist changes
	if err := saveContractMetadata(meta); err != nil {
		return fmt.Errorf("failed to save contract metadata: %w", err)
	}

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return fmt.Errorf("failed to open chat database: %w", err)
	}
	defer sqliteDB.Close()

	dbData := db.ContractMessageData{ContractData: meta.ContractData}
	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
	}

	// 7. Notify Frontend via Event
	payload := ContractChangePayload{
		Status: string(typescontract.ContractActive),
	}
	if err := InsertEvent(meta.UUID, meta.ChatID, EventTypeContractChange, payload, "cli", time.Now().Add(5*time.Second)); err != nil {
		logger.Warning("Failed to insert contract change event", "error", err)
	}

	logger.Info("Primary workdir changed", "uuid", uuid, "name", name, "path", targetPath)
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
	
	// Format Workdirs
	sb.WriteString("  Workdirs:\n")
	for i, w := range info.Workdirs {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("    [Primary]   %s (%s)\n", filepath.Base(w.Path), w.Path))
		} else {
			sb.WriteString(fmt.Sprintf("    [Secondary] %s (%s)\n", w.Name, w.Path))
		}
	}
	
	// Calculate and display Homedir
	homedir := GetDefaultHomeDir(info.UUID, "")
	sb.WriteString(fmt.Sprintf("  Homedir:      %s\n", homedir))
	
	sb.WriteString(fmt.Sprintf("  Created At:   %s\n", info.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Expires At:   %s\n", info.ExpiresAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Exec Timeout: %ds\n", info.ExecTimeout))
	
	whitelistVal := "Default Safe Set"
	if info.NoWhitelist {
		whitelistVal = "Unrestricted"
	} else if len(info.Whitelist) > 0 {
		whitelistVal = fmt.Sprintf("`%s`", strings.Join(info.Whitelist, "`, `"))
	}
	sb.WriteString(fmt.Sprintf("  Whitelist:    %s\n", whitelistVal))

	// Workspace Preferences
	terminal := info.PreferredTerminal
	if terminal == "" {
		terminal = "None"
	}
	sb.WriteString(fmt.Sprintf("  Terminal:     %s\n", terminal))
	
	return sb.String()
}

// Note: FormatContractInfo has been updated to calculate and display the Homedir
// dynamically to avoid modifying the ContractInfoResult struct in models.go.
// The logic is embedded in the function body below.

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
	sb.WriteString("Contract Information:\n")
	sb.WriteString(fmt.Sprintf("  UUID:         %s\n", result.UUID))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", result.Status))
	sb.WriteString(fmt.Sprintf("  Expires At:   %s\n", result.ExpiresAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Primary Workdir: %s\n", result.PrimaryWorkdir))
	sb.WriteString("\n")

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

	if result.DiffUnified != "" {
		sb.WriteString("Diff:\n")
		sb.WriteString(result.DiffUnified)
	}

	return sb.String()
}

// GetDefaultHomeDir returns the default home directory for a contract.
// It now supports a dumpType parameter to create hierarchical structures (e.g., dumps/<uuid>/mapped).
func GetDefaultHomeDir(uuid string, dumpType string) string {
	gscHome, _ := settings.GetGSCHome(false)
	baseDir := filepath.Join(gscHome, settings.HomesRelPath, uuid)
	
	if dumpType != "" {
		return filepath.Join(baseDir, dumpType)
	}
	
	return baseDir
}

// writeContractMarker creates the .gsc-contract.json file in the home directory.
func writeContractMarker(homedir, uuid, workdir string) error {
	marker := map[string]string{
		"contract_uuid": uuid,
		"workdir":       workdir,
		"homedir":       homedir,
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contract marker: %w", err)
	}
	markerPath := filepath.Join(homedir, ".gsc-contract.json")
	if err := os.WriteFile(markerPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write contract marker: %w", err)
	}
	logger.Info("Contract marker written", "path", markerPath)
	return nil
}

// DiscoverContractHome walks up the directory tree to find the contract home.
func DiscoverContractHome() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	for {
		markerPath := filepath.Join(cwd, ".gsc-contract.json")
		if _, err := os.Stat(markerPath); err == nil {
			return cwd, nil
		}

		parent := filepath.Dir(cwd)
		if parent == cwd {
			break // Reached root
		}
		cwd = parent
	}

	return "", fmt.Errorf("contract home not found")
}
