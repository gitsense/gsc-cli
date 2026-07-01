/**
 * Component: Notes Add Command
 * Block-UUID: f2a3b4c5-d6e7-8901-fabc-012345678901
 * Parent-UUID: N/A
 * Version: 2.0.0
 * Description: Implements gsc notes add with --target flag for scoped writes.
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

const noteTemplate = `{
  "summary": "Short description of the note",
  "content": "Detailed note content",
  "topic": "registered-topic-slug",
  "glob_patterns": ["path/to/files/**"],
  "tags": ["category"],
  "linked_files": ["specific/file.go"],
  "importance": "medium"
}
`

func addCmd() *cobra.Command {
	var (
		fromFile      string
		useStdin      bool
		showTemplate  bool
		summary       string
		content       string
		importance    string
		topic         string
		relatedTopics []string
		globs         []string
		tags          []string
		linkedFiles   []string
		targetValue   string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a note in one shot from flags, a file, or stdin",
		Long: `Create a note without hand-editing a draft file.

Provide the content one of four ways:
  --template           Print a note template and exit
  --from-file <path>   Note-shaped JSON
  --stdin              Note-shaped JSON on stdin
  individual flags     --summary/--content/--glob/--tag/...

The content is validated, then committed to the notes store.`,
		Example: `  # Print a template
  gsc notes add --template

  # From individual flags
  gsc notes add --target personal --glob "internal/cli/**" \
    --summary "CLI architecture notes" \
    --content "The CLI uses cobra for command handling..."

  # From a JSON file
  gsc notes add --target repo --from-file /tmp/note.json

  # From stdin
  cat note.json | gsc notes add --target personal --stdin`,

		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse target early
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}

			if showTemplate {
				fmt.Print(noteTemplate)
				return nil
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
				// Flag mode
				if importance == "" {
					importance = "medium"
				}
				note = notespkg.Note{
					Summary:       summary,
					Content:       content,
					Topic:         topic,
					RelatedTopics: relatedTopics,
					Importance:    importance,
					GlobPatterns:  globs,
					Tags:          tags,
					LinkedFiles:   linkedFiles,
				}
			}

			// Validate and normalize
			result := notespkg.ValidateAndNormalize(note)
			if !result.Valid() {
				fmt.Println("Note content is invalid; nothing committed:")
				for _, err := range result.Errors {
					fmt.Printf("  ERROR %s\n", err)
				}
				return fmt.Errorf("note content is invalid")
			}

			// Generate ID and timestamps
			now := time.Now().UTC()
			id, err := notespkg.NewNoteID(now)
			if err != nil {
				return fmt.Errorf("failed to generate note ID: %w", err)
			}

			note = result.Note
			note.ID = id
			note.SchemaVersion = "1.0.0"
			note.CreatedAt = now
			note.UpdatedAt = now
			note.Keywords = notespkg.KeywordsFor(note)
			note.ParentKeywords = notespkg.ParentKeywordsFor(note)

			// Commit
			if err := notespkg.AppendRecordToTarget(note, target); err != nil {
				return fmt.Errorf("failed to commit note: %w", err)
			}

			// Rebuild the Brain
			if err := notespkg.RebuildAndImportForTarget(target); err != nil {
				fmt.Printf("Warning: failed to rebuild Brain: %v\n", err)
			}

			// Show destination
			recordsPath, _ := notespkg.RecordsPathForTarget(target)
			fmt.Printf("Note written to %s scope: %s\n", target, recordsPath)
			fmt.Printf("Note committed: %s\n", note.ID)
			fmt.Printf("Summary: %s\n", note.Summary)
			if len(note.Tags) > 0 {
				fmt.Printf("Tags: %s\n", note.Tags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read note-shaped JSON content from a file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read note-shaped JSON content from stdin")
	cmd.Flags().BoolVar(&showTemplate, "template", false, "Print a note template and exit")
	cmd.Flags().StringVar(&summary, "summary", "", "Note summary (required)")
	cmd.Flags().StringVar(&content, "content", "", "Note content")
	cmd.Flags().StringVar(&importance, "importance", "", "Importance: low, medium, or high (default medium)")
	cmd.Flags().StringVar(&topic, "topic", "", "Primary topic slug (required)")
	cmd.Flags().StringArrayVar(&relatedTopics, "related-topic", nil, "Related topic slug (max 2, repeatable)")
	cmd.Flags().StringArrayVar(&globs, "glob", nil, "Glob pattern (repeatable)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag slug (repeatable)")
	cmd.Flags().StringArrayVar(&linkedFiles, "linked-file", nil, "Repo-relative related file (repeatable)")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")
	return cmd
}
