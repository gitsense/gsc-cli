/**
 * Component: Provenance Manager
 * Block-UUID: 8a7b9c1d-2e3f-4a5b-8c9d-0e1f2a3b4c5d
 * Parent-UUID: cb9d2a5f-1200-45e4-b3eb-52e40894ada4
 * Version: 1.0.1
 * Description: Handles file operations (update-file, new-file) and provenance logging. Enforces traceability rules like UUID uniqueness and parent-child validation.
 * Language: Go
 * Created-at: 2026-02-26T04:22:53.564Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1)
 */


package manifest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/search"
	"github.com/gitsense/gsc-cli/internal/traceability"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// Contract-specific exit codes
const (
	ExitContractExpired      = 10 // Contract has expired
	ExitParentUUIDMismatch   = 11 // Parent-UUID in code doesn't match file
	ExitDuplicateBlockUUID   = 12 // Block-UUID already exists in workdir
	ExitTargetFileNotFound   = 13 // Target file not found (for update-file)
	ExitTargetPathExists     = 14 // Target path already exists (for new-file)
	ExitInvalidTargetPath    = 15 // Target path is not relative or escapes workdir
)

// UpdateFile updates an existing traceable file using a contract.
// It validates the Parent-UUID, performs an atomic write, and logs the provenance.
func UpdateFile(contractUUID string, sourceFile string) error {
	ctx := context.Background()
	engine := &search.RipgrepEngine{}

	// 1. Load and Validate Contract
	contract, err := GetContract(contractUUID)
	if err != nil {
		return fmt.Errorf("failed to load contract: %w", err)
	}

	if !isContractActive(contract) {
		return &ContractError{Code: ExitContractExpired, Message: "Contract has expired"}
	}

	// 2. Parse Source File (New Code)
	newContent, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	newMeta, _, err := traceability.ParseHeader(string(newContent))
	if err != nil {
		return fmt.Errorf("failed to parse source file header: %w", err)
	}

	// 3. Find Target File in Workdir via Parent-UUID
	targetPath, _, err := engine.FindBlockByUUID(ctx, contract.Workdir, newMeta.ParentUUID)
	if err != nil {
		return &ContractError{Code: ExitTargetFileNotFound, Message: fmt.Sprintf("Failed to locate target file for Parent-UUID %s: %v", newMeta.ParentUUID, err)}
	}

	// 4. Read Existing File (for SourceVersion)
	oldContent, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("failed to read target file: %w", err)
	}

	oldMeta, _, err := traceability.ParseHeader(string(oldContent))
	if err != nil {
		return fmt.Errorf("failed to parse target file header: %w", err)
	}

	// 5. Validate Inheritance
	if oldMeta.BlockUUID != newMeta.ParentUUID {
		return &ContractError{Code: ExitParentUUIDMismatch, Message: fmt.Sprintf("Parent-UUID mismatch: Expected %s, found %s", oldMeta.BlockUUID, newMeta.ParentUUID)}
	}

	// Calculate relative path for provenance log
	relPath, err := filepath.Rel(contract.Workdir, targetPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// 6. Atomic Write
	tmpPath := targetPath + ".gsc-tmp"
	if err := os.WriteFile(tmpPath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// 7. Log Provenance
	entry := ProvenanceEntry{
		Timestamp:     time.Now().UTC(),
		Status:        ProvenanceSaved,
		Action:        "update-file",
		FilePath:      relPath,
		BlockUUID:     newMeta.BlockUUID,
		ParentUUID:    newMeta.ParentUUID,
		SourceVersion: oldMeta.Version,
		TargetVersion: newMeta.Version,
		Author:        newMeta.Authors,
		ContractUUID:  contract.UUID,
		Description:   "Updated via gsc contract",
	}

	if err := logProvenance(entry, contract.Workdir); err != nil {
		// Log failure is fatal as per requirements
		return fmt.Errorf("failed to write provenance log: %w", err)
	}

	logger.Info("File updated successfully", "file", targetPath, "version", newMeta.Version)
	return nil
}

// NewFile creates a new traceable file using a contract.
// It validates path safety and UUID uniqueness, then logs the provenance.
func NewFile(contractUUID string, targetRelativePath string, sourceFile string) error {
	ctx := context.Background()
	engine := &search.RipgrepEngine{}

	// 1. Load and Validate Contract
	contract, err := GetContract(contractUUID)
	if err != nil {
		return fmt.Errorf("failed to load contract: %w", err)
	}

	if !isContractActive(contract) {
		return &ContractError{Code: ExitContractExpired, Message: "Contract has expired"}
	}

	// 2. Validate Target Path (Relative & Safe)
	if filepath.IsAbs(targetRelativePath) {
		return &ContractError{Code: ExitInvalidTargetPath, Message: "Target path must be relative"}
	}

	// Resolve absolute path and check for directory traversal
	absTargetPath := filepath.Join(contract.Workdir, targetRelativePath)
	relPath, err := filepath.Rel(contract.Workdir, absTargetPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return &ContractError{Code: ExitInvalidTargetPath, Message: "Target path escapes workdir"}
	}

	// Check if file already exists
	if _, err := os.Stat(absTargetPath); err == nil {
		return &ContractError{Code: ExitTargetPathExists, Message: "Target path already exists"}
	}

	// 3. Parse Source File
	newContent, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	newMeta, _, err := traceability.ParseHeader(string(newContent))
	if err != nil {
		return fmt.Errorf("failed to parse source file header: %w", err)
	}

	// 4. Ensure UUID Uniqueness
	if err := engine.EnsureUUIDUniqueness(ctx, contract.Workdir, newMeta.BlockUUID); err != nil {
		return &ContractError{Code: ExitDuplicateBlockUUID, Message: fmt.Sprintf("Block-UUID %s already exists in workdir", newMeta.BlockUUID)}
	}

	// Calculate relative path for provenance log
	relPath, err := filepath.Rel(contract.Workdir, absTargetPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// 5. Create Directories if needed
	if err := os.MkdirAll(filepath.Dir(absTargetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// 6. Atomic Write
	tmpPath := absTargetPath + ".gsc-tmp"
	if err := os.WriteFile(tmpPath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, absTargetPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// 7. Log Provenance
	entry := ProvenanceEntry{
		Timestamp:     time.Now().UTC(),
		Status:        ProvenanceSaved,
		Action:        "create-file",
		FilePath:      relPath,
		BlockUUID:     newMeta.BlockUUID,
		ParentUUID:    "N/A",
		SourceVersion: "N/A",
		TargetVersion: newMeta.Version,
		Author:        newMeta.Authors,
		ContractUUID:  contract.UUID,
		Description:   "Created via gsc contract",
	}

	if err := logProvenance(entry, contract.Workdir); err != nil {
		return fmt.Errorf("failed to write provenance log: %w", err)
	}

	logger.Info("File created successfully", "file", absTargetPath, "version", newMeta.Version)
	return nil
}

// logProvenance appends a JSONL entry to the project-local provenance log.
func logProvenance(entry ProvenanceEntry, baseDir string) error {
	// Construct path relative to the contract's workdir, not the CWD
	logPath := filepath.Join(baseDir, settings.GitSenseDir, settings.ProvenanceFileName)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return fmt.Errorf("failed to create provenance log directory: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open provenance log: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal provenance entry: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write provenance entry: %w", err)
	}

	return nil
}

// isContractActive checks if a contract is currently active (not expired or cancelled).
func isContractActive(c *ContractMetadata) bool {
	if c.Status == ContractCancelled {
		return false
	}
	if c.Status == ContractExpired {
		return false
	}
	if time.Now().After(c.ExpiresAt) {
		return false
	}
	return true
}

// ContractError wraps an error with a specific exit code for the CLI.
type ContractError struct {
	Code    int
	Message string
}

func (e *ContractError) Error() string {
	return e.Message
}
