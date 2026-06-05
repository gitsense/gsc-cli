/**
 * Component: Import Git Rebuild Orchestrator
 * Block-UUID: 056aa5ef-2502-40fa-a242-3a4633073cba
 * Parent-UUID: fb7981f4-a605-436b-80f7-13e4c28a5718
 * Version: 1.6.0
 * Description: Added AnalysisLoadedCount pointer to RebuildConfig to pass the loaded record count back to the caller without changing the function signature. This fixes the "declared and not used" compilation error and allows command.go to display the count in the summary. v1.4.0: Consolidated rebuild state into import-git.json by removing separate rebuild-state.json file. Removed loadRebuildState, saveRebuildState, and cleanupRebuildState functions. Updated RunRebuild to use LoadState and SaveRebuildState from state.go. Fixed stageCheckClean to ignore .gitsense/ directory changes to prevent false positives when modifying import-git.json. v1.4.1: Fixed missing "path/filepath" import that was accidentally removed during refactoring. v1.6.0: Updated stageDumpAnalysis to discover and dump all unique analyzers instead of just code-intent. Updated stageLoadAnalysis to handle multi-analyzer JSONL dumps by bucketing records and performing per-analyzer snapshots.
 * Language: Go
 * Created-at: 2026-05-17T14:54:54.689Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.4.1), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0)
 */


package importgit

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// RebuildConfig holds the configuration for the rebuild process.
type RebuildConfig struct {
	GitPath             string
	Owner               string
	Repo                string
	Branch              string
	DBConn              *sql.DB
	RefChatID           int64
	ShadowPath          string
	GSCHome             string
	AnalysisLoadedCount *int64 // Pointer to receive the loaded count for summary display
}

// RebuildStageFunc defines the signature for a rebuild stage function.
type RebuildStageFunc func(ctx context.Context, state *RebuildState, config *RebuildConfig) error

// RunRebuild executes the rebuild workflow.
// It accepts an importFn callback to perform the actual import (Stage 5) to avoid circular dependencies.
func RunRebuild(ctx context.Context, config *RebuildConfig, importFn func() error) error {
	// Load rebuild state from import-git.json (if resuming)
	importState, err := LoadState(config.GitPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load import state: %w", err)
	}

	// v1.5.0: Validate that import state exists and branch is present
	// This prevents confusing errors later when trying to save rebuild state
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no import state found at %s. Rebuild requires an existing import. Run 'gsc app import git' first.", filepath.Join(config.GitPath, gitsenseDirName, stateFileName))
		}
		return fmt.Errorf("failed to load import state: %w", err)
	}

	// Verify the branch exists in the state
	if _, ok := importState.Branches[config.Branch]; !ok {
		return fmt.Errorf("branch '%s' not found in import state. Rebuild requires an existing import for this branch. Run 'gsc app import git' first.", config.Branch)
	}

	var state *RebuildState
	if importState != nil {
		if bs, ok := importState.Branches[config.Branch]; ok {
			state = bs.Rebuild
		}
	}

	if state == nil {
		// Initialize new state
		state = &RebuildState{
			RefChatID:   config.RefChatID,
			StartedAt:   time.Now().Format(time.RFC3339),
			Checkpoints: make(map[string]string),
		}
	}

	// Define Stages
	// Note: load_analysis is NOT in this list. It runs after the import callback.
	stages := []struct {
		name string
		fn   RebuildStageFunc
	}{
		{"check_clean", stageCheckClean},
		{"dump_analysis", stageDumpAnalysis},
		{"delete_shadow", stageDeleteShadow},
		{"soft_delete_db", stageSoftDeleteDB},
	}

	// Execute Stages
	for _, stage := range stages {
		// Skip if already complete
		if state.Checkpoints[stage.name] == "complete" {
			logger.Info("Skipping completed stage", "stage", stage.name)
			continue
		}

		// Mark as pending
		state.Checkpoints[stage.name] = "pending"
		if err := SaveRebuildState(config.GitPath, config.Branch, state); err != nil {
			return fmt.Errorf("failed to save state before stage %s: %w", stage.name, err)
		}

		logger.Info("Starting rebuild stage", "stage", stage.name)
		
		// Execute stage
		if err := stage.fn(ctx, state, config); err != nil {
			return fmt.Errorf("stage %s failed: %w", stage.name, err)
		}

		// Mark as complete
		state.Checkpoints[stage.name] = "complete"
		if err := SaveRebuildState(config.GitPath, config.Branch, state); err != nil {
			return fmt.Errorf("failed to save state after stage %s: %w", stage.name, err)
		}
	}

	// Execute Import (Stage 5)
	// This is done outside the loop because it's a callback
	if state.Checkpoints["import"] != "complete" {
		logger.Info("Starting rebuild stage", "stage", "import")
		if err := importFn(); err != nil {
			return fmt.Errorf("stage import failed: %w", err)
		}
		state.Checkpoints["import"] = "complete"
		if err := SaveRebuildState(config.GitPath, config.Branch, state); err != nil {
			return fmt.Errorf("failed to save state after stage import: %w", err)
		}
	}

	// Fetch New RefChatID for Analysis Restoration
	// Only do this if we have analysis to restore
	var loadedCount int64
	if state.HasAnalysis {
		newRefChatID, err := getNewRefChatID(config.DBConn, config.Owner, config.Repo, config.Branch)
		if err != nil {
			return fmt.Errorf("failed to resolve new RefChatID after import: %w", err)
		}
		state.RefChatID = newRefChatID
	}

	// Execute Load Analysis (Stage 6)
	// This must run after import so we have the new RefChatID.
	if state.Checkpoints["load_analysis"] != "complete" {
		logger.Info("Starting rebuild stage", "stage", "load_analysis")
		
		// Mark as pending
		state.Checkpoints["load_analysis"] = "pending"
		if err := SaveRebuildState(config.GitPath, config.Branch, state); err != nil {
			return fmt.Errorf("failed to save state before stage load_analysis: %w", err)
		}

		loadedCount, err = stageLoadAnalysis(ctx, state, config)
		if err != nil {
			return fmt.Errorf("stage load_analysis failed: %w", err)
		}

		// Mark as complete
		state.Checkpoints["load_analysis"] = "complete"
		if err := SaveRebuildState(config.GitPath, config.Branch, state); err != nil {
			return fmt.Errorf("failed to save state after stage load_analysis: %w", err)
		}
	}

	// Pass the loaded count back to the caller via the config pointer
	if config.AnalysisLoadedCount != nil {
		*config.AnalysisLoadedCount = loadedCount
	}

	// Cleanup
	// Clear rebuild state from import-git.json
	if err := ClearRebuildState(config.GitPath, config.Branch); err != nil {
		logger.Warning("Failed to clear rebuild state", "error", err)
	}
	// Delete temp analysis dump file if it exists
	if state.AnalysisDumpFile != "" {
		if err := os.Remove(state.AnalysisDumpFile); err != nil && !os.IsNotExist(err) {
			logger.Warning("Failed to delete temporary analysis dump file", "path", state.AnalysisDumpFile, "error", err)
		}
	}

	return nil
}

