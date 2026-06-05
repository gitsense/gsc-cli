/**
 * Component: Intent Workflow Manager Resume
 * Block-UUID: 00c0206f-a8b3-48ff-be84-41fbb3eb2d81
 * Parent-UUID: 506175ad-3c09-4f9b-81bd-fb84393bf62b
 * Version: 1.0.3
 * Description: Updated template paths from data/templates to cli/templates to separate CLI-specific data from app-specific data.
 * Language: Go
 * Created-at: 2026-04-29T02:44:35.946Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package intent_workflow

import (
	"time"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// StartResumeChangeTurn initiates a resume-change turn to generate missing metadata.
// It extracts Git provenance, renders the resume task template, and spawns the subprocess.
func (m *Manager) StartResumeChangeTurn(failedTurnNumber int) error {
	if m.session == nil {
		return fmt.Errorf("session not initialized")
	}

	m.debugLogger.Log("DEBUG", fmt.Sprintf("Starting resume-change turn for failed turn %d", failedTurnNumber))

	// Find the failed change turn
	var failedTurn *TurnState
	for i := range m.session.Turns {
		if m.session.Turns[i].TurnNumber == failedTurnNumber && m.session.Turns[i].TurnType == "change" {
			failedTurn = &m.session.Turns[i]
			break
		}
	}

	if failedTurn == nil {
		return fmt.Errorf("failed change turn %d not found", failedTurnNumber)
	}

	if failedTurn.Status != "error" {
		return fmt.Errorf("turn %d is not in error state", failedTurnNumber)
	}

	// Calculate next turn number
	nextTurn := m.GetNextTurnNumber()
	m.currentTurn = nextTurn
	m.session.Status = "resume-change"

	// Ensure turn directory exists
	if err := m.config.EnsureTurnDir(nextTurn); err != nil {
		m.debugLogger.LogError("Failed to create turn directory", err)
		m.markAsError("DIR_CREATE_FAILED", fmt.Sprintf("Failed to create turn directory: %v", err))
		return err
	}

	// Write intent to turn directory (reuse original intent)
	intentPath := filepath.Join(m.config.GetTurnDir(nextTurn), "intent.md")
	if err := os.WriteFile(intentPath, []byte(m.session.Intent), 0644); err != nil {
		m.debugLogger.LogError("Failed to write intent file", err)
		m.markAsError("INTENT_WRITE_FAILED", fmt.Sprintf("Failed to write intent file: %v", err))
		return err
	}

	// Extract modified files with SHAs
	// Priority 1: Read from change-metadata.jsonl (preserves AI descriptions)
	// Priority 2: Fallback to git diff --raw
	var modifiedFiles []ModifiedFileWithSHA
	var err error
	
	failedTurnDir := m.config.GetTurnDir(failedTurnNumber)
	modifiedFiles, err = m.readChangeMetadataJSONL(failedTurnDir, m.session.WorkingDirectories)
	if err != nil {
		m.debugLogger.Log("DEBUG", fmt.Sprintf("Could not read change-metadata.jsonl (or it was empty), falling back to git diff: %v", err))
		modifiedFiles, err = m.extractModifiedFilesWithSHAs(m.session.WorkingDirectories)
		if err != nil {
			m.debugLogger.LogError("Failed to extract modified files", err)
			return fmt.Errorf("failed to extract modified files: %w", err)
		}
	}

	// Render resume_task.md template with SHAs
	if err := m.renderResumeTask(nextTurn, modifiedFiles); err != nil {
		m.debugLogger.LogError("Failed to render resume task", err)
		return fmt.Errorf("failed to render resume task: %w", err)
	}

	// Close previous eventWriter if it exists
	if m.eventWriter != nil {
		m.eventWriter.Close()
	}

	// Create log file for this turn
	logFilename := fmt.Sprintf("raw-stream-%d.ndjson", time.Now().Unix())
	logPath := m.config.GetTurnLogFile(m.currentTurn, logFilename)

	m.eventWriter, err = NewEventWriter(logPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create event writer", err)
		m.markAsError("INIT_FAILED", fmt.Sprintf("Failed to create event writer: %v", err))
		return err
	}

	// Create new turn state and append to turns slice
	newTurn := TurnState{
		TurnNumber:  nextTurn,
		TurnType:    "resume-change",
		Status:      "running",
		StartedAt:   time.Now(),
		LogPath:     logPath,
		ProcessInfo: ProcessInfo{Running: true},
	}
	m.session.Turns = append(m.session.Turns, newTurn)

	// Write session state to persist log paths
	if err = m.writeSessionState(); err != nil {
		m.debugLogger.LogError("Failed to write session state", err)
		return err
	}

	// Write turn history to disk
	if err = m.writeTurnHistory(m.currentTurn); err != nil {
		m.debugLogger.LogError("Failed to write turn history", err)
	}

	// Spawn subprocess for resume-change turn
	if err = m.spawnClaudeSubprocess(m.currentTurn, "resume-change", []string{}); err != nil {
		m.debugLogger.LogError("Failed to spawn subprocess", err)
		m.markAsError("SPAWN_FAILED", fmt.Sprintf("Failed to spawn subprocess: %v", err))
		return err
	}

	// Wait for stream processing to complete (blocking for worker process)
	m.wg.Wait()

	return m.writeSessionState()
}

// readChangeMetadataJSONL reads the change-metadata.jsonl file from the turn directory.
// It returns a list of ModifiedFileWithSHA objects. If the file doesn't exist or is invalid,
// it returns an error so the caller can fall back to git diff.
func (m *Manager) readChangeMetadataJSONL(turnDir string, workdirs []WorkingDirectory) ([]ModifiedFileWithSHA, error) {
	jsonlPath := filepath.Join(turnDir, "change-metadata.jsonl")
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err // File not found
	}
	defer file.Close()

	var files []ModifiedFileWithSHA
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var gscData GSCFileData
		if err := json.Unmarshal(scanner.Bytes(), &gscData); err != nil {
			m.debugLogger.Log("WARN", fmt.Sprintf("Skipping invalid JSONL line: %v", err))
			continue
		}

		// Convert AbsolutePath to WorkingDir + Path
		// We need to find which working directory this file belongs to
		var foundWd string
		var relPath string
		for _, wd := range workdirs {
			if strings.HasPrefix(gscData.AbsolutePath, wd.Path) {
				foundWd = wd.Path
				relPath = strings.TrimPrefix(gscData.AbsolutePath, wd.Path)
				// Remove leading slash if present
				if strings.HasPrefix(relPath, "/") {
					relPath = relPath[1:]
				}
				break
			}
		}

		if foundWd == "" {
			m.debugLogger.Log("WARN", fmt.Sprintf("Could not determine working directory for %s", gscData.AbsolutePath))
			continue
		}

		files = append(files, ModifiedFileWithSHA{
			WorkingDir: foundWd,
			Path:       relPath,
			OldSHA:     gscData.OldBlobSHA,
			NewSHA:     gscData.NewBlobSHA,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no valid entries found in JSONL")
	}

	return files, nil
}

// extractModifiedFilesWithSHAs runs git commands to find modified files and their SHAs
// This method now delegates to GitProvider for consistency.
func (m *Manager) extractModifiedFilesWithSHAs(workdirs []WorkingDirectory) ([]ModifiedFileWithSHA, error) {
	// Use the existing GitProvider to get changes
	gitProvider := NewGitProvider(workdirs)
	changes, err := gitProvider.GetChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to get changes from GitProvider: %w", err)
	}

	var files []ModifiedFileWithSHA
	for absPath, prov := range changes {
		// Determine working directory and relative path
		var foundWd string
		var relPath string
		for _, wd := range workdirs {
			if strings.HasPrefix(absPath, wd.Path) {
				foundWd = wd.Path
				relPath = strings.TrimPrefix(absPath, wd.Path)
				if strings.HasPrefix(relPath, "/") {
					relPath = relPath[1:]
				}
				break
			}
		}

		if foundWd == "" {
			m.debugLogger.Log("WARN", fmt.Sprintf("Could not determine working directory for %s", absPath))
			continue
		}

		files = append(files, ModifiedFileWithSHA{
			WorkingDir: foundWd,
			Path:       relPath,
			OldSHA:     prov.OldBlobSHA,
			NewSHA:     prov.NewBlobSHA,
		})
	}

	return files, nil
}

// renderResumeTask renders the resume_task.md template with the modified files list
func (m *Manager) renderResumeTask(turn int, modifiedFiles []ModifiedFileWithSHA) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	templatePath := filepath.Join(gscHome, "cli", "templates", "claude", "intent-workflow", "change", "resume_task.md")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read resume task template: %w", err)
	}

	tmpl, err := template.New("resume_task").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse resume task template: %w", err)
	}

	var buf strings.Builder
	data := struct {
		ModifiedFiles []ModifiedFileWithSHA
		EnableCodeProvenance bool
	}{
		ModifiedFiles: modifiedFiles,
		EnableCodeProvenance: m.session.EnableCodeProvenance,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute resume task template: %w", err)
	}

	// Write rendered content to turn directory
	// Note: spawn.go must be updated to not overwrite this file if it exists
	turnDir := m.config.GetTurnDir(turn)
	resumeTaskPath := filepath.Join(turnDir, "resume_task.md")
	return os.WriteFile(resumeTaskPath, []byte(buf.String()), 0644)
}

// ModifiedFileWithSHA represents a modified file with its Git SHAs
type ModifiedFileWithSHA struct {
	WorkingDir string
	Path       string
	OldSHA     string
	NewSHA     string
}
