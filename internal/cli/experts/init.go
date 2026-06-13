/**
 * Component: Experts Init Command
 * Block-UUID: 0c705c93-a38e-4bd8-be52-5c8dd3f36a15
 * Parent-UUID: 61309a9f-c448-4641-87ff-6644489df6fc
 * Version: 1.0.5
 * Description: Auto-builds the gsc-lessons Brain during init if records.jsonl exists but the Brain is absent. Eliminates the need for users to run gsc lessons build manually after cloning or pulling new lessons.
 * Language: Go
 * Created-at: 2026-05-02T00:38:48.946Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), claude-sonnet-4-6 (v1.0.5)
 */

package experts

import (
	"context"
	"fmt"
	"time"

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/internal/git"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	"github.com/spf13/cobra"
)

// NewInitCmd creates and returns the 'gsc experts init' command.
func NewInitCmd() *cobra.Command {
	var flags InitFlags

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize expert context for the current repository",
		Long: `Generates the 'experts-context.md' file containing the active Brain schemas,
query rules, and AI behavioral guidelines. This enables the AI agent to become
"Brain-Aware" and use the GitSense Intelligence Layer.

Use --silent to suppress all output (for inline agents).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), &flags)
		},
	}

	AddInitFlags(cmd, &flags)
	return cmd
}

// runInit executes the logic for the init command.
func runInit(ctx context.Context, flags *InitFlags) error {
	// 1. Resolve git root
	repoRoot, err := git.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	// 2. Resolve context file path
	contextPath := experts.ContextFilePath(repoRoot)

	// 3. Load brains
	cfg := experts.ExpertsConfig{
		Databases: flags.DBs,
		RepoPath:  repoRoot,
		UserLevel: flags.UserLevel,
	}

	brains, err := experts.LoadBrains(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to load brains: %w", err)
	}

	if !lessonsBrainPresent(brains) {
		if built, buildErr := tryBuildLessonsBrain(); buildErr != nil {
			fmt.Printf("Warning: could not build lessons Brain: %v\n", buildErr)
		} else if built {
			if !flags.Silent {
				fmt.Println("Lessons Brain initialized from committed records.")
			}
			brains, err = experts.LoadBrains(ctx, cfg)
			if err != nil {
				return fmt.Errorf("failed to load brains: %w", err)
			}
		}
	}

	// 4. Generate context
	expertsCtx := experts.ExpertsContext{
		GeneratedAt: time.Now(),
		RepoPath:    repoRoot,
		UserLevel:   flags.UserLevel,
		Brains:      brains,
	}

	if err := experts.Generate(ctx, expertsCtx, contextPath); err != nil {
		return fmt.Errorf("failed to generate context file: %w", err)
	}

	// 5. Print orientation message (skip if silent mode)
	if !flags.Silent {
		msg := experts.OrientationMessage(expertsCtx, contextPath)
		fmt.Println(msg)
	}

	return nil
}

func lessonsBrainPresent(brains []experts.BrainSummary) bool {
	for _, b := range brains {
		if b.Name == lessonspkg.DatabaseName {
			return true
		}
	}
	return false
}

func tryBuildLessonsBrain() (bool, error) {
	records, err := lessonspkg.LoadRecords()
	if err != nil || len(records) == 0 {
		return false, nil
	}
	return true, lessonspkg.RebuildAndImport()
}
