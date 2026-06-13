/**
 * Component: Lessons Commit Workflow
 * Block-UUID: a68214db-d160-4afd-a56d-69fe43cc293a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Orchestrates lesson commit and refresh workflows, including validation, canonical storage, draft archival, manifest rebuild, and Brain import.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package lessons

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/manifest"
)

func CommitDraft(confirmedBy string) (*Record, string, error) {
	if err := EnsureWorkspace(); err != nil {
		return nil, "", err
	}
	path, err := DraftPath()
	if err != nil {
		return nil, "", err
	}
	result := ReadAndValidateDraft(path)
	if !result.Valid() {
		return nil, "", fmt.Errorf("draft is invalid; run 'gsc lessons review'")
	}

	now := time.Now().UTC()
	id, err := NewLessonID(now)
	if err != nil {
		return nil, "", err
	}
	draft := result.Draft
	record := Record{
		ID:             id,
		SchemaVersion:  "1.0.0",
		CreatedAt:      now,
		UpdatedAt:      now,
		Summary:        draft.Summary,
		Details:        draft.Details,
		AppliesTo:      draft.AppliesTo,
		Tags:           draft.Tags,
		Keywords:       keywordsFor(draft),
		ParentKeywords: parentKeywordsFor(draft),
		Importance:     draft.Importance,
		ReviewChecks:   draft.ReviewChecks,
		AI:             draft.AI,
		ConfirmedBy:    confirmedBy,
		ConfirmedAt:    now,
	}

	if err := AppendRecord(record); err != nil {
		return nil, "", err
	}
	archivePath, err := archiveCommittedDraft(path, id)
	if err != nil {
		return nil, "", err
	}
	manifestPath, err := RebuildManifest()
	if err != nil {
		return nil, "", err
	}
	if err := manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false); err != nil {
		return nil, "", err
	}
	return &record, archivePath, nil
}

func RebuildAndImport() error {
	if err := EnsureWorkspace(); err != nil {
		return err
	}
	manifestPath, err := RebuildManifest()
	if err != nil {
		return err
	}
	return manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false)
}

func RebuildAndImportFromRecords(records []Record) error {
	if err := EnsureWorkspace(); err != nil {
		return err
	}
	manifestPath, err := RebuildManifestFromRecords(records)
	if err != nil {
		return err
	}
	return manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false)
}

func archiveCommittedDraft(path string, id string) (string, error) {
	archiveDir, err := ArchiveDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return "", err
	}
	archivePath := filepath.Join(archiveDir, id+".json")
	if err := os.Rename(path, archivePath); err != nil {
		return "", err
	}
	return archivePath, nil
}
