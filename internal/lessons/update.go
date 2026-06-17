/**
 * Component: Lessons Update Lifecycle
 * Block-UUID: 8f3b1c64-2a90-4d7e-bb15-0e9c6a2f5d41
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Stages, validates, and commits a full replacement of a committed lesson, preserving identity/provenance and re-deriving keywords.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0)
 */


package lessons

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/manifest"
)

// UpdateStage is the pending replacement: the new draft content plus the ID of
// the committed lesson it will replace. It lives in its own staging file so it
// never collides with an in-progress create draft.
type UpdateStage struct {
	TargetID string `json:"target_id"`
	Draft    Draft  `json:"draft"`
}

// UpdateDraftPath is the staging file for a pending lesson update.
func UpdateDraftPath() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tmp", "lesson-update.json"), nil
}

// ValidateUpdateDraft normalizes and validates staged update content.
func ValidateUpdateDraft(draft Draft) ValidationResult {
	return ValidateDraftValue(draft)
}

// WriteUpdateStage persists the pending update to its staging file.
func WriteUpdateStage(stage UpdateStage) (string, error) {
	if err := EnsureWorkspace(); err != nil {
		return "", err
	}
	path, err := UpdateDraftPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(stage, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// ReadUpdateStage loads the pending update. A missing stage returns (nil, path, nil).
func ReadUpdateStage() (*UpdateStage, string, error) {
	path, err := UpdateDraftPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, path, nil
	}
	if err != nil {
		return nil, path, err
	}
	var stage UpdateStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return nil, path, fmt.Errorf("invalid staged update: %w", err)
	}
	return &stage, path, nil
}

// DiscardUpdateStage removes the pending update if present.
func DiscardUpdateStage() (string, bool, error) {
	path, err := UpdateDraftPath()
	if err != nil {
		return "", false, err
	}
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return path, false, nil
	}
	if err := os.Remove(path); err != nil {
		return path, false, err
	}
	return path, true, nil
}

// CommitUpdate replaces the staged target record with the new content. It
// re-validates first (the do-not-trust gate) and refuses on any error, leaving
// the original record untouched. Identity (id, created_at, schema_version) and
// provenance (confirmed_by/at) are preserved; updated_at is bumped and keywords
// are re-derived. The stage is cleared on success.
func CommitUpdate() (*Record, error) {
	if err := EnsureWorkspace(); err != nil {
		return nil, err
	}
	stage, _, err := ReadUpdateStage()
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, fmt.Errorf("no staged lesson update; run 'gsc lessons update --id <id> --file <path>'")
	}

	result := ValidateUpdateDraft(stage.Draft)
	if !result.Valid() {
		return nil, fmt.Errorf("staged update is invalid; run 'gsc lessons update review'")
	}

	records, err := LoadRecords()
	if err != nil {
		return nil, err
	}
	index := -1
	for i := range records {
		if records[i].ID == stage.TargetID {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, fmt.Errorf("target lesson no longer exists: %s", stage.TargetID)
	}

	draft := result.Draft
	original := records[index]
	now := time.Now().UTC()
	updated := Record{
		ID:             original.ID,
		SchemaVersion:  original.SchemaVersion,
		CreatedAt:      original.CreatedAt,
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
		ConfirmedBy:    original.ConfirmedBy,
		ConfirmedAt:    original.ConfirmedAt,
	}
	records[index] = updated

	if err := WriteRecords(records); err != nil {
		return nil, err
	}
	manifestPath, err := RebuildManifest()
	if err != nil {
		return nil, err
	}
	if err := manifest.ImportManifest(context.Background(), manifestPath, DatabaseName, true, false); err != nil {
		return nil, err
	}

	if path, err := UpdateDraftPath(); err == nil {
		_ = os.Remove(path)
	}
	return &updated, nil
}
