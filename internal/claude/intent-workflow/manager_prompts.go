/**
 * Component: Intent Workflow Manager Prompts
 * Block-UUID: 1a9f0d3e-5f6a-4b4c-9d8e-0f1a2b3c4d5e
 * Parent-UUID: 9a8f9c2d-4e5f-4a3b-8c7d-9e0f1a2b3c4f
 * Version: 1.6.0
 * Description: Updated template paths from data/templates to cli/templates to separate CLI-specific data from app-specific data.
 * Language: Go
 * Created-at: 2026-04-28T14:48:38.553Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0)
 */


package intent_workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// writeReferenceFilesNDJSON writes the reference files to an NDJSON file in the turn directory
func (m *Manager) writeReferenceFilesNDJSON() error {
	if len(m.session.ReferenceFilesContext) == 0 {
		return nil // No reference files to write
	}

	turnDir := m.config.GetTurnDir(m.currentTurn)
	refPath := filepath.Join(turnDir, "references.ndjson")
	file, err := os.Create(refPath)
	if err != nil {
		m.debugLogger.LogError("Failed to create references.ndjson", err)
		return fmt.Errorf("failed to create references.ndjson: %w", err)
	}
	defer file.Close()

	for _, ref := range m.session.ReferenceFilesContext {
		data, err := json.Marshal(ref)
		if err != nil {
			m.debugLogger.LogError("Failed to marshal reference file", err)
			return fmt.Errorf("failed to marshal reference file: %w", err)
		}
		if _, err := file.WriteString(string(data) + "\n"); err != nil {
			m.debugLogger.LogError("Failed to write reference file", err)
			return fmt.Errorf("failed to write reference file: %w", err)
		}
	}

	return nil
}

// formatReferenceFilesMetadata formats reference files for display in the prompt
func (m *Manager) formatReferenceFilesMetadata() string {
	if len(m.session.ReferenceFilesContext) == 0 {
		return "No reference files provided."
	}

	var sb strings.Builder
	sb.WriteString("The following reference files have been included:\n")
	for i, ref := range m.session.ReferenceFilesContext {
		sb.WriteString(fmt.Sprintf("- reference-file-%03d: %s (chat-id: %d, repo: %s)\n",
			i+1, ref.RelativePath, ref.ChatID, ref.Repository))
	}
	sb.WriteString("\n**Note:** Complete reference file data is available in `references.ndjson` if you need to examine raw content.\n")
	return sb.String()
}

// formatWorkingDirectories formats working directories for display in the prompt
func (m *Manager) formatWorkingDirectories() string {
	if len(m.session.WorkingDirectories) == 0 {
		return "No working directories provided."
	}

	var sb strings.Builder
	sb.WriteString("The following working directories will be searched:\n")
	for i, wd := range m.session.WorkingDirectories {
		sb.WriteString(fmt.Sprintf("- workdir-%03d: %s (path: %s)\n",
			i+1, wd.Name, wd.Path))
	}
	return sb.String()
}

// buildSystemPrompt reads and combines intent + shared + turn-specific prompts
func (m *Manager) buildSystemPrompt(gscHome string, turnType string) (string, error) {
	agentTemplatesPath := filepath.Join(gscHome, "cli", "templates", "claude", "intent-workflow")

	// Read intent prompt (workflow governance)
	intentPromptPath := filepath.Join(agentTemplatesPath, "shared", "intent_prompt.md")
	intentContent, err := os.ReadFile(intentPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read intent prompt: %w", err)
	}

	// Read shared prompt
	sharedPath := filepath.Join(agentTemplatesPath, "shared", "system_prompt_shared.md")
	sharedContent, err := os.ReadFile(sharedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read shared system prompt: %w", err)
	}

	// Read turn-specific prompt
	var turnPromptPath string
	if turnType == "discovery" {
		turnPromptPath = filepath.Join(agentTemplatesPath, "discovery", "system_prompt.md")
	} else if turnType == "change" {
		turnPromptPath = filepath.Join(agentTemplatesPath, "change", "system_prompt.md")
	} else {
		// Handle generic resume types (e.g., resume-change, resume-verify)
		baseType, isResume := parseTurnType(turnType)
		if isResume {
			// Use the base type's system prompt
			turnPromptPath = filepath.Join(agentTemplatesPath, baseType, "system_prompt.md")
		} else {
			return "", fmt.Errorf("unknown turn type: %s", turnType)
		}
	}
	turnContent, err := os.ReadFile(turnPromptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read turn-specific system prompt: %w", err)
	}

	// Combine prompts (tool capabilities now come from experts context file when available)
	combined := fmt.Sprintf(`# Agent System Prompt

This file combines intent workflow governance, shared principles, and turn-specific instructions.

---

# Intent Workflow

%s

---

# Shared Principles

%s

---

# %s Mission

%s
`, string(intentContent), string(sharedContent), turnType, string(turnContent))

	return combined, nil
}

