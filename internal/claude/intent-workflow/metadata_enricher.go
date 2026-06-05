/**
 * Component: Metadata Enricher
 * Block-UUID: ece2bc34-0e05-4fbe-b8d4-c72e42977778
 * Parent-UUID: 1e5b1e64-2320-4713-afbf-e4c791f7e1e4
 * Version: 1.3.0
 * Description: Orchestrates post-processing of .change-meta.json files after a change turn. Performs three-way validation (AI claims vs git truth vs metadata presence), enriches meta files with CLI-derived fields (OldBlobSHA, NewBlobSHA, ChangeType, Language), and manages meta file cleanup. Updated Validate() to return otherChanges (files changed by user or other processes) for neutral UI attribution instead of stderr warnings. Added missing strings import.
 * Language: Go
 * Created-at: 2026-04-29T02:41:15.835Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package intent_workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MetadataEnricher orchestrates validation, enrichment, and cleanup of
// .change-meta.json files produced by the AI during a change turn.
type MetadataEnricher struct {
	workdirs    []WorkingDirectory
	gitProvider *GitProvider
}

// NewMetadataEnricher creates a MetadataEnricher for the given working directories.
func NewMetadataEnricher(workdirs []WorkingDirectory) *MetadataEnricher {
	return &MetadataEnricher{
		workdirs:    workdirs,
		gitProvider: NewGitProvider(workdirs),
	}
}

// Validate performs a three-way comparison between:
//  1. AI's claimed files  (claimedFiles from the change response)
//  2. Git ground truth    (gitChanges from GitProvider.GetChanges)
//  3. Metadata presence   (metaFiles found on disk)
//
// Returns ErrorDetails when any claimed file lacks its .change-meta.json.
// Also returns otherChanges (files changed by user or other processes) for UI attribution.
// Ghost metadata files produce stderr warnings only.
func (e *MetadataEnricher) Validate(
	claimedFiles []FileModified,
	gitChanges map[string]FileProvenance,
	metaFiles []string,
) (*ErrorDetails, []FileModified, error) {
	// Build set of absolute paths covered by meta files
	coveredFiles := make(map[string]bool)
	for _, metaFilePath := range metaFiles {
		gscData, err := readAndParseMetaFile(metaFilePath)
		if err != nil {
			return nil, nil, err
		}
		coveredFiles[gscData.AbsolutePath] = true
	}

	// Build set of claimed absolute paths for cross-checks
	claimedPaths := make(map[string]bool)
	for _, f := range claimedFiles {
		claimedPaths[filepath.Join(f.WorkingDir, f.Path)] = true
	}

	// Check 1: Missing metadata - AI claimed a file but no meta file exists
	var missing []ErrorFile
	for _, f := range claimedFiles {
		absPath := filepath.Join(f.WorkingDir, f.Path)
		if !coveredFiles[absPath] {
			missing = append(missing, ErrorFile{
				FilePath:     f.Path,
				WorkingDir:   f.WorkingDir,
				Reason:       "Missing .change-meta.json file",
				ExpectedPath: "." + filepath.Base(f.Path) + ".change-meta.json",
				Resumable:    true,
			})
		}
	}
	if len(missing) > 0 {
		return &ErrorDetails{
			ErrorCode:  "MISSING_CHANGE_METADATA",
			Message:    fmt.Sprintf("Missing .change-meta.json files for %d file(s)", len(missing)),
			ErrorFiles: missing,
		}, nil, nil
	}

	// Check 2: Other changes - git changed a file the AI didn't report
	// These are captured for UI attribution (not an error)
	var otherChanges []FileModified
	for absPath, prov := range gitChanges {
		if !claimedPaths[absPath] {
			// Determine working directory and relative path
			var workingDir string
			var relPath string
			for _, wd := range e.workdirs {
				if strings.HasPrefix(absPath, wd.Path+string(os.PathSeparator)) {
					workingDir = wd.Path
					relPath = strings.TrimPrefix(absPath, wd.Path)
					if strings.HasPrefix(relPath, string(os.PathSeparator)) {
						relPath = relPath[1:]
					}
					break
				}
			}

			otherChanges = append(otherChanges, FileModified{
				WorkingDir:   workingDir,
				Path:         relPath,
				Status:       prov.ChangeType,
				OldBlobSHA:   prov.OldBlobSHA,
				NewBlobSHA:   prov.NewBlobSHA,
				LinesAdded:   prov.LinesAdded,
				LinesDeleted: prov.LinesDeleted,
			})
		}
	}

	// Check 3: Ghost metadata - meta file exists but git shows no change
	for _, metaFilePath := range metaFiles {
		gscData, err := readAndParseMetaFile(metaFilePath)
		if err != nil {
			continue
		}
		if _, ok := gitChanges[gscData.AbsolutePath]; !ok {
			fmt.Fprintf(os.Stderr, "WARNING: .change-meta.json exists for %s but git shows no change\n", gscData.AbsolutePath)
		}
	}

	return nil, otherChanges, nil
}

// Enrich reads each .change-meta.json (absolute_path + description only, as
// written by the AI), enriches it with OldBlobSHA, NewBlobSHA, ChangeType,
// and Language, then overwrites the file with the complete JSON.
func (e *MetadataEnricher) Enrich(metaFiles []string, gitChanges map[string]FileProvenance) error {
	for _, metaFilePath := range metaFiles {
		gscData, err := readAndParseMetaFile(metaFilePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", metaFilePath, err)
		}

		if prov, ok := gitChanges[gscData.AbsolutePath]; ok {
			gscData.OldBlobSHA = prov.OldBlobSHA
			gscData.NewBlobSHA = prov.NewBlobSHA
			gscData.ChangeType = prov.ChangeType
		}

		gscData.Language = DetectLanguage(gscData.AbsolutePath)

		enriched, err := json.MarshalIndent(gscData, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", metaFilePath, err)
		}

		if err := os.WriteFile(metaFilePath, enriched, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", metaFilePath, err)
		}
	}

	return nil
}

// Cleanup removes meta files from disk unless keepFiles is true.
func (e *MetadataEnricher) Cleanup(metaFiles []string, keepFiles bool) error {
	if keepFiles {
		return nil
	}
	for _, metaFilePath := range metaFiles {
		if err := os.Remove(metaFilePath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", metaFilePath, err)
		}
	}
	return nil
}
