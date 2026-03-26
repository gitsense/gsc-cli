/**
 * Component: Contract Manager
 * Block-UUID: d5c4ae73-40e7-4ac1-8735-34706b244cea
 * Parent-UUID: 19cb9bb2-6951-4408-9e80-d753b10a0c9a
 * Version: 1.21.0
 * Description: Removed Terminal and Review Tool fields from the contract info output display.
 * Language: Go
 * Created-at: 2026-03-26T15:58:26.955Z
 * Authors: GLM-4.7 (v1.20.0), GLM-4.7 (v1.21.0)
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
		ContractData: typescontract.ContractData{
			UUID:        uuid.New().String(),
			Workdirs: []typescontract.WorkdirEntry{
				{Name: "primary", Path: absWorkdir, AddedAt: now, Status: "active"},
			},
			Authcode:    authcode,
			Description: description,
			Status:      typescontract.ContractActive,
			ExpiresAt:   now.Add(time.Duration(settings.DefaultContractTTL) * time.Hour),
			Whitelist:   whitelist,
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
		if c.Status == typescontract.ContractActive && len(c.Workdirs) > 0 && c.Workdirs[0].Path == absWorkdir {
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
	if meta.Status != typescontract.ContractCancelled && meta.Status == typescontract.ContractActive && time.Now().After(meta.ExpiresAt) {
		status = string(typescontract.ContractExpired)
	}

	result := &ContractInfoResult{
		UUID: meta.UUID,
		Workdirs: func() []string {
			paths := make([]string, len(meta.Workdirs)) // Helper to extract paths
			for i, w := range meta.Workdirs {
				paths[i] = w.Path
			}
			return paths
		}(),
		Description: meta.Description,
		Status:      status,
		CreatedAt:   meta.CreatedAt,
		ExpiresAt:   meta.ExpiresAt,
		Authcode:    meta.Authcode,
		Workdir:     meta.Workdirs[0].Path,
		ExecTimeout: meta.ExecTimeout,
		Whitelist:   meta.Whitelist,
		NoWhitelist: meta.NoWhitelist,
		PreferredTerminal: meta.PreferredTerminal,
		PreferredReview:   meta.PreferredReview,
	}

	if sanitize {
		cwd, err := os.Getwd()
		if err == nil {
			relPath, err := filepath.Rel(cwd, meta.Workdirs[0].Path)
			if err == nil {
				result.Workdir = relPath
			} else {
				result.Workdir = filepath.Base(meta.Workdirs[0].Path)
			}
		} else {
			result.Workdir = filepath.Base(meta.Workdirs[0].Path)
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
