/**
 * Component: Change Post-Processor
 * Block-UUID: 1c1f3e43-595e-4255-a855-53382ee68c3c
 * Parent-UUID: 3e413119-345a-470b-9451-83c27884ea50
 * Version: 1.10.0
 * Description: Orchestrates the post-processing phase after a change turn completes. Added support for code provenance recording and ephemeral header injection. When enableProvenance is true, it extracts existing headers, generates new UUIDs, injects headers into modified files, and records entries in the worktree-level provenance ledger. Added version fallback to default to 1.0.0 if AI omits version. Added ModelID population for provenance entries. Added GitContext, OtherChanges, and Environment capture for comprehensive audit trail and UI attribution.
 * Language: Go
 * Created-at: 2026-04-29T02:42:03.684Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0)
 */


package intent_workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/provenance"
	"github.com/gitsense/gsc-cli/internal/version"
	"github.com/google/uuid"
)


// PostProcessor orchestrates the post-processing phase for a change turn.
type PostProcessor struct {
	manager          *Manager
	turnStartTime    time.Time
	turn             int
	keepMeta         bool
	enableProvenance bool
	debugLogger      *DebugLogger
}

// NewPostProcessor creates a new PostProcessor.
func NewPostProcessor(manager *Manager, turn int, turnStartTime time.Time, keepMeta bool, enableProvenance bool, debugLogger *DebugLogger) *PostProcessor {
	return &PostProcessor{
		manager:          manager,
		turnStartTime:    turnStartTime,
		turn:             turn,
		keepMeta:         keepMeta,
		enableProvenance: enableProvenance,
		debugLogger:      debugLogger,
	}
}

