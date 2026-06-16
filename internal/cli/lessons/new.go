/**
 * Component: Lessons New Command
 * Block-UUID: ff1394b3-af3e-4341-a0d8-eee2ced3dfe9
 * Parent-UUID: c61954d2-897b-4d57-b0a5-11ec9fea7892
 * Version: 1.3.0
 * Description: Added discard option to existing-draft help text.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0), Codex GPT-5 (v1.1.0), claude-sonnet-4-6 (v1.2.0), MiMo-v2.5-pro (v1.3.0)
 */


package lessons

import (
	"fmt"
	"os"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	var replace bool
	cmd := &cobra.Command{
		Use:          "new",
		Short:        "Start a new lesson draft",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := lessonspkg.DraftPath()
			if err == nil {
				if _, statErr := os.Stat(path); statErr == nil && !replace {
					fmt.Printf("Found existing lesson draft:\n  %s\n\n", path)
					fmt.Println("This may be an uncommitted lesson from a previous session.")
					fmt.Println()
					fmt.Println("Next actions:")
					fmt.Println("  gsc lessons validate            # check the draft")
					fmt.Println("  gsc lessons review              # inspect the draft for human confirmation")
					fmt.Println("  gsc lessons commit              # persist it as a lesson")
					fmt.Println("  gsc lessons discard             # delete the draft")
					fmt.Println("  gsc lessons new --replace       # delete it and start a new draft")
					return nil
				}
			}

			path, err = lessonspkg.CreateDraft(replace)
			if err != nil {
				return err
			}
			fmt.Print(lessonspkg.DraftGuide())
			fmt.Println()
			fmt.Print(lessonspkg.SchemaGuide())
			fmt.Println()
			fmt.Printf("Draft created:\n  %s\n\n", path)
			fmt.Println("Agent workflow:")
			fmt.Println("  1. Fill in the draft with a concrete lesson.")
			fmt.Println("  2. Run: gsc lessons validate")
			fmt.Println("  3. If valid, tell the user to run: gsc lessons review")
			return nil
		},
	}
	cmd.Flags().BoolVar(&replace, "replace", false, "Delete any existing draft and start fresh")
	return cmd
}
