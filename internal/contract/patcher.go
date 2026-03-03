/*
 * Component: GitSense Patch Engine
 * Block-UUID: a380da23-f19d-4098-b218-4d203064a9df
 * Parent-UUID: bce75ecd-b9a8-43fa-a746-c8c465324390
 * Version: 1.3.0
 * Description: Replaced fmt.Printf with logger.Warning to adhere to project logging standards. Retained improved error messages and single-file validation logic.
 * Language: Go
 * Created-at: 2026-03-03T05:20:17.968Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.1.0), Claude Haiku 4.5 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package contract

import (
	"bytes"
	"fmt"
	"strings"

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
