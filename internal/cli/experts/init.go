/**
 * Component: Experts Init Command
 * Block-UUID: 0c705c93-a38e-4bd8-be52-5c8dd3f36a15
 * Parent-UUID: 61309a9f-c448-4641-87ff-6644489df6fc
 * Version: 2.0.0
 * Description: Non-mutating by default: prints expert context to stdout. Use --out to write to a file. Works outside a git repo for personal-only context.
 * Language: Go
 * Created-at: 2026-05-02T00:38:48.946Z
 * Authors: GLM-4.7 (v1.0.0), MiMo-v2.5-pro (v2.0.0)
 */

package experts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/internal/git"
	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

// NewInitCmd creates and returns the 'gsc experts init' command.
func NewInitCmd() *cobra.Command {
	var flags InitFlags

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Print expert context for the current environment",
		Long: `Generates expert context containing Brain schemas, query rules, and AI
behavioral guidelines. Prints to stdout by default.

Use --out <path> to write to a file instead (e.g., --out .gitsense/experts-context.md).
Use --silent to suppress all output (for inline agents).

Outside a git repo, generates personal-scope-only context.
Inside a repo, generates full repo + personal context.`,
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
	// 1. Resolve git root (may be empty outside a repo)
	repoRoot, repoErr := git.FindProjectRoot()
	inRepo := repoErr == nil

	// 2. Load brains (only works in a repo)
	var brains []experts.BrainSummary
	if inRepo {
		cfg := experts.ExpertsConfig{
			Databases: flags.DBs,
			RepoPath:  repoRoot,
			UserLevel: flags.UserLevel,
		}

		var err error
		brains, err = experts.LoadBrains(ctx, cfg)
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
	}

	// 3. Check if rules exist (repo only)
	hasRules := false
	if inRepo {
		var err error
		hasRules, err = rulesExist()
		if err != nil {
			fmt.Printf("Warning: could not check for rules: %v\n", err)
		}
	}

	// 4. Build context
	expertsCtx := experts.ExpertsContext{
		GeneratedAt: time.Now(),
		RepoPath:    repoRoot,
		UserLevel:   flags.UserLevel,
		Brains:      brains,
		HasRules:    hasRules,
		RulesMode:   flags.Rules,
	}

	// 5. Render context
	output, err := experts.Render(ctx, expertsCtx)
	if err != nil {
		return fmt.Errorf("failed to render context: %w", err)
	}

	// 6. Output
	if flags.Silent {
		return nil
	}

	if flags.Out != "" {
		// Write to file
		if err := os.MkdirAll(filepath.Dir(flags.Out), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		if err := os.WriteFile(flags.Out, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write context file: %w", err)
		}
		msg := experts.OrientationMessage(expertsCtx, flags.Out, true)
		fmt.Println(msg)
	} else {
		// Print to stdout
		fmt.Print(output)
		fmt.Println()
		msg := experts.OrientationMessage(expertsCtx, "", false)
		fmt.Println(msg)
	}

	return nil
}

func rulesExist() (bool, error) {
	records, err := rulespkg.LoadRecords()
	if err != nil {
		return false, err
	}
	return len(records) > 0, nil
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
