/**
 * Component: Provenance Manager
 * Block-UUID: b4f3a8fa-e1e5-48ce-87c1-d16f78cf9ab6
 * Parent-UUID: e06c54c2-beef-4b93-860b-66b7e9fdbdda
 * Version: 1.1.1
 * Description: Added TestFile function to support the 'contract test' command, enabling pre-flight validation of code changes including UUID uniqueness and diff generation.
 * Language: Go
 * Created-at: 2026-02-26T18:13:46.301Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.1.0), GLM-4.7 (v1.1.1)
 */


package contract

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
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/sergi/go-diff/diffmatchpatch"
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

// Error codes for the 'test' command
const (
	ErrCodeContractExpired    = "CONTRACT_EXPIRED"
	ErrCodeContractCancelled  = "CONTRACT_CANCELLED"
	ErrCodeDuplicateBlockUUID = "DUPLICATE_BLOCK_UUID"
	ErrCodeParentNotFound     = "PARENT_UUID_NOT_FOUND"
	ErrCodeParentMismatch     = "PARENT_UUID_MISMATCH"
	ErrCodeHeaderParseFailed  = "HEADER_PARSE_FAILED"
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
	relPath, err = filepath.Rel(contract.Workdir, absTargetPath)
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

// TestFile validates a file change against a contract without writing it.
// It checks contract status, UUID uniqueness, and generates a diff if a parent exists.
func TestFile(contractUUID string, sourceFile string, sanitize bool) (*ContractTestResult, error) {
	ctx := context.Background()
	engine := &search.RipgrepEngine{}

	// 1. Load Contract
	contract, err := GetContract(contractUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to load contract: %w", err)
	}

	// 2. Check Contract Status
	if contract.Status == ContractCancelled {
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:   false,
			ErrorCode: ErrCodeContractCancelled,
			Message:   "The contract has been cancelled.",
		}, nil
	}

	if contract.Status == ContractExpired || time.Now().After(contract.ExpiresAt) {
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:   false,
			ErrorCode: ErrCodeContractExpired,
			Message:   "The contract has expired.",
		}, nil
	}

	// 3. Parse Source File
	newContentBytes, err := os.ReadFile(sourceFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	newMeta, _, err := traceability.ParseHeader(string(newContentBytes))
	if err != nil {
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:   false,
			ErrorCode: ErrCodeHeaderParseFailed,
			Message:   fmt.Sprintf("Failed to parse source file header: %v", err),
		}, nil
	}

	// 4. Check Block-UUID Uniqueness
	isUnique := true
	if err := engine.EnsureUUIDUniqueness(ctx, contract.Workdir, newMeta.BlockUUID); err != nil {
		isUnique = false
		// Try to find where it exists for a better error message
		if targetPath, _, findErr := engine.FindBlockByUUID(ctx, contract.Workdir, newMeta.BlockUUID); findErr == nil {
			relPath, _ := filepath.Rel(contract.Workdir, targetPath)
			return &ContractTestResult{
				ContractInfoResult: ContractInfoResult{
					UUID:        contract.UUID,
					Status:      string(contract.Status),
					Description: contract.Description,
					Workdir:     contract.Workdir,
					CreatedAt:   contract.CreatedAt,
					ExpiresAt:   contract.ExpiresAt,
				},
				Success:   false,
				ErrorCode: ErrCodeDuplicateBlockUUID,
				Message:   fmt.Sprintf("Block-UUID '%s' already exists in '%s'.", newMeta.BlockUUID, relPath),
				BlockUUID: newMeta.BlockUUID,
				IsUnique:  false,
			}, nil
		}
	}

	// 5. Handle Parent-UUID
	if newMeta.ParentUUID == "" || newMeta.ParentUUID == "N/A" {
		// New file scenario
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:    true,
			Message:    "Contract valid. New file detected.",
			BlockUUID:  newMeta.BlockUUID,
			ParentUUID: newMeta.ParentUUID,
			IsUnique:   isUnique,
		}, nil
	}

	// Update scenario: Find Parent
	targetPath, _, err := engine.FindBlockByUUID(ctx, contract.Workdir, newMeta.ParentUUID)
	if err != nil {
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:   false,
			ErrorCode: ErrCodeParentNotFound,
			Message:   fmt.Sprintf("Parent-UUID '%s' not found in workdir.", newMeta.ParentUUID),
			BlockUUID: newMeta.BlockUUID,
			ParentUUID: newMeta.ParentUUID,
			IsUnique:  isUnique,
		}, nil
	}

	// Read target file content
	oldContentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read target file: %w", err)
	}

	oldMeta, _, err := traceability.ParseHeader(string(oldContentBytes))
	if err != nil {
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:   false,
			ErrorCode: ErrCodeHeaderParseFailed,
			Message:   fmt.Sprintf("Failed to parse target file header: %v", err),
			BlockUUID: newMeta.BlockUUID,
			ParentUUID: newMeta.ParentUUID,
			IsUnique:  isUnique,
		}, nil
	}

	// Validate Parent-UUID Match
	if oldMeta.BlockUUID != newMeta.ParentUUID {
		return &ContractTestResult{
			ContractInfoResult: ContractInfoResult{
				UUID:        contract.UUID,
				Status:      string(contract.Status),
				Description: contract.Description,
				Workdir:     contract.Workdir,
				CreatedAt:   contract.CreatedAt,
				ExpiresAt:   contract.ExpiresAt,
			},
			Success:   false,
			ErrorCode: ErrCodeParentMismatch,
			Message:   fmt.Sprintf("Parent-UUID mismatch: Expected '%s', found '%s' in target file.", newMeta.ParentUUID, oldMeta.BlockUUID),
			BlockUUID: newMeta.BlockUUID,
			ParentUUID: newMeta.ParentUUID,
			IsUnique:  isUnique,
		}, nil
	}

	// 6. Generate Diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(oldContentBytes), string(newContentBytes), false)
	
	// Generate HTML Diff
	diffHTML := dmp.DiffPrettyHtml(diffs)
	
	// Generate Unified Diff
	// Note: diffmatchpatch doesn't have a built-in unified diff generator, 
	// so we construct a simple one or just return the raw diffs if needed.
	// For now, we will return a simplified unified representation or just the HTML.
	// A proper unified diff generator is complex, so we'll stick to HTML for the frontend
	// and a simple text representation for the CLI.
	diffUnified := generateUnifiedDiff(string(oldContentBytes), string(newContentBytes))

	// 7. Sanitize Paths
	relPath := targetPath
	if sanitize {
		relPath, _ = filepath.Rel(contract.Workdir, targetPath)
	}

	return &ContractTestResult{
		ContractInfoResult: ContractInfoResult{
			UUID:        contract.UUID,
			Status:      string(contract.Status),
			Description: contract.Description,
			Workdir:     contract.Workdir,
			CreatedAt:   contract.CreatedAt,
			ExpiresAt:   contract.ExpiresAt,
		},
		Success:      true,
		Message:      "Contract valid. Block-UUID is unique. Parent found.",
		RelativePath: relPath,
		DiffHTML:     diffHTML,
		DiffUnified:  diffUnified,
		BlockUUID:    newMeta.BlockUUID,
		ParentUUID:   newMeta.ParentUUID,
		IsUnique:     isUnique,
	}, nil
}

// generateUnifiedDiff creates a simple unified diff string.
// This is a basic implementation; a full implementation would handle line numbers and context.
func generateUnifiedDiff(oldText, newText string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, false)
	
	var sb strings.Builder
	sb.WriteString("--- Original\n")
	sb.WriteString("+++ Modified\n")
	
	// This is a very simplified view. A real unified diff needs line numbers.
	// For the purpose of this feature, the HTML diff is the primary artifact.
	// We will just list the changes here.
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			sb.WriteString("+ ")
			sb.WriteString(diff.Text)
		case diffmatchpatch.DiffDelete:
			sb.WriteString("- ")
			sb.WriteString(diff.Text)
		case diffmatchpatch.DiffEqual:
			sb.WriteString("  ")
			sb.WriteString(diff.Text)
		}
	}
	return sb.String()
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
