/**
 * Component: Scout CLI Start Command
 * Block-UUID: 62aaec2a-f9af-4e4b-a7d5-cc03a07d8737
 * Parent-UUID: 62afa9b6-3970-4d6c-ab91-1ad8945821af
 * Version: 1.0.3
 * Description: Implements 'gsc claude scout start' command for initiating new Scout sessions
 * Language: Go
 * Created-at: 2026-03-27T22:58:30.292Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package scoutcli

import (
	"fmt"
	"path/filepath"
	"strings"

	claudescout "github.com/gitsense/gsc-cli/internal/claude/scout"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// StartCmd creates the "scout start" subcommand
func StartCmd() *cobra.Command {
	flags := &StartFlags{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new Scout discovery session",
		Long: `Start a new fire-and-forget Scout session to discover relevant files.

The Scout will:
1. Search working directories using gsc insights and gsc grep
2. Discover candidate files using the Tiny Overview brain
3. Score and rank candidates by relevance
4. Optionally proceed to verification (re-scoring with Claude)

The session runs as a background subprocess and can be monitored with 'gsc claude scout status'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStartCommand(cmd, flags)
		},
	}

	RegisterStartFlags(cmd, flags)

	return cmd
}

// runStartCommand executes the start command logic
func runStartCommand(cmd *cobra.Command, flags *StartFlags) error {
	// Validate flags
	if err := ValidateStartFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Generate a unique session ID
	sessionID := uuid.New().String()[:12]

	// Convert working directory paths to absolute paths
	workdirs := make([]claudescout.WorkingDirectory, len(flags.WorkingDirectories))
	for i, wd := range flags.WorkingDirectories {
		absPath, err := filepath.Abs(wd)
		if err != nil {
			return fmt.Errorf("failed to resolve working directory path %s: %w", wd, err)
		}

		workdirs[i] = claudescout.WorkingDirectory{
			ID:   i + 1,
			Name: filepath.Base(absPath),
			Path: absPath,
		}
	}

	// Convert reference file paths to absolute paths
	refFiles := make([]claudescout.ReferenceFile, len(flags.ReferenceFiles))
	for i, rf := range flags.ReferenceFiles {
		absPath, err := filepath.Abs(rf)
		if err != nil {
			return fmt.Errorf("failed to resolve reference file path %s: %w", rf, err)
		}

		refFiles[i] = claudescout.ReferenceFile{
			OriginalPath: absPath,
			LocalPath:    filepath.Base(absPath),
		}
	}

	// Create a new scout manager
	manager, err := claudescout.NewManager(sessionID)
	if err != nil {
		return fmt.Errorf("failed to create scout manager: %w", err)
	}

	// Initialize the session
	if err := manager.InitializeSession(flags.Intent, workdirs, refFiles, flags.AutoReview); err != nil {
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	// Start Turn 1 discovery
	if err := manager.StartTurn1Discovery(); err != nil {
		return fmt.Errorf("failed to start discovery: %w", err)
	}

	// Close the event writer
	if err := manager.CloseEventWriter(); err != nil {
		return fmt.Errorf("failed to close event writer: %w", err)
	}

	// Output session information
	fmt.Fprintf(cmd.OutOrStdout(), "Scout session started\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", sessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
	fmt.Fprintf(cmd.OutOrStdout(), "\nStop the session with:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout stop -s %s\n", sessionID)

	return nil
}

// ParseWorkdirs converts working directory strings to WorkingDirectory structs
func ParseWorkdirs(paths []string) ([]claudescout.WorkingDirectory, error) {
	workdirs := make([]claudescout.WorkingDirectory, len(paths))

	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve working directory path %s: %w", path, err)
		}

		workdirs[i] = claudescout.WorkingDirectory{
			ID:   i + 1,
			Name: filepath.Base(absPath),
			Path: absPath,
		}
	}

	return workdirs, nil
}

// ParseRefFiles converts reference file strings to ReferenceFile structs
func ParseRefFiles(paths []string) ([]claudescout.ReferenceFile, error) {
	refFiles := make([]claudescout.ReferenceFile, len(paths))

	for i, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve reference file path %s: %w", path, err)
		}

		refFiles[i] = claudescout.ReferenceFile{
			OriginalPath: absPath,
			LocalPath:    filepath.Base(absPath),
		}
	}

	return refFiles, nil
}

// FormatSessionPath returns a user-friendly session path for display
func FormatSessionPath(sessionID string) string {
	return strings.TrimSpace(fmt.Sprintf("scout:%s", sessionID))
}

// GetSessionShortID returns a shortened session ID for display
func GetSessionShortID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:8]
}