// getNewRefChatID resolves the RefChatID for the current branch after a fresh import.
func getNewRefChatID(dbConn *sql.DB, owner, repo, branch string) (int64, error) {
	groupID, err := db.GetGroupID(dbConn, owner, repo)
	if err != nil {
		return 0, fmt.Errorf("failed to get group ID: %w", err)
	}
	refID, err := db.GetRefChatID(dbConn, groupID, branch)
	if err != nil {
		return 0, fmt.Errorf("failed to get ref chat ID: %w", err)
	}
	return refID, nil
}

// stageCheckClean ensures there are no uncommitted changes.
// v1.4.0: Fixed to ignore .gitsense/ directory changes to prevent false positives.
func stageCheckClean(ctx context.Context, state *RebuildState, config *RebuildConfig) error {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = config.GitPath
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	for _, line := range strings.Split(strings.TrimRight(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		// Porcelain v1: "XY path" - path starts at index 3
		if len(line) > 3 && !strings.HasPrefix(line[3:], ".gitsense/") {
			return fmt.Errorf("source repository has uncommitted changes. Please commit or stash your changes before rebuilding.")
		}
	}
	return nil
}

// stageDumpAnalysis dumps the branch analysis to a temporary file.
// v1.6.0: Updated to discover and dump all unique analyzers instead of just code-intent.
func stageDumpAnalysis(ctx context.Context, state *RebuildState, config *RebuildConfig) error {
	// Discover analyzers
	analyzers, err := db.GetUniqueAnalyzers(config.DBConn, state.RefChatID)
	if err != nil {
		return fmt.Errorf("failed to discover analyzers: %w", err)
	}

	if len(analyzers) == 0 {
		state.HasAnalysis = false
		logger.Info("No analysis records found to dump")
		return nil
	}

	// Create temp file
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf(".gitsense-rebuild-%d.jsonl", state.RefChatID))
	
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp dump file: %w", err)
	}
	defer file.Close()

	dumpID := fmt.Sprintf("rebuild-%d", time.Now().Unix())
	totalWritten := int64(0)

	for _, analyzerType := range analyzers {
		logger.Info("Dumping analysis", "analyzer", analyzerType)
		// We don't have existing hashes for a fresh rebuild dump, so pass nil.
		written, _, err := db.DumpAnalysis(config.DBConn, state.RefChatID, analyzerType, dumpID, config.Owner, config.Repo, config.Branch, file, nil)
		if err != nil {
			return fmt.Errorf("failed to dump analysis for %s: %w", analyzerType, err)
		}
		totalWritten += written
	}

	if totalWritten > 0 {
		state.AnalysisDumpFile = tempFile
		state.HasAnalysis = true
		logger.Info("Analysis dumped successfully", "total_records", totalWritten, "path", tempFile)
	} else {
		state.HasAnalysis = false
		logger.Info("No analysis records were written to dump")
	}
	
	return nil
}

