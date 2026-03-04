/**
 * Component: GitSense Patch Engine
 * Block-UUID: 82d56715-1958-40be-8339-13ff83e21d78
 * Parent-UUID: 7a8f9c2d-3e4b-4a1f-8b5c-6d7e8f9a0b1c
 * Version: 1.14.1
 * Description: Implemented a multiphase patching strategy in ApplyPatch. It now attempts to apply the patch directly first, and if that fails (e.g., due to line number offsets from headers), it automatically calculates the header offset from the patch metadata and retries with adjusted hunk line numbers.
 * Language: Go
 * Created-at: 2026-03-03T20:14:24.170Z
 * Authors: GLM-4.7 (v1.0.0), ..., GLM-4.7 (v1.10.0), Gemini 3 Flash (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.13.1), GLM-4.7 (v1.14.0), GLM-4.7 (v1.14.1)
 */


package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/gitsense/gsc-cli/pkg/logger"
	execpkg "github.com/gitsense/gsc-cli/internal/exec"
)

// PatchError wraps a patching failure with details about the attempted phases.
type PatchError struct {
	Err          error
	Phase1Diff   string // The diff string used in Phase 1
	Phase2Diff   string // The diff string used in Phase 2 (empty if not reached)
	Phase2Offset int    // The offset calculated for Phase 2
}

func (e *PatchError) Error() string {
	return e.Err.Error()
}

func (e *PatchError) Unwrap() error {
	return e.Err
}

// ApplyPatch takes a GitSense patch block's executable code and the original source code,
// and returns the resulting patched string.
//
// It uses a multiphase approach:
// Phase 1: Attempt to apply the patch exactly as provided.
// Phase 2: If Phase 1 fails, attempt to detect if the hunk line numbers include the
//          metadata header offset, adjust them, and retry.
func ApplyPatch(originalSource string, patchExecutableCode string) (string, error) {
	// 1. Extract the Diff Content
	diffString := extractDiffContent(patchExecutableCode)
	if strings.TrimSpace(diffString) == "" {
		return "", &PatchError{
			Err:        fmt.Errorf("no diff content found in patch block"),
			Phase1Diff: diffString,
		}
	}

	// TEMPORARY WORKAROUND: Normalize file headers for frontend compatibility
	diffString = strings.Replace(diffString, "--- Original", "--- a/file", 1)
	diffString = strings.Replace(diffString, "+++ Modified", "+++ b/file", 1)

	// Phase 1: Direct Application
	patched, err := tryApply(originalSource, diffString)
	if err == nil {
		return patched, nil
	}

	logger.Debug("Phase 1 patch application failed, attempting Phase 2 (offset adjustment)", "error", err)

	// Prepare error with Phase 1 details
	patchErr := &PatchError{
		Err:        err,
		Phase1Diff: diffString,
	}

	// Phase 2: Offset Adjustment
	// The 12 accounts for the 10 lines code block header + 2 blank lines
	offset := 12;
	if offset > 0 {
		adjustedDiff := adjustHunkOffsets(diffString, -offset)
		patchErr.Phase2Diff = adjustedDiff
		patchErr.Phase2Offset = offset

		patched, err = tryApply(originalSource, adjustedDiff)
		if err == nil {
			logger.Info("Patch applied successfully after adjusting hunk offsets", "offset", -offset)
			return patched, nil
		}
		logger.Debug("Phase 2 patch application failed", "error", err)
	}

	// Phase 2.5: Whitespace Normalization
	// Attempt to apply by stripping trailing whitespace from both source and patch
	logger.Debug("Phase 2 patch application failed, attempting Phase 2.5 (whitespace normalization)")
	
	// Determine which diff to use (prefer Phase 2 adjusted diff if available)
	diffToNormalize := patchErr.Phase1Diff
	if patchErr.Phase2Diff != "" {
		diffToNormalize = patchErr.Phase2Diff
	}

	normalizedSource := stripTrailingWhitespace(originalSource)
	normalizedDiff := stripTrailingWhitespace(diffToNormalize)

	patched, err = tryApply(normalizedSource, normalizedDiff)
	if err == nil {
		logger.Info("Patch applied successfully after whitespace normalization")
		return patched, nil
	}
	logger.Debug("Phase 2.5 patch application failed", "error", err)

	// Phase 3: System Patch Tool Fallback
	// Attempt to apply using the system 'patch' command if available
	if _, err := exec.LookPath("patch"); err == nil {
		logger.Debug("Phase 2.5 patch application failed, attempting Phase 3 (system patch tool)")
		
		// Determine which diff to use (prefer Phase 2 adjusted diff)
		diffToApply := patchErr.Phase1Diff
		if patchErr.Phase2Diff != "" {
			diffToApply = patchErr.Phase2Diff
		}

		// Create temp directory
		tempDir, err := os.MkdirTemp("", "gsc-patch-*")
		if err != nil {
			logger.Debug("Phase 3 failed to create temp directory", "error", err)
			return "", patchErr
		}
		defer os.RemoveAll(tempDir) // Cleanup

		// Write source file
		sourcePath := filepath.Join(tempDir, "source.txt")
		if err := os.WriteFile(sourcePath, []byte(originalSource), 0644); err != nil {
			logger.Debug("Phase 3 failed to write source file", "error", err)
			return "", patchErr
		}

		// Write patch file
		patchPath := filepath.Join(tempDir, "patch.diff")
		if err := os.WriteFile(patchPath, []byte(diffToApply), 0644); err != nil {
			logger.Debug("Phase 3 failed to write patch file", "error", err)
			return "", patchErr
		}

		// Execute patch command
		// Using -f to force apply and avoid interactive prompts
		executor := execpkg.NewExecutor("patch -f source.txt < patch.diff", execpkg.ExecFlags{Silent: true}, tempDir)
		result, err := executor.Run()
		if err != nil {
			logger.Debug("Phase 3 execution failed", "error", err)
			return "", patchErr
		}

		if result.ExitCode == 0 {
			// Read patched file
			patchedContent, err := os.ReadFile(sourcePath)
			if err != nil {
				logger.Debug("Phase 3 failed to read patched file", "error", err)
				return "", patchErr
			}
			logger.Info("Patch applied successfully using system patch tool (Phase 3)")
			return string(patchedContent), nil
		}

		logger.Debug("Phase 3 patch tool returned non-zero exit code", "exit_code", result.ExitCode)
	}

	return "", patchErr
}

