/**
 * Component: Contract Manager
 * Block-UUID: b7fd22f4-114c-40e8-a3e9-412de6ba6eb6
 * Parent-UUID: 1fb9baea-6ef6-427d-90bf-5a07d1a06a88
 * Version: 1.15.0
 * Description: Added initEventsDB to create the messaging database skeleton on contract creation and added confirmation prompt to DeleteContract.
 * Language: Go
 * Created-at: 2026-03-07T02:26:57.766Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), Gemini 3 Flash (v1.0.7), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.3.2), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), Gemini 3 Flash (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), Gemini 3 Flash (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0)
 */


package contract

import (
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
	
	// Apply Defaults for Security Settings
	finalWhitelist := whitelist
	finalNoWhitelist := noWhitelist
	finalExecTimeout := execTimeout

	// If no whitelist provided and not unrestricted, use the default safe set
	if len(finalWhitelist) == 0 && !finalNoWhitelist {
		finalWhitelist = settings.DefaultSafeSet
	}

	// If timeout is 0 (default flag value), use the system default
	if finalExecTimeout == 0 {
		finalExecTimeout = settings.DefaultExecTimeout
	}

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
		Whitelist:   finalWhitelist,
		NoWhitelist: finalNoWhitelist,
		ExecTimeout: finalExecTimeout,
		PreferredEditor:   preferredEditor,
		PreferredTerminal: preferredTerminal,
		PreferredReview:   preferredReview,
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
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredEditor:   meta.PreferredEditor,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
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

	dbPath := filepath.Join(gscHome, "data", "dumps", uuid, "events.sqlite3")
	
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
		if meta.Status == ContractActive && now.After(meta.ExpiresAt) {
			meta.Status = ContractExpired
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
func GetContractInfo(uuid string, sanitize bool) (*ContractInfoResult, error) {
	meta, err := GetContract(uuid)
	if err != nil {
		return nil, err
	}

	status := string(meta.Status)
	if meta.Status != ContractCancelled && meta.Status == ContractActive && time.Now().After(meta.ExpiresAt) {
		status = string(ContractExpired)
	}

	result := &ContractInfoResult{
		UUID:        meta.UUID,
		Description: meta.Description,
		Status:      status,
		CreatedAt:   meta.CreatedAt,
		ExpiresAt:   meta.ExpiresAt,
		Authcode:    meta.Authcode,
		Workdir:     meta.Workdir,
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredEditor:   meta.PreferredEditor,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if sanitize {
		cwd, err := os.Getwd()
		if err == nil {
			relPath, err := filepath.Rel(cwd, meta.Workdir)
			if err == nil {
				result.Workdir = relPath
			} else {
				result.Workdir = filepath.Base(meta.Workdir)
			}
		} else {
			result.Workdir = filepath.Base(meta.Workdir)
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

	if meta.Status == ContractCancelled {
		return fmt.Errorf("contract %s is already cancelled", uuid)
	}

	meta.Status = ContractCancelled

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

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      string(meta.Status),
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredEditor:   meta.PreferredEditor,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
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

	if meta.Status == ContractDone {
		return fmt.Errorf("contract %s is already marked as done", uuid)
	}

	meta.Status = ContractDone

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

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      string(meta.Status),
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredEditor:   meta.PreferredEditor,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
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

	if meta.Status == ContractCancelled {
		return fmt.Errorf("cannot renew a cancelled contract")
	}

	duration := time.Duration(hours) * time.Hour
	newStart := time.Now()
	if meta.ExpiresAt.After(newStart) {
		newStart = meta.ExpiresAt
	}
	meta.ExpiresAt = newStart.Add(duration)

	if meta.Status == ContractExpired || meta.Status == ContractDone {
		meta.Status = ContractActive
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

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      string(meta.Status),
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredEditor:   meta.PreferredEditor,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		return fmt.Errorf("failed to update contract message in database: %w", err)
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

	dbData := db.ContractMessageData{
		Description: meta.Description,
		Workdir:     meta.Workdir,
		ExpiresAt:   meta.ExpiresAt,
		UUID:        meta.UUID,
		Status:      "deleted",
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredEditor:   meta.PreferredEditor,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if err := db.UpdateContractMessagesByUUID(sqliteDB, meta.UUID, dbData); err != nil {
		logger.Warning("Failed to update contract message in database", "error", err)
	}

	logger.Info("Contract deleted", "uuid", uuid)
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
	sb.WriteString(fmt.Sprintf("  Exec Timeout: %ds\n", info.ExecTimeout))
	
	whitelistVal := "Default Safe Set"
	if info.NoWhitelist {
		whitelistVal = "Unrestricted"
	} else if len(info.Whitelist) > 0 {
		whitelistVal = fmt.Sprintf("`%s`", strings.Join(info.Whitelist, "`, `"))
	}
	sb.WriteString(fmt.Sprintf("  Whitelist:    %s\n", whitelistVal))

	// Workspace Preferences
	editor := info.PreferredEditor
	if editor == "" {
		editor = "None"
	}
	sb.WriteString(fmt.Sprintf("  Editor:       %s\n", editor))

	terminal := info.PreferredTerminal
	if terminal == "" {
		terminal = "None"
	}
	sb.WriteString(fmt.Sprintf("  Terminal:     %s\n", terminal))

	review := info.PreferredReview
	if review == "" {
		review = "None"
	}
	sb.WriteString(fmt.Sprintf("  Review Tool:  %s\n", review))
	
	return sb.String()
}

// GetDefaultDumpDir returns the default output directory for a contract dump.
// It now supports a dumpType parameter to create hierarchical structures (e.g., dumps/<uuid>/mapped).
func GetDefaultDumpDir(uuid string, dumpType string) string {
	gscHome, _ := settings.GetGSCHome(false)
	baseDir := filepath.Join(gscHome, "data", "dumps", uuid)
	
	if dumpType != "" {
		return filepath.Join(baseDir, dumpType)
	}
	
	return baseDir
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
	sb.WriteString("Contract Information:\n")
	sb.WriteString(fmt.Sprintf("  UUID:         %s\n", result.UUID))
	sb.WriteString(fmt.Sprintf("  Status:       %s\n", result.Status))
	sb.WriteString(fmt.Sprintf("  Expires At:   %s\n", result.ExpiresAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("  Workdir:      %s\n", result.Workdir))
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
