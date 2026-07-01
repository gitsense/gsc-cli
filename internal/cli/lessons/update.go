/**
 * Component: Lessons Update Command
 * Block-UUID: c2e7a0d9-4b18-4f36-9a72-1d5e8b3c6f02
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Implements the gsc lessons update lifecycle: stage a full replacement by --id, then validate/review/commit/discard. Errors on unknown subcommands.
 * Language: Go
 * Created-at: 2026-06-17
 * Authors: claude-opus-4-8 (v1.0.0), claude-opus-4-8 (v1.1.0)
 */

package lessons

import (
	"fmt"
	"io"
	"os"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func updateCmd() *cobra.Command {
	var (
		id          string
		file        string
		useStdin    bool
		targetValue string
	)
	cmd := &cobra.Command{
		Use:   "update --id <id> (--file <path> | --stdin)",
		Short: "Stage a full replacement of a committed lesson",
		Long: `Stage a full replacement of a committed lesson, then review and commit it.

The target is selected deterministically by --id (a full lesson ID or a unique
short-ID prefix). The new content is Draft-shaped JSON (summary, details,
applies_to, tags, importance, review_checks) supplied via --file or --stdin —
not a raw record line; keywords and identity are managed by gsc.

Validation always runs before anything is staged or committed. If the content
is invalid, nothing is staged and the original lesson is left untouched.

  gsc lessons update --target repo --id <id> --file new.json   # validate + stage + show diff
  gsc lessons update review                        # re-show the old -> new diff
  gsc lessons update commit                        # replace the lesson in place
  gsc lessons update discard                        # drop the staged update`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			target, err := gitsensescope.ParseTarget(targetValue)
			if err != nil {
				return err
			}
			data, err := readUpdateInput(file, useStdin)
			if err != nil {
				return err
			}
			record, err := lessonspkg.ResolveRecordFromTarget(id, target)
			if err != nil {
				return err
			}
			if record == nil {
				return fmt.Errorf("lesson not found in %s store: %s", target, id)
			}

			result := lessonspkg.ValidateDraftBytes(data, "update content must not include id; pass the target via --id")
			if !result.Valid() {
				fmt.Println("Update content is invalid; nothing staged:")
				for _, validationErr := range result.Errors {
					fmt.Printf("  ERROR %s\n", validationErr)
				}
				return fmt.Errorf("update content is invalid")
			}

			path, err := lessonspkg.WriteUpdateStage(lessonspkg.UpdateStage{
				TargetID: record.ID,
				Target:   string(target),
				Draft:    result.Draft,
			})
			if err != nil {
				return err
			}
			fmt.Print(lessonspkg.RenderUpdateReview(*record, result, path))
			fmt.Println()
			fmt.Println("Next actions:")
			fmt.Println("  gsc lessons update review       # re-show this comparison")
			fmt.Println("  gsc lessons update commit       # replace the lesson in place")
			fmt.Println("  gsc lessons update discard      # drop this staged update")
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "ID (or unique short-ID prefix) of the lesson to replace")
	cmd.Flags().StringVar(&file, "file", "", "Path to Draft-shaped JSON with the new content")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read Draft-shaped JSON content from stdin")
	cmd.Flags().StringVar(&targetValue, "target", "", "Write target: repo or personal (required)")

	cmd.AddCommand(updateValidateCmd())
	cmd.AddCommand(updateReviewCmd())
	cmd.AddCommand(updateCommitCmd())
	cmd.AddCommand(updateDiscardCmd())
	return cmd
}

func updateValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "validate",
		Short:        "Validate the staged lesson update without rendering a review",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stage, path, err := lessonspkg.ReadUpdateStage()
			if err != nil {
				return err
			}
			if stage == nil {
				return fmt.Errorf("no staged lesson update; run 'gsc lessons update --id <id> --file <path>'")
			}
			result := lessonspkg.ValidateUpdateDraft(stage.Draft)
			if result.Valid() {
				fmt.Printf("OK staged update is valid: %s\n", path)
				return nil
			}
			fmt.Printf("Staged update is invalid: %s\n", path)
			for _, validationErr := range result.Errors {
				fmt.Printf("  ERROR %s\n", validationErr)
			}
			return fmt.Errorf("staged update is invalid")
		},
	}
}

func updateReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "review",
		Short:        "Show the staged update as an old -> new comparison",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stage, path, err := lessonspkg.ReadUpdateStage()
			if err != nil {
				return err
			}
			if stage == nil {
				return fmt.Errorf("no staged lesson update; run 'gsc lessons update --id <id> --file <path>'")
			}
			target, err := targetFromUpdateStage(stage)
			if err != nil {
				return err
			}
			original, err := lessonspkg.ResolveRecordFromTarget(stage.TargetID, target)
			if err != nil {
				return err
			}
			if original == nil {
				return fmt.Errorf("target lesson no longer exists: %s", stage.TargetID)
			}
			result := lessonspkg.ValidateUpdateDraft(stage.Draft)
			fmt.Print(lessonspkg.RenderUpdateReview(*original, result, path))
			if !result.Valid() {
				return fmt.Errorf("staged update is invalid")
			}
			fmt.Println()
			fmt.Println("If this replacement is correct, run:")
			fmt.Println("  gsc lessons update commit")
			fmt.Println()
			fmt.Println("If this replacement is incorrect, run:")
			fmt.Println("  gsc lessons update discard")
			return nil
		},
	}
}

func updateCommitCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "commit",
		Short:        "Replace the target lesson with the staged update and rebuild the Brain",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stage, _, err := lessonspkg.ReadUpdateStage()
			if err != nil {
				return err
			}
			target, err := targetFromUpdateStage(stage)
			if err != nil {
				return err
			}
			record, err := lessonspkg.CommitUpdateForTarget(target)
			if err != nil {
				return err
			}
			recordsPath, _ := lessonspkg.RecordsPathForTarget(target)
			fmt.Printf("Updated lesson in %s scope: %s\n", target, record.ID)
			if target == gitsensescope.TargetRepo {
				fmt.Println("Rebuilt and imported Brain: gsc-lessons")
			} else {
				fmt.Println("Rebuilt manifest: gsc-lessons")
			}
			fmt.Println()
			fmt.Println("To preserve the change for teammates and future clones, commit:")
			fmt.Printf("  %s\n", recordsPath)
			return nil
		},
	}
}

func targetFromUpdateStage(stage *lessonspkg.UpdateStage) (gitsensescope.Target, error) {
	if stage == nil {
		return "", fmt.Errorf("no staged lesson update; run 'gsc lessons update --id <id> --file <path>'")
	}
	if stage.Target == "" {
		return "", fmt.Errorf("staged lesson update has no target; restage with --target repo or --target personal")
	}
	return gitsensescope.ParseTarget(stage.Target)
}

func updateDiscardCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "discard",
		Short:        "Discard the staged lesson update",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, discarded, err := lessonspkg.DiscardUpdateStage()
			if err != nil {
				return err
			}
			if !discarded {
				fmt.Println("No staged lesson update found to discard.")
				return nil
			}
			fmt.Printf("Discarded staged update:\n  %s\n", path)
			return nil
		},
	}
}

func readUpdateInput(file string, useStdin bool) ([]byte, error) {
	switch {
	case useStdin && file != "":
		return nil, fmt.Errorf("use either --file or --stdin, not both")
	case useStdin:
		return io.ReadAll(os.Stdin)
	case file != "":
		return os.ReadFile(file)
	default:
		return nil, fmt.Errorf("provide the new content with --file <path> or --stdin")
	}
}