// Run executes the full post-processing sequence.
func (p *PostProcessor) Run() error {
	// 1. Open post-process log
	log, err := NewPostProcessLogger(p.manager.config.GetSessionDir(), p.turn)
	if err != nil {
		p.debugLogger.LogError("Failed to open post-process log", err)
		return fmt.Errorf("failed to open post-process log: %w", err)
	}
	defer log.Close()

	log.Log("POST-PROCESSING", "Starting post-processing phase")
	log.Log("STATUS", "Current status: change_post_processing")

	// 2. Find all .change-meta.json files
	log.Log("STEP", "FindChangeMetaFiles")
	metaFiles, err := FindChangeMetaFiles(p.manager.session.WorkingDirectories)
	if err != nil {
		log.Log("ERROR", fmt.Sprintf("Failed to find meta files: %v", err))
		return fmt.Errorf("failed to find .change-meta.json files: %w", err)
	}
	log.Log("STEP", fmt.Sprintf("Found %d file(s)", len(metaFiles)))
	for _, f := range metaFiles {
		log.Log("STEP", fmt.Sprintf("  - %s", f))
	}

	// 3. Get Git ground truth
	log.Log("STEP", "GitProvider.GetChanges")
	gitProvider := NewGitProvider(p.manager.session.WorkingDirectories)
	gitChanges, err := gitProvider.GetChanges()
	if err != nil {
		log.Log("ERROR", fmt.Sprintf("Failed to get git changes: %v", err))
		return fmt.Errorf("failed to get git changes: %w", err)
	}
	log.Log("STEP", "GitProvider.GetChanges completed successfully")

	// 3.5 Get Git context for each working directory
	log.Log("STEP", "GitProvider.GetGitContext")
	gitContexts := make(map[string]GitContext)
	for _, wd := range p.manager.session.WorkingDirectories {
		ctx, err := gitProvider.GetGitContext(wd.Path)
		if err != nil {
			log.Log("WARN", fmt.Sprintf("Failed to get git context for %s: %v", wd.Path, err))
			// Don't fail the turn, just log warning
			continue
		}
		gitContexts[wd.Path] = *ctx
		log.Log("STEP", fmt.Sprintf("  - %s: branch=%s, head=%s", wd.Path, ctx.BranchName, ctx.HeadSHA))
	}

	// 4. Get change results from session to validate against
	changeResults, err := p.getChangeResults()
	if err != nil {
		log.Log("ERROR", fmt.Sprintf("Failed to get change results: %v", err))
		return fmt.Errorf("failed to get change results: %w", err)
	}

	// 5. Three-way validation (AI claims vs Git truth vs metadata presence)
	log.Log("STEP", "MetadataEnricher.Validate")
	enricher := NewMetadataEnricher(p.manager.session.WorkingDirectories)
	errorDetails, otherChanges, err := enricher.Validate(changeResults.FilesModified.Files, gitChanges, metaFiles)
	if err != nil {
		log.Log("ERROR", fmt.Sprintf("Validation logic error: %v", err))
		return fmt.Errorf("validation logic error: %w", err)
	}

	// If validation returned error details, mark turn as error and store structured error
	if errorDetails != nil {
		log.Log("ERROR", fmt.Sprintf("Validation failed: %s", errorDetails.Message))
		p.manager.MarkTurnAsError(p.turn, errorDetails)
		return fmt.Errorf("validation failed: %s", errorDetails.Message)
	}
	log.Log("STEP", "Validation passed")

	// 5.5 Log other changes for audit trail
	if len(otherChanges) > 0 {
		log.Log("STEP", fmt.Sprintf("Detected %d other change(s) not reported by AI", len(otherChanges)))
		for _, oc := range otherChanges {
			log.Log("STEP", fmt.Sprintf("  - %s: %s", oc.Path, oc.Status))
		}
	}

	// 6. Enrich meta files (add SHAs, change_type, language)
	log.Log("STEP", "MetadataEnricher.Enrich")
	if err := enricher.Enrich(metaFiles, gitChanges); err != nil {
		log.Log("ERROR", fmt.Sprintf("Enrichment failed: %v", err))
		return fmt.Errorf("enrichment failed: %w", err)
	}
	log.Log("STEP", "MetadataEnricher.Enrich completed successfully")

	// 6.5 Record Provenance and Inject Headers
	if p.enableProvenance {
		log.Log("STEP", "Recording Provenance and Injecting Headers")
		if err := p.recordProvenance(metaFiles, gitChanges); err != nil {
			log.Log("ERROR", fmt.Sprintf("Provenance recording failed: %v", err))
			return fmt.Errorf("provenance recording failed: %w", err)
		}
		log.Log("STEP", "Provenance recording completed successfully")
	}

	// 7. Process .change-meta.json files to build changelog (with correction loop)
	log.Log("STEP", "runMetadataCorrectionLoop")
	changelog, err := p.runMetadataCorrectionLoop(metaFiles)
	if err != nil {
		log.Log("ERROR", fmt.Sprintf("Metadata processing failed: %v", err))
		return fmt.Errorf("failed to process .change-meta.json files: %w", err)
	}
	log.Log("STEP", fmt.Sprintf("runMetadataCorrectionLoop completed: %d entries", len(changelog)))

	// 8. Aggregate metadata to JSONL for resumption support
	log.Log("STEP", "WriteChangeMetadataJSONL")
	if err := WriteChangeMetadataJSONL(metaFiles, p.manager.GetConfig().GetTurnDir(p.turn)); err != nil {
		log.Log("ERROR", fmt.Sprintf("Failed to write change-metadata.jsonl: %v", err))
		return fmt.Errorf("failed to write change-metadata.jsonl: %w", err)
	}
	log.Log("STEP", "WriteChangeMetadataJSONL completed successfully")

	// 9. Update session provenance from gitChanges
	log.Log("STEP", "updateSessionProvenance")
	p.updateSessionProvenance(gitChanges)
	log.Log("STEP", "Updated session provenance")

	// 10. Cleanup .change-meta.json files
	log.Log("STEP", "MetadataEnricher.Cleanup")
	if err := enricher.Cleanup(metaFiles, p.keepMeta); err != nil {
		log.Log("WARN", fmt.Sprintf("Cleanup failed: %v", err))
		// Don't fail the turn for cleanup errors
	} else {
		log.Log("STEP", fmt.Sprintf("Cleanup completed: %d file(s) removed", len(metaFiles)))
	}

	// 11. Write result.json
	log.Log("STEP", "writeChangeResult")
	if err := p.writeChangeResult(changelog, otherChanges, gitContexts); err != nil {
		log.Log("ERROR", fmt.Sprintf("Failed to write result.json: %v", err))
		return fmt.Errorf("failed to write result.json: %w", err)
	}
	log.Log("STEP", "writeChangeResult completed successfully")

	// 12. Set final status to change_complete
	log.Log("STATUS", "Setting status to change_complete")
	session := p.manager.GetSession()
	session.Status = "change_complete"
	if err := p.manager.WriteSessionState(); err != nil {
		log.Log("ERROR", fmt.Sprintf("Failed to write session state: %v", err))
		return fmt.Errorf("failed to write session state: %w", err)
	}

	log.Log("POST-PROCESSING", "Completed successfully")
	return nil
}

