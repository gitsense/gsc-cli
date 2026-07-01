/**
 * Component: Notes Update Command
 * Block-UUID: a3b4c5d6-e7f8-9012-abcd-123456789012
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc notes update with --target flag for scoped writes.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */


package notes

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	var (
		id            string
		fromFile      string
		useStdin      bool
		summary       string
		content       string
		importance    string
		globs         []string
		tags          []string
		linkedFiles   []string
		topic         string
		relatedTopics []string
		targetValue   string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an existing note",
		Long: `Update an existing note with new content.

Provide the note ID and the new content one of three ways:
  --from-file <path>   Note-shaped JSON
  --stdin              Note-shaped JSON on stdin
  individual flags     --summary/--content/--glob/--tag/...

The note is validated, then the existing note is replaced.`,

		Example: `  # Update a note's summary
  gsc notes update --target personal --id <id> --summary "New summary"

  # Update a note from a JSON file
  gsc notes update --target repo --id <id> --from-file /tmp/note.json

  # Update a note's content
  gsc notes update --target personal --id <id> --content "Updated content"`,

		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}

			// Parse target
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			// Find existing note in target scope
			existing, err := notespkg.ResolveRecordFromTarget(id, target)
			if err != nil {
				return err
			}
			if existing == nil {
				return fmt.Errorf("note not found in %s store: %s", target, id)
			}

			var note notespkg.Note
			if fromFile != "" || useStdin {
				// JSON mode
				var data []byte
				var err error
				if useStdin {
					data, err = io.ReadAll(os.Stdin)
				} else {
					data, err = os.ReadFile(fromFile)
				}
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				if err := json.Unmarshal(data, &note); err != nil {
					return fmt.Errorf("invalid JSON: %w", err)
				}
			} else {
				// Flag mode - merge with existing
				note = *existing
				if cmd.Flags().Changed("summary") {
					note.Summary = summary
				}
				if cmd.Flags().Changed("content") {
					note.Content = content
				}
				if cmd.Flags().Changed("importance") {
					note.Importance = importance
				}
				if cmd.Flags().Changed("glob") {
					note.GlobPatterns = globs
				}
				if cmd.Flags().Changed("tag") {
					note.Tags = tags
				}
				if cmd.Flags().Changed("linked-file") {
					note.LinkedFiles = linkedFiles
				}
				if cmd.Flags().Changed("topic") {
					note.Topic = topic
				}
				if cmd.Flags().Changed("related-topic") {
					note.RelatedTopics = relatedTopics
				}
			}

			// Preserve identity
			note.ID = existing.ID
			note.SchemaVersion = existing.SchemaVersion
			note.CreatedAt = existing.CreatedAt

			// Update timestamp
			note.UpdatedAt = time.Now().UTC()

			// Validate and normalize
			result := notespkg.ValidateAndNormalize(note)
			if !result.Valid() {
				fmt.Println("Note content is invalid; nothing updated:")
				for _, err := range result.Errors {
					fmt.Printf("  ERROR %s\n", err)
				}
				return fmt.Errorf("note content is invalid")
			}

			note = result.Note
			note.Keywords = notespkg.KeywordsFor(note)
			note.ParentKeywords = notespkg.ParentKeywordsFor(note)

			// Load all records from target, replace the note
			records, err := notespkg.LoadRecordsFromTarget(target)
			if err != nil {
				return fmt.Errorf("failed to load notes: %w", err)
			}

			found := false
			for i, r := range records {
				if r.ID == note.ID {
					records[i] = note
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("note not found in %s store: %s", target, note.ID)
			}

			// Write records
			if err := notespkg.WriteRecordsToTarget(records, target); err != nil {
				return fmt.Errorf("failed to write notes: %w", err)
			}

			// Rebuild the Brain
			if err := notespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}

			fmt.Printf("Note updated in %s scope: %s\n", target, note.ID)
			fmt.Printf("Summary: %s\n", note.Summary)
			if len(note.Tags) > 0 {
				fmt.Printf("Tags: %s\n", note.Tags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Note ID to update (required)")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read note-shaped JSON content from a file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read note-shaped JSON content from stdin")
	cmd.Flags().StringVar(&summary, "summary", "", "Note summary")
	cmd.Flags().StringVar(&content, "content", "", "Note content")
	cmd.Flags().StringVar(&importance, "importance", "", "Importance: low, medium, or high")
	cmd.Flags().StringArrayVar(&globs, "glob", nil, "Glob pattern (repeatable)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag slug (repeatable)")
	cmd.Flags().StringArrayVar(&linkedFiles, "linked-file", nil, "Repo-relative related file (repeatable)")
	cmd.Flags().StringVar(&topic, "topic", "", "Primary topic slug")
	cmd.Flags().StringArrayVar(&relatedTopics, "related-topic", nil, "Related topic slug (max 2, repeatable)")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