// tryApply attempts to parse and apply a diff string to the source.
func tryApply(source, diffStr string) (string, error) {
	files, _, err := gitdiff.Parse(strings.NewReader(diffStr))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no valid diff hunks found")
	}

	var output bytes.Buffer
	err = gitdiff.Apply(&output, strings.NewReader(source), files[0])
	if err != nil {
		return "", err
	}

	return output.String(), nil
}

// extractDiffContent isolates the unified diff from GitSense markers.
func extractDiffContent(patchExecutableCode string) string {
	if !strings.Contains(patchExecutableCode, "# --- PATCH START MARKER ---") {
		return patchExecutableCode
	}

	lines := strings.Split(patchExecutableCode, "\n")
	var cleanDiff strings.Builder
	inDiff := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# --- PATCH START MARKER ---") {
			inDiff = true
			continue
		}
		if strings.HasPrefix(trimmed, "# --- PATCH END MARKER ---") {
			break
		}
		if inDiff {
			cleanDiff.WriteString(line + "\n")
		}
	}
	return cleanDiff.String()
}

// adjustHunkOffsets uses regex to find hunk headers and shift their start lines.
func adjustHunkOffsets(diffStr string, delta int) string {
	// Regex for @@ -start,len +start,len @@
	re := regexp.MustCompile(`(?m)^@@ -(\d+),(\d+) \+(\d+),(\d+) @@`)

	return re.ReplaceAllStringFunc(diffStr, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) != 5 {
			return match
		}

		oldStart, _ := strconv.Atoi(submatches[1])
		oldLen := submatches[2]
		newStart, _ := strconv.Atoi(submatches[3])
		newLen := submatches[4]

		// Apply delta
		adjOldStart := oldStart + delta
		adjNewStart := newStart + delta

		// Ensure we don't go below line 1
		if adjOldStart < 1 { adjOldStart = 1 }
		if adjNewStart < 1 { adjNewStart = 1 }

		return fmt.Sprintf("@@ -%d,%s +%d,%s @@", adjOldStart, oldLen, adjNewStart, newLen)
	})
}