// recordProvenance handles header injection and ledger recording for all modified files.
func (p *PostProcessor) recordProvenance(metaFiles []string, gitChanges map[string]FileProvenance) error {
	for _, metaFilePath := range metaFiles {
		gscData, err := readAndParseMetaFile(metaFilePath)
		if err != nil {
			return err
		}

		// Determine working directory for this file
		var workingDir string
		var relPath string
		for _, wd := range p.manager.session.WorkingDirectories {
			if strings.HasPrefix(gscData.AbsolutePath, wd.Path) {
				workingDir = wd.Path
				relPath = strings.TrimPrefix(gscData.AbsolutePath, wd.Path)
				if strings.HasPrefix(relPath, string(os.PathSeparator)) {
					relPath = relPath[1:]
				}
				break
			}
		}

		if workingDir == "" {
			return fmt.Errorf("could not determine working directory for %s", gscData.AbsolutePath)
		}

		// 1. Read existing header to get ParentUUID and OldVersion
		parentUUID, oldVersion, _, _, err := provenance.ReadExistingHeader(gscData.OldBlobSHA, workingDir)
		if err != nil {
			return fmt.Errorf("failed to read existing header for %s: %w", gscData.AbsolutePath, err)
		}

		// 2. Prepare Provenance Entry
		newBlockUUID := uuid.New().String()
		timestamp := time.Now().UTC()
		
		entry := &provenance.ProvenanceEntry{
			Timestamp:      timestamp,
			SessionID:      p.manager.session.SessionID,
			TurnID:         p.turn,
			BlockUUID:      newBlockUUID,
			ParentUUID:     parentUUID,
			OldVersion:     oldVersion,
			NewVersion: func() string {
				if gscData.NewVersion == "" {
					return "1.0.0"
				}
				return gscData.NewVersion
			}(),
			Path:           relPath,
			WorkingDirPath: workingDir,
			OldBlobSHA:     gscData.OldBlobSHA,
			ChangeType:     gscData.ChangeType,
			AuthorType:     "ai",
			AuthorName:     p.manager.session.Model,
			ModelID:        p.manager.session.Model,
			Source:         "gsc-cli",
			Description:    gscData.Description,
		}

		// Populate line counts from gitChanges
		if prov, ok := gitChanges[gscData.AbsolutePath]; ok {
			entry.LinesAdded = prov.LinesAdded
			entry.LinesDeleted = prov.LinesDeleted
		}

		// 3. Inject Header into file
		if err := provenance.InjectCodeBlockHeader(gscData.AbsolutePath, gscData.Language, entry); err != nil {
			return err
		}

		// 4. Recalculate NewBlobSHA (now includes the header)
		newSHA, err := computeBlobSHA(workingDir, gscData.AbsolutePath)
		if err != nil {
			return fmt.Errorf("failed to recalculate SHA for %s: %w", gscData.AbsolutePath, err)
		}
		entry.NewBlobSHA = newSHA

		// 5. Record in Ledger
		if err := provenance.RecordChange(workingDir, entry); err != nil {
			return err
		}

		// 6. Update meta file data for subsequent steps (JSONL aggregation, result.json)
		gscData.OldVersion = oldVersion
		gscData.NewBlobSHA = newSHA
		
		updatedMeta, err := json.MarshalIndent(gscData, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(metaFilePath, updatedMeta, 0644); err != nil {
			return err
		}
	}
	return nil
}

// getChangeResults retrieves the change results from the turn in the session state.
func (p *PostProcessor) getChangeResults() (*ChangeResult, error) {
	status, err := p.manager.GetSessionStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get session status: %w", err)
	}

	for i := range status.Turns {
		if status.Turns[i].TurnNumber == p.turn {
			if status.Turns[i].Result != nil && status.Turns[i].Result.Change != nil {
				return status.Turns[i].Result.Change, nil
			}
			break
		}
	}

	return nil, fmt.Errorf("change results not found for turn %d", p.turn)
}

