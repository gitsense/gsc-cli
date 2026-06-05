/**
 * Component: Experts Forget Command
 * Block-UUID: 82778be9-ac09-4151-81ee-76cd1e6a56f6
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the 'gsc experts forget' command. Removes the expert context file and prints an override instruction to the AI, telling it to disregard previous Brain-Aware rules and revert to standard coding assistant behavior.
 * Language: Go
 * Created-at: 2026-05-01T16:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package experts

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/spf13/cobra"
)

// NewForgetCmd creates and returns the 'gsc experts forget' command.
func NewForgetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forget",
		Short: "Remove expert context and reset AI behavior",
		Long: `Removes the 'experts-context.md' file and prints an override instruction
to the AI, telling it to disregard previous Brain-Aware rules and revert
to standard coding assistant behavior.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runForget()
		},
	}

	return cmd
}

// runForget executes the logic for the forget command.
func runForget() error {
	// 1. Resolve git root
	repoRoot, err := git.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	// 2. Resolve context file path
	contextPath := experts.ContextFilePath(repoRoot)

	// 3. Check if context file exists
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		fmt.Println("ℹ️  No expert context found. Nothing to forget.")
		return nil
	}

	// 4. Remove the file
	if err := os.Remove(contextPath); err != nil {
		return fmt.Errorf("failed to remove context file: %w", err)
	}

	// 5. Print the forget message
	msg := experts.ForgetMessage()
	fmt.Println(msg)

	return nil
}
