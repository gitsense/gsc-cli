/**
 * Component: Notes Overview Command
 * Block-UUID: a9b0c1d2-e3f4-5678-abcd-890123456789
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc notes overview to summarize all notes in a human-readable digest.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package notes

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	"github.com/spf13/cobra"
)

func overviewCmd() *cobra.Command {
	var scopeValue string
	cmd := &cobra.Command{
		Use:          "overview",
		Short:        "Summarize all notes in a human-readable digest",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}
			sourcedRecords, err := notespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return err
			}
			renderScopedNotesOverview(scope, sourcedRecords)
			return nil
		},
	}
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}

func renderScopedNotesOverview(scope gitsensescope.Scope, records []notespkg.SourcedNote) {
	fmt.Printf("Scope: %s\n\n", notesScopeLabel(scope))
	if len(records) == 0 {
		fmt.Println(notesScopeEmptyMessage(scope))
		return
	}
	repoRecords, personalRecords := splitSourcedNotesBySource(records)
	if len(repoRecords) > 0 {
		fmt.Println("Repo notes:")
		fmt.Print(notespkg.RenderOverview(notespkg.UnwrapSourcedNotes(repoRecords)))
	}
	if len(personalRecords) > 0 {
		if len(repoRecords) > 0 {
			fmt.Println()
		}
		fmt.Println("Personal notes:")
		fmt.Print(notespkg.RenderOverview(notespkg.UnwrapSourcedNotes(personalRecords)))
	}
}

func notesScopeLabel(scope gitsensescope.Scope) string {
	if scope == gitsensescope.ScopeAll {
		return "all (repo + personal)"
	}
	return string(scope)
}

func notesScopeEmptyMessage(scope gitsensescope.Scope) string {
	if scope == gitsensescope.ScopeAll {
		return "No notes found in repo or personal scope."
	}
	return fmt.Sprintf("No notes found in %s scope.", scope)
}