// runMetadataCorrectionLoop attempts to process metadata files, running a correction loop if parsing fails.
func (p *PostProcessor) runMetadataCorrectionLoop(metaFiles []string) ([]ChangelogEntry, error) {
	const maxTries = 3

	for attempt := 1; attempt <= maxTries; attempt++ {
		changelog, err := ProcessChangeMetaFiles(metaFiles)
		if err == nil {
			return changelog, nil
		}

		if !strings.Contains(err.Error(), "failed to parse") {
			return nil, err
		}

		if attempt == maxTries {
			return nil, fmt.Errorf("metadata correction failed after %d attempts: %w", maxTries, err)
		}

		p.debugLogger.Log("WORKER", fmt.Sprintf("Metadata correction attempt %d/%d", attempt, maxTries))

		var badFiles []map[string]string
		for _, filePath := range metaFiles {
			data, _ := os.ReadFile(filePath)
			badFiles = append(badFiles, map[string]string{
				"path":    filePath,
				"content": string(data),
			})
		}

		turnDir := p.manager.GetConfig().GetTurnDir(p.turn)
		badMetaPath := filepath.Join(turnDir, "bad-metadata-files.json")
		badMetaData, _ := json.MarshalIndent(badFiles, "", "  ")
		_ = os.WriteFile(badMetaPath, badMetaData, 0644)

		if err := SpawnMetadataCorrectionSubprocess(turnDir, badMetaPath); err != nil {
			return nil, fmt.Errorf("metadata correction subprocess failed: %w", err)
		}
	}

	return nil, fmt.Errorf("metadata correction failed")
}

// updateSessionProvenance updates the session state with Git provenance.
func (p *PostProcessor) updateSessionProvenance(gitChanges map[string]FileProvenance) {
	session := p.manager.GetSession()
	if session == nil || len(session.Turns) == 0 {
		return
	}

	var currentTurnState *TurnState
	for i := range session.Turns {
		if session.Turns[i].TurnNumber == p.turn {
			currentTurnState = &session.Turns[i]
			break
		}
	}
	if currentTurnState == nil || currentTurnState.Result == nil || currentTurnState.Result.Change == nil {
		return
	}

	for i := range currentTurnState.Result.Change.FilesModified.Files {
		f := &currentTurnState.Result.Change.FilesModified.Files[i]
		absPath := filepath.Join(f.WorkingDir, f.Path)
		if prov, ok := gitChanges[absPath]; ok {
			f.OldBlobSHA = prov.OldBlobSHA
			f.NewBlobSHA = prov.NewBlobSHA
			f.LinesAdded = prov.LinesAdded
			f.LinesDeleted = prov.LinesDeleted
		}
	}

	if err := p.manager.WriteSessionState(); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: Failed to write session state after updating provenance: %v\n", err)
	}
}

// writeChangeResult writes the final result.json file for the turn.
func (p *PostProcessor) writeChangeResult(changelog []ChangelogEntry, otherChanges []FileModified, gitContexts map[string]GitContext) error {
	session := p.manager.GetSession()

	var currentTurn *TurnState
	for i := range session.Turns {
		if session.Turns[i].TurnNumber == p.turn {
			currentTurn = &session.Turns[i]
			break
		}
	}

	if currentTurn == nil {
		return fmt.Errorf("turn %d not found", p.turn)
	}

	var changeResults *ChangeResult
	if currentTurn.Result != nil && currentTurn.Result.Change != nil {
		changeResults = currentTurn.Result.Change
	} else {
		changeResults = &ChangeResult{
			ChangeRequest: p.manager.GetSession().Intent,
			FilesModified: FilesModifiedSummary{
				TotalCount: 0,
				Files:      []FileModified{},
			},
		}
	}

	changeResults.Changelog = changelog
	changeResults.OtherChanges = FilesModifiedSummary{
		TotalCount: len(otherChanges),
		Files:      otherChanges,
	}
	changeResults.GitContexts = gitContexts

	// Populate environment metadata
	changeResults.Environment = Environment{
		GSCVersion: version.Version,
		User:       getCurrentUser(),
		Timestamp:  p.turnStartTime.UTC().Format(time.RFC3339),
	}

	resultPath := filepath.Join(p.manager.GetConfig().GetTurnDir(p.turn), "result.json")
	resultData, err := json.MarshalIndent(changeResults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal change results: %w", err)
	}

	return os.WriteFile(resultPath, resultData, 0644)
}

// getCurrentUser returns the current OS user, or "unknown" if unavailable.
func getCurrentUser() string {
	currentUser, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return currentUser.Username
}
