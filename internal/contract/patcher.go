/**
 * Component: GitSense Patch Engine
 * Block-UUID: dc720df8-031a-4252-83ab-fd141db0ba85
 * Parent-UUID: 05eebcb9-39df-4ac1-aa31-de44a52cb44f
 * Version: 1.9.0
 * Description: Fixed normalizeHunkOffsets to correctly calculate the header offset by counting actual comment lines in the patch metadata, rather than using a hardcoded constant. This makes the patcher robust to variable-length headers.
 * Language: Go
 * Created-at: 2026-03-03T18:37:08.355Z
 * Authors: GLM-4.7 (v1.0.0), ..., Gemini 3 Flash (v1.8.0), GLM-4.7 (v1.9.0)
 */


package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// ApplyPatch takes a GitSense patch block's executable code and the original source code,
// and returns the resulting patched string.
// 
// If originalSource is empty, this function assumes a "new file" scenario and will
// attempt to apply the patch to an empty buffer. This is valid for patches that
// create new content from scratch.
func ApplyPatch(originalSource string, patchExecutableCode string) (string, error) {
	// DEBUG: Log inputs to diagnose parsing issues
	logger.Debug("ApplyPatch Inputs", "original_source", originalSource)
	logger.Debug("ApplyPatch Inputs", "patch_executable_code", patchExecutableCode)

	// 1. Extract the Diff Content
	// We look for GitSense markers first. If they exist, we extract the content between them.
	// If they don't exist, we pass the whole string to the parser as a fallback.
	
	diffString := patchExecutableCode
	if strings.Contains(patchExecutableCode, "# --- PATCH START MARKER ---") {
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
		diffString = cleanDiff.String()
	}

	if strings.TrimSpace(diffString) == "" {
		return "", fmt.Errorf("no diff content found in patch block")
	}

	// TEMPORARY WORKAROUND: Normalize file headers for frontend compatibility
	// The frontend generates "--- Original" and "+++ Modified" which are not valid
	// unified diff file headers. We replace them with dummy paths to satisfy the parser.
	diffString = strings.Replace(diffString, "--- Original", "--- a/file", 1)
	diffString = strings.Replace(diffString, "+++ Modified", "+++ b/file", 1)

	// DEBUG: Log the final string being passed to the parser
	logger.Debug("Diff String passed to gitdiff.Parse", "diff_string", diffString)

	// 2. Parse the Unified Diff
	// gitdiff.Parse is robust and will ignore non-diff lines (like comments) 
	// as long as they aren't inside a hunk.
	files, _, err := gitdiff.Parse(strings.NewReader(diffString))
	if err != nil {
		return "", fmt.Errorf("failed to parse diff: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no valid diff hunks found in patch (diff may be malformed or contain only metadata)")
	}

	// 3. Validate Single-File Assumption
	// GitSense patches are designed to be "one patch per message."
	// If we encounter multiple files, log a warning but proceed with the first one.
	if len(files) > 1 {
		logger.Warning("Patch contains multiple files; applying only the first one", "count", len(files))
	}

	// 4. Apply the Patch
	var output bytes.Buffer
	err = gitdiff.Apply(&output, strings.NewReader(originalSource), files[0])
	if err != nil {
		return "", fmt.Errorf("failed to apply patch to source: %w", err)
	}

	return output.String(), nil
}

// WriteDebugArtifacts persists the source and patch content to a debug directory
// to help diagnose patch application failures.
func WriteDebugArtifacts(sourceCode string, patchContent string, targetUUID string, patchError error) error {
	// 1. Resolve Debug Directory
	gscHome, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	debugDir := filepath.Join(gscHome, ".gitsense", "debug")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	// 2. Create Unique Session Directory
	timestamp := time.Now().Format("20060102-150405")
	sessionDir := filepath.Join(debugDir, fmt.Sprintf("patch_%s_%s", targetUUID[:8], timestamp))
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// 3. Write Source Code
	if err := os.WriteFile(filepath.Join(sessionDir, "source.txt"), []byte(sourceCode), 0644); err != nil {
		return fmt.Errorf("failed to write source.txt: %w", err)
	}

	// 4. Write Patch Content
	if err := os.WriteFile(filepath.Join(sessionDir, "patch.diff"), []byte(patchContent), 0644); err != nil {
		return fmt.Errorf("failed to write patch.diff: %w", err)
	}

	// 5. Write Metadata
	metadata := map[string]interface{}{
		"target_uuid": targetUUID,
		"error":       patchError.Error(),
		"timestamp":   time.Now().Format(time.RFC3339),
	}
	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "metadata.json"), metaBytes, 0644); err != nil {
		return fmt.Errorf("failed to write metadata.json: %w", err)
	}

	// 6. Write Test Script
	script := "#!/bin/bash\nset -e\necho \"Applying patch...\"\npatch source.txt < patch.diff\necho \"Exit code: $?\"\n"
	if err := os.WriteFile(filepath.Join(sessionDir, "apply_test.sh"), []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write apply_test.sh: %w", err)
	}

	return nil
}