// stageDeleteShadow deletes the shadow repository.
func stageDeleteShadow(ctx context.Context, state *RebuildState, config *RebuildConfig) error {
	if err := DeleteShadow(config.ShadowPath); err != nil {
		return fmt.Errorf("failed to delete shadow repo: %w", err)
	}
	return nil
}

// stageSoftDeleteDB recursively soft deletes the branch from the database.
func stageSoftDeleteDB(ctx context.Context, state *RebuildState, config *RebuildConfig) error {
	if state.RefChatID == 0 {
		return fmt.Errorf("RefChatID is required to soft delete database")
	}

	if err := db.DeleteChatAndDescendants(config.DBConn, state.RefChatID); err != nil {
		return fmt.Errorf("failed to soft delete branch: %w", err)
	}
	
	return nil
}

// stageLoadAnalysis loads the analysis from the temporary file.
// v1.6.0: Updated to handle multi-analyzer JSONL dumps by bucketing records and performing per-analyzer snapshots.
func stageLoadAnalysis(ctx context.Context, state *RebuildState, config *RebuildConfig) (int64, error) {
	if !state.HasAnalysis {
		logger.Info("No analysis to restore")
		return 0, nil
	}

	if state.AnalysisDumpFile == "" {
		return 0, fmt.Errorf("analysis dump file path is missing")
	}

	// Open file
	file, err := os.Open(state.AnalysisDumpFile)
	if err != nil {
		return 0, fmt.Errorf("failed to open analysis dump file: %w", err)
	}
	defer file.Close()

	// Ingest JSONL (File -> RAM)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	
	var allRecords []db.DumpRecord
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		var record db.DumpRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return 0, fmt.Errorf("failed to parse JSONL line %d: %w", lineCount, err)
		}
		allRecords = append(allRecords, record)
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading dump file: %w", err)
	}

	// Group by analyzer prefix
	buckets := make(map[string][]db.DumpRecord)
	for _, r := range allRecords {
		// Extract prefix (e.g., "code-intent" from "code-intent::file-content::default")
		prefix := strings.SplitN(r.Analyzer, "::", 2)[0]
		buckets[prefix] = append(buckets[prefix], r)
	}

	var candidates []db.LoadItem
	for analyzer, records := range buckets {
		logger.Info("Preparing restoration for analyzer", "analyzer", analyzer, "records", len(records))
		
		// Build Target Snapshot for this specific analyzer
		targetMap, err := db.BuildTargetSnapshot(config.DBConn, state.RefChatID, analyzer+"::%")
		if err != nil {
			return 0, fmt.Errorf("failed to build target snapshot for %s: %w", analyzer, err)
		}

		for _, record := range records {
			if entry, ok := targetMap[record.Path]; ok {
				if !entry.HasAnalysis {
					candidates = append(candidates, db.LoadItem{
						Path:     record.Path,
						ChatID:   entry.ChatID,
						ParentID: entry.ParentID,
						Record:   record,
					})
				}
			}
		}
	}

	if len(candidates) == 0 {
		logger.Info("No analysis candidates to load (all files already analyzed or not in branch)")
		return 0, nil
	}

	// Load Analysis
	loaded, err := db.LoadAnalysis(ctx, config.DBConn, candidates, 1000, func(n int, path string) {
		logger.Debug("Loading analysis", "count", n, "path", path)
	})

	if err != nil {
		return 0, fmt.Errorf("failed to load analysis: %w", err)
	}

	logger.Info("Analysis loaded successfully", "records", loaded)
	return loaded, nil
}