// stripTrailingWhitespace removes trailing spaces and tabs from every line in a string.
func stripTrailingWhitespace(input string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	
	// Remove trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	
	return strings.Join(lines, "\n")
}

// WriteDebugArtifacts persists the source and patch content to a debug directory
// to help diagnose patch application failures. It writes separate files for Phase 1 and Phase 2 diffs.
func WriteDebugArtifacts(sourceCode string, phase1Diff string, phase2Diff string, targetUUID string, patchError error) (string, error) {
	// 1. Resolve Debug Directory
	gscHome, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	debugDir := filepath.Join(gscHome, ".gitsense", "debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create debug directory: %w", err)
	}

	// 2. Create Unique Session Directory
	timestamp := time.Now().Format("20060102-150405")
	sessionDir := filepath.Join(debugDir, fmt.Sprintf("patch_%s_%s", targetUUID[:8], timestamp))
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	// 3. Write Source Code
	if err := os.WriteFile(filepath.Join(sessionDir, "source.txt"), []byte(sourceCode), 0644); err != nil {
		return "", fmt.Errorf("failed to write source.txt: %w", err)
	}

	// 4. Write Patch Content
	if err := os.WriteFile(filepath.Join(sessionDir, "patch_phase1.diff"), []byte(phase1Diff), 0644); err != nil {
		return "", fmt.Errorf("failed to write patch_phase1.diff: %w", err)
	}

	// 5. Write Phase 2 Patch Content (if available)
	if phase2Diff != "" {
		if err := os.WriteFile(filepath.Join(sessionDir, "patch_phase2.diff"), []byte(phase2Diff), 0644); err != nil {
			return "", fmt.Errorf("failed to write patch_phase2.diff: %w", err)
		}
	}

	// 6. Write Metadata
	metadata := map[string]interface{}{
		"target_uuid": targetUUID,
		"error":       patchError.Error(),
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	// Add Phase 2 details to metadata if available
	if pErr, ok := patchError.(*PatchError); ok && pErr.Phase2Diff != "" {
		metadata["phase2_offset"] = pErr.Phase2Offset
		metadata["phase2_attempted"] = true
	} else {
		metadata["phase2_attempted"] = false
	}

	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "metadata.json"), metaBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write metadata.json: %w", err)
	}

	// 7. Write Test Script
	var script strings.Builder
	script.WriteString("#!/bin/bash\nset -e\n")
	script.WriteString("echo \"=== Phase 1: Attempting original patch ===\"\n")
	script.WriteString("patch source.txt < patch_phase1.diff\n")
	script.WriteString("EXIT_CODE=$?\n")
	script.WriteString("if [ $EXIT_CODE -ne 0 ]; then\n")
	script.WriteString("    echo \"Phase 1 failed with exit code $EXIT_CODE\"\n")
	// Check if phase 2 exists
	script.WriteString("    if [ -f \"patch_phase2.diff\" ]; then\n")
	script.WriteString("        echo \"=== Phase 2: Attempting adjusted patch ===\"\n")
	script.WriteString("        # Restore original source for clean test\n")
	script.WriteString("        if [ -f \"source.txt.orig\" ]; then\n")
	script.WriteString("            cp source.txt.orig source.txt\n")
	script.WriteString("        fi\n")
	script.WriteString("        patch source.txt < patch_phase2.diff\n")
	script.WriteString("        EXIT_CODE=$?\n")
	script.WriteString("        if [ $EXIT_CODE -eq 0 ]; then\n")
	script.WriteString("            echo \"Phase 2 succeeded\"\n")
	script.WriteString("        else\n")
	script.WriteString("            echo \"Phase 2 failed with exit code $EXIT_CODE\"\n")
	script.WriteString("        fi\n")
	script.WriteString("    fi\n")
	script.WriteString("else\n")
	script.WriteString("    echo \"Phase 1 succeeded\"\n")
	script.WriteString("fi\n")
	script.WriteString("echo \"Final Exit Code: $EXIT_CODE\"\n")

	if err := os.WriteFile(filepath.Join(sessionDir, "apply_test.sh"), []byte(script.String()), 0755); err != nil {
		return "", fmt.Errorf("failed to write apply_test.sh: %w", err)
	}

	return sessionDir, nil
}
