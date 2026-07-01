/**
 * Component: Notes Get Command
 * Block-UUID: c5d6e7f8-a9b0-1234-cdef-456789012345
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc notes get, the agent-facing command for querying notes by file, glob, or tag.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package notes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	gitpkg "github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	"github.com/spf13/cobra"
)

func getCmd() *cobra.Command {
	var (
		file       string
		glob       string
		tag        string
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Query notes for a file, glob pattern, or tag",
		Long: `Query notes that match a specific file, glob pattern, or tag.

This is the primary command for coding agents to check for notes before modifying files.

Exit codes:
  0 - Lookup succeeded (including "no notes found")
  1 - Lookup failed (bad args, etc.)`,
		Example: `  # Check notes for a file
  gsc notes get --file internal/cli/root.go

  # Check notes for a directory
  gsc notes get --glob "internal/cli/**"

  # Check notes by tag
  gsc notes get --tag todo

  # JSON output for agents
  gsc notes get --file internal/cli/root.go --format json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate: at least one query parameter
			if file == "" && glob == "" && tag == "" {
				return fmt.Errorf("at least one of --file, --glob, or --tag is required")
			}

			normalizedFile := file
			var gitRoot string
			if file != "" && filepath.IsAbs(file) {
				discoveredRoot, err := discoverNotesRepoFromPath(file)
				if err != nil {
					return fmt.Errorf("could not discover repository for path %s: %w", file, err)
				}
				gitRoot = discoveredRoot

				relPath, err := filepath.Rel(gitRoot, file)
				if err != nil {
					return fmt.Errorf("failed to compute relative path: %w", err)
				}
				normalizedFile = filepath.ToSlash(relPath)
			}

			// Parse scope
			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			var records []notespkg.SourcedNote
			var loadErr error
			if gitRoot != "" {
				records, loadErr = notespkg.LoadRecordsFromScopeForRepo(scope, gitRoot)
			} else {
				records, loadErr = notespkg.LoadRecordsFromScope(scope)
			}
			if loadErr != nil {
				return fmt.Errorf("failed to load notes: %w", loadErr)
			}

			var sourcedMatched []notespkg.SourcedMatchedNote
			queryType := ""
			queryValue := ""

			if file != "" {
				queryType = "file"
				queryValue = file
				sourcedMatched = notespkg.GetSourcedNotesForFile(records, normalizedFile)
			} else if glob != "" {
				queryType = "glob"
				queryValue = glob
				sourcedMatched = notespkg.GetSourcedNotesForGlob(records, glob)
			} else if tag != "" {
				queryType = "tag"
				queryValue = tag
				sourcedMatched = notespkg.GetSourcedNotesForTag(records, tag)
			}

			switch format {
			case "json":
				return renderGetJSON(queryType, queryValue, scope, gitRoot, sourcedMatched)
			default:
				return renderGetHuman(queryType, queryValue, scope, sourcedMatched)
			}
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "File path to query")
	cmd.Flags().StringVar(&glob, "glob", "", "Glob pattern to query")
	cmd.Flags().StringVar(&tag, "tag", "", "Tag to query")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")
	return cmd
}

// sourcedJSONNote wraps MatchedNote with source for JSON output.
type sourcedJSONNote struct {
	Source      gitsensescope.Source `json:"source"`
	Note        notespkg.Note        `json:"note"`
	MatchReason string               `json:"match_reason"`
}

func renderGetHuman(queryType, queryValue string, scope gitsensescope.Scope, matched []notespkg.SourcedMatchedNote) error {
	fmt.Printf("Query: %s=%s\n", queryType, queryValue)
	fmt.Printf("Scope: %s", scope)
	if scope == gitsensescope.ScopeAll {
		fmt.Print(" (repo + personal)")
	}
	fmt.Println()
	fmt.Printf("Notes matched: %d\n\n", len(matched))

	if len(matched) == 0 {
		emptyMessage := fmt.Sprintf("No notes found in %s scope.", scope)
		fmt.Println(emptyMessage)
		return nil
	}

	// Group by source for display
	repoMatched, personalMatched := splitBySource(matched)

	if len(repoMatched) > 0 {
		fmt.Println("Repo notes:")
		fmt.Print(notespkg.RenderMatchedNotesTable(unwrapSourcedMatched(repoMatched)))
	}
	if len(personalMatched) > 0 {
		if len(repoMatched) > 0 {
			fmt.Println()
		}
		fmt.Println("Personal notes:")
		fmt.Print(notespkg.RenderMatchedNotesTable(unwrapSourcedMatched(personalMatched)))
	}
	return nil
}

func splitBySource(matched []notespkg.SourcedMatchedNote) (repo, personal []notespkg.SourcedMatchedNote) {
	for _, smn := range matched {
		if smn.Source == gitsensescope.SourceRepo {
			repo = append(repo, smn)
		} else {
			personal = append(personal, smn)
		}
	}
	return
}

func unwrapSourcedMatched(matched []notespkg.SourcedMatchedNote) []notespkg.MatchedNote {
	result := make([]notespkg.MatchedNote, len(matched))
	for i, smn := range matched {
		result[i] = smn.MatchedNote
	}
	return result
}

// discoverNotesRepoFromPath discovers the owning repository from an absolute file path.
func discoverNotesRepoFromPath(absPath string) (string, error) {
	startPath := absPath
	info, err := os.Stat(absPath)
	if err != nil {
		startPath = filepath.Dir(absPath)
	} else if !info.IsDir() {
		startPath = filepath.Dir(absPath)
	}

	root, err := gitpkg.FindGitRootFrom(startPath)
	if err != nil {
		return "", fmt.Errorf("no git repository found for path %s", absPath)
	}
	return root, nil
}

func renderGetJSON(queryType, queryValue string, scope gitsensescope.Scope, gitRoot string, matched []notespkg.SourcedMatchedNote) error {
	high, medium, low := 0, 0, 0
	for _, smn := range matched {
		switch smn.MatchedNote.Note.Importance {
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}

	if gitRoot == "" {
		var err error
		gitRoot, err = gitpkg.FindGitRoot()
		if err != nil {
			gitRoot = ""
		}
	}

	// Determine active sources
	sourceSet := make(map[gitsensescope.Source]bool)
	for _, smn := range matched {
		sourceSet[smn.Source] = true
	}
	var sources []gitsensescope.Source
	if sourceSet[gitsensescope.SourceRepo] {
		sources = append(sources, gitsensescope.SourceRepo)
	}
	if sourceSet[gitsensescope.SourcePersonal] {
		sources = append(sources, gitsensescope.SourcePersonal)
	}

	// Build notes with source
	notes := make([]sourcedJSONNote, len(matched))
	for i, smn := range matched {
		notes[i] = sourcedJSONNote{
			Source:      smn.Source,
			Note:        smn.MatchedNote.Note,
			MatchReason: smn.MatchedNote.MatchReason,
		}
	}

	output := struct {
		Query struct {
			File string `json:"file,omitempty"`
			Glob string `json:"glob,omitempty"`
			Tag  string `json:"tag,omitempty"`
		} `json:"query"`
		Scope   string                 `json:"scope"`
		Sources []gitsensescope.Source `json:"sources"`
		GitRoot string                 `json:"git_root"`
		Notes   []sourcedJSONNote      `json:"notes"`
		Summary struct {
			NotesMatched int `json:"notes_matched"`
			High         int `json:"high"`
			Medium       int `json:"medium"`
			Low          int `json:"low"`
		} `json:"summary"`
	}{}

	switch queryType {
	case "file":
		output.Query.File = queryValue
	case "glob":
		output.Query.Glob = queryValue
	case "tag":
		output.Query.Tag = queryValue
	}

	output.Scope = string(scope)
	output.Sources = sources
	output.GitRoot = gitRoot
	output.Notes = notes
	output.Summary.NotesMatched = len(matched)
	output.Summary.High = high
	output.Summary.Medium = medium
	output.Summary.Low = low

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
