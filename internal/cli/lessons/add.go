/**
 * Component: Lessons Add Command
 * Block-UUID: 6b4d2f81-0c93-4e57-a8d6-3f1b9c7e0a25
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc lessons add, the one-shot create path from flags, a file, or stdin that stages a draft for commit.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0)
 */

package lessons

import (
	"fmt"
	"io"
	"os"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

// contentFlags are the per-field flags that build a draft directly.
var addContentFlags = []string{
	"summary", "details", "importance", "file", "linked-file",
	"command", "topic", "related-topic", "tag", "review-check", "provider", "model-id", "agent",
}

func addCmd() *cobra.Command {
	var (
		fromFile string
		useStdin bool
		replace  bool

		summary       string
		details       string
		importance    string
		files         []string
		linkedFiles   []string
		commands      []string
		topic         string
		relatedTopics []string
		tags          []string
		reviewChecks  []string
		provider      string
		modelID       string
		agent         string
	)
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a lesson in one shot from flags, a file, or stdin",
		Long: `Create a lesson without hand-editing the draft file.

Provide the content one of three ways:
  --from-file <path>   Draft-shaped JSON
  --stdin              Draft-shaped JSON on stdin
  individual flags     --summary/--details/--file/--tag/...

The content is validated, then staged as the lesson draft and shown for review.
Commit it with "gsc lessons draft commit --target <repo|personal>".`,
		Example: `  # From a JSON file
  gsc lessons add --from-file /tmp/lesson.json
  gsc lessons draft commit --target repo

  # From stdin
  cat lesson.json | gsc lessons add --stdin
  gsc lessons draft commit --target personal

  # From individual flags
  gsc lessons add --summary "..." --details "..." --file internal/foo.go --tag foo --importance high
  gsc lessons draft commit --target repo`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" && useStdin {
				return fmt.Errorf("use only one of --from-file or --stdin")
			}
			jsonMode := fromFile != "" || useStdin
			flagsUsed := anyFlagChanged(cmd, addContentFlags)
			if jsonMode && flagsUsed {
				return fmt.Errorf("provide content via --from-file/--stdin OR individual flags, not both")
			}

			var result lessonspkg.ValidationResult
			if jsonMode {
				data, err := readAddInput(fromFile, useStdin)
				if err != nil {
					return err
				}
				result = lessonspkg.ValidateDraftBytes(data, "lesson content must not include id; gsc generates lsn_<uuid-v7>")
			} else {
				if importance == "" {
					importance = "medium"
				}
				result = lessonspkg.ValidateDraftValue(lessonspkg.Draft{
					Summary:       summary,
					Details:       details,
					Topic:         topic,
					RelatedTopics: relatedTopics,
					Importance:    importance,
					AppliesTo: lessonspkg.AppliesTo{
						Files:       files,
						LinkedFiles: linkedFiles,
						Commands:    commands,
					},
					Tags:         tags,
					ReviewChecks: reviewChecks,
					AI: lessonspkg.AIProvenance{
						Provider: provider,
						ModelID:  modelID,
						Agent:    agent,
					},
				})
			}

			if !result.Valid() {
				fmt.Println("Lesson content is invalid; nothing staged:")
				for _, validationErr := range result.Errors {
					fmt.Printf("  ERROR %s\n", validationErr)
				}
				return fmt.Errorf("lesson content is invalid")
			}

			path, err := lessonspkg.WriteDraft(result.Draft, replace)
			if err != nil {
				return err
			}
			fmt.Print(lessonspkg.RenderDraftReview(result, path))
			fmt.Println()
			fmt.Println("If this lesson is correct, run:")
			fmt.Println("  gsc lessons draft commit --target <repo|personal>")
			fmt.Println()
			fmt.Println("If it is incorrect, run:")
			fmt.Println("  gsc lessons draft discard")
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read Draft-shaped JSON content from a file")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read Draft-shaped JSON content from stdin")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace an existing draft instead of refusing")
	cmd.Flags().StringVar(&summary, "summary", "", "Lesson summary")
	cmd.Flags().StringVar(&details, "details", "", "Lesson details")
	cmd.Flags().StringVar(&importance, "importance", "", "Importance: low, medium, or high (default medium)")
	cmd.Flags().StringArrayVar(&files, "file", nil, "Repo-relative file this lesson applies to (repeatable)")
	cmd.Flags().StringArrayVar(&linkedFiles, "linked-file", nil, "Repo-relative related file (repeatable)")
	cmd.Flags().StringArrayVar(&commands, "command", nil, "Command this lesson applies to (repeatable)")
	cmd.Flags().StringVar(&topic, "topic", "", "Primary topic slug (required)")
	cmd.Flags().StringArrayVar(&relatedTopics, "related-topic", nil, "Related topic slug (max 2, repeatable)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag slug (repeatable)")
	cmd.Flags().StringArrayVar(&reviewChecks, "review-check", nil, "Review check (repeatable)")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider provenance")
	cmd.Flags().StringVar(&modelID, "model-id", "", "AI model id provenance")
	cmd.Flags().StringVar(&agent, "agent", "", "AI agent provenance")
	return cmd
}

func anyFlagChanged(cmd *cobra.Command, names []string) bool {
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func readAddInput(file string, useStdin bool) ([]byte, error) {
	if useStdin {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(file)
}