// writePrompt generates and writes the task.md file for the current turn.
func (m *Manager) writePrompt(turnDir string, turn int, workdirsMarkdown string, refFilesMarkdown string, turnType string, orphanedFiles []string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	agentTemplatesPath := filepath.Join(gscHome, "cli", "templates", "claude", "intent-workflow")

	// Determine template file based on turn type
	var templatePath string
	if turnType == "discovery" {
		templatePath = filepath.Join(agentTemplatesPath, "discovery", "task.md")
	} else if turnType == "change" {
		templatePath = filepath.Join(agentTemplatesPath, "change", "task.md")
	} else {
		// Handle generic resume types (e.g., resume-change, resume-verify)
		_, isResume := parseTurnType(turnType)
		if isResume {
			// Resume turns use a pre-generated resume_task.md file
			// This file is copied by spawn.go based on the base type
			resumeTaskPath := filepath.Join(turnDir, "resume_task.md")
			content, err := os.ReadFile(resumeTaskPath)
			if err != nil {
				return fmt.Errorf("failed to read resume task: %w", err)
			}
			
			// Write the resume task content directly to task.md
			taskPath := filepath.Join(turnDir, "task.md")
			return os.WriteFile(taskPath, content, 0644)
		} else {
			return fmt.Errorf("unknown turn type: %s", turnType)
		}
	}

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read task template: %w", err)
	}

	// Read intent.md file from turn directory (consistent for all turn types)
	// This supports large intents efficiently and avoids inline injection issues
	intentPath := filepath.Join(turnDir, "intent.md")
	intentContent, err := os.ReadFile(intentPath)
	if err != nil {
		return fmt.Errorf("failed to read intent: %w", err)
	}

	// Read turn-history.json if it exists
	var turnHistoryJSON string
	var turnHistoryExists bool

	// Read selected-candidates.json if it exists (for selective validation)
	var reviewFilesJSON string
	var hasReviewFiles bool

	selectedCandPath := filepath.Join(turnDir, "selected-candidates.json")
	if data, err := os.ReadFile(selectedCandPath); err == nil {
		var selectedCands SelectedCandidates
		if err := json.Unmarshal(data, &selectedCands); err == nil {
			var fullPaths []string
			for _, cand := range selectedCands.Selected {
				// Find the workdir to resolve the full path
				for _, wd := range m.session.WorkingDirectories {
					if wd.ID == cand.WorkdirID {
						fullPath := filepath.Join(wd.Path, cand.FilePath)
						fullPaths = append(fullPaths, fullPath)
						break
					}
				}
			}
			if len(fullPaths) > 0 {
				reviewFilesBytes, _ := json.MarshalIndent(fullPaths, "", "  ")
				reviewFilesJSON = string(reviewFilesBytes)
				hasReviewFiles = true
			}
		}
	}

	turnHistoryPath := filepath.Join(turnDir, "turn-history.json")
	if data, err := os.ReadFile(turnHistoryPath); err == nil {
		turnHistoryJSON = string(data)
		turnHistoryExists = true
	}

	// Build pre-flight cleanup context
	var preFlightContext string
	if len(orphanedFiles) > 0 {
		var sb strings.Builder
		sb.WriteString("## Pre-Flight Cleanup\n\n")
		sb.WriteString("The following orphaned .change-meta.json files were found and removed:\n")
		for _, f := range orphanedFiles {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
		sb.WriteString("\n")
		sb.WriteString("This indicates a previous turn may have stopped unexpectedly. Some files may already contain the changes you're being asked to make. Please verify each file's current state before making changes.\n")
		preFlightContext = sb.String()
	} else {
		preFlightContext = "## Pre-Flight Cleanup\n\nNo orphaned metadata files found. Working directory is clean.\n"
	}

	// Compute Active Discovery variables for change turns
	var hasActiveDiscovery bool
	var isDiscoverySkipped bool
	var discoveryContext string

	if turnType == "change" {
		// Walk turns in reverse to find the most recent discovery turn
		for i := len(m.session.Turns) - 1; i >= 0; i-- {
			turn := m.session.Turns[i]
			if turn.TurnType == "discovery" {
				if turn.Status == "complete" && turn.Result != nil && turn.Result.Discovery != nil {
					hasActiveDiscovery = true
					// Serialize DiscoveryResult for template injection
					discoveryData, err := json.MarshalIndent(turn.Result.Discovery, "", "  ")
					if err != nil {
						m.debugLogger.LogError("Failed to marshal discovery context", err)
					} else {
						discoveryContext = string(discoveryData)
					}
				} else if turn.Status == "skipped" {
					isDiscoverySkipped = true
				}
				break // Found the most recent discovery turn
			}
		}
	}

	// Build experts mode context for discovery turns
	var expertsModeContext string
	if turnType == "discovery" {
		// Read codebase-overview.json to determine experts mode
		overviewPath := m.config.GetCodebaseOverviewFile()
		if overviewData, err := os.ReadFile(overviewPath); err == nil {
			var overview CodebaseOverview
			if err := json.Unmarshal(overviewData, &overview); err == nil {
				var sb strings.Builder
				sb.WriteString("## Discovery Mode Configuration\n\n")
				
				hasExpertsMode := false
				for _, wd := range overview.WorkingDirectories {
					if wd.ExpertsEnabled {
						hasExpertsMode = true
						sb.WriteString(fmt.Sprintf("### %s (workdir-%03d)\n", wd.Name, wd.ID))
						sb.WriteString(fmt.Sprintf("**Mode:** Experts Mode\n"))
						sb.WriteString(fmt.Sprintf("**Experts Context:** %s\n\n", wd.ExpertsContextPath))
						sb.WriteString(fmt.Sprintf("Before starting discovery for this workdir, you MUST read the experts context file:\n"))
						sb.WriteString(fmt.Sprintf("```\n"))
						sb.WriteString(fmt.Sprintf("cat %s\n", wd.ExpertsContextPath))
						sb.WriteString(fmt.Sprintf("```\n\n"))
					} else {
						sb.WriteString(fmt.Sprintf("### %s (workdir-%03d)\n", wd.Name, wd.ID))
						sb.WriteString(fmt.Sprintf("**Mode:** Generic Mode\n"))
						sb.WriteString(fmt.Sprintf("**Tools:** Use only grep, find, and Read. gsc tools are NOT available.\n\n"))
					}
				}
				
				if hasExpertsMode {
					sb.WriteString("**IMPORTANT:** The experts context file contains all available brain schemas, field definitions, and tool usage guides. You MUST read it before using any gsc tools.\n\n")
				}
				
				expertsModeContext = sb.String()
			}
		}
	}

	tmpl, err := template.New("task").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse task template: %w", err)
	}

	// Prepare template data
	data := struct {
		Workdirs             string
		RefFiles             string
		Intent               string
		TurnType             string
		TurnHistoryExists    bool
		TurnHistoryJSON      string
		ReviewFilesJSON      string
		HasReviewFiles       bool
		PreFlightContext     string
		HasActiveDiscovery   bool
		IsDiscoverySkipped   bool
		DiscoveryContext     string
		ExpertsModeContext   string
		TurnDir              string // Absolute path to turn directory for robust file resolution
	}{
		Workdirs:           workdirsMarkdown,
		RefFiles:           refFilesMarkdown,
		Intent:             string(intentContent),
		TurnType:           turnType,
		TurnHistoryExists:  turnHistoryExists,
		TurnHistoryJSON:    turnHistoryJSON,
		ReviewFilesJSON:    reviewFilesJSON,
		HasReviewFiles:     hasReviewFiles,
		PreFlightContext:   preFlightContext,
		HasActiveDiscovery: hasActiveDiscovery,
		IsDiscoverySkipped: isDiscoverySkipped,
		DiscoveryContext:   discoveryContext,
		ExpertsModeContext: expertsModeContext,
		TurnDir:            turnDir, // Absolute path to turn directory
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute task template: %w", err)
	}

	// Write to task.md
	taskPath := filepath.Join(turnDir, "task.md")
	return os.WriteFile(taskPath, []byte(buf.String()), 0644)
}

// copyFile copies a file from src to dst
func (m *Manager) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// WriteAgentPermissions writes the agent permissions file to restrict Bash commands
func (m *Manager) WriteAgentPermissions(turnDir string) error {
	permissionsPath := filepath.Join(turnDir, ".agent-permissions")
	// For now, we allow all commands, but this file can be used to restrict
	// specific commands in the future.
	// The current implementation in spawn.go just writes an empty file or specific rules.
	// Based on spawn.go context, it seems to be a placeholder or specific to the bash script.
	// We will write a basic permission set.
	permissions := `{
  "allowed_commands": ["gsc"],
  "allowed_paths": ["."]
}`
	return os.WriteFile(permissionsPath, []byte(permissions), 0644)
}

// getFormatFile returns the filesystem path of the response_format.md
// specification for the given turn type, or an empty string for unrecognised
// turn types.
func getFormatFile(gscHome, turnType string) string {
	agentTemplatesPath := filepath.Join(gscHome, "cli", "templates", "claude", "intent-workflow")
	switch turnType {
	case "discovery":
		return filepath.Join(agentTemplatesPath, "discovery", "response_format.md")
	case "change":
		return filepath.Join(agentTemplatesPath, "change", "response_format.md")
	default:
		return ""
	}
}
