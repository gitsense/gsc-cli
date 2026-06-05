/**
 * Component: Experts Init Command
 * Block-UUID: 61309a9f-c448-4641-87ff-6644489df6fc
 * Parent-UUID: 7d6f2621-ac33-476e-9cc6-4b0fdbc6501c
 * Version: 1.0.4
 * Description: Implements the 'gsc experts init' command. Added --silent flag support to suppress all output for inline agents. When --silent is used, the experts-context.md file is generated without printing the orientation message.
 * Language: Go
 * Created-at: 2026-05-02T00:38:48.946Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package experts

import (
	"context"
	"fmt"
	"time"

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/internal/git"
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

	if len(brains) == 0 {
		return fmt.Errorf("no active brains found. Please import a manifest first using 'gsc manifest import'")
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
