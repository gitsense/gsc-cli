/**
 * Component: Scout CLI Start Command
 * Block-UUID: 82fe7af5-fd9a-4eab-a490-d155201b45a1
 * Parent-UUID: f7b69aa2-6b58-4e2b-925b-cca47fe064b5
 * Version: 1.0.9
 * Description: Implements 'gsc claude scout start' command with turn-aware session handling
 * Language: Go
 * Created-at: 2026-03-28T23:12:57.555Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.0.5), GLM-4.7 (v1.0.6), claude-haiku-4-5-20251001 (v1.0.9)
 */


package scoutcli

import (
	"fmt"
	"os"
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
	// Validate that unsupported flags are not set
	if err := ValidateScoutFlags(cmd); err != nil {
		return err
	}

	// Validate flags
	if err := ValidateStartFlags(flags); err != nil {
		return fmt.Errorf("invalid flags: %w", err)
	}

	// EARLY VALIDATION: Reject turns > 2 for now
	if flags.Turn > 2 {
		return fmt.Errorf(
			"Turn %d is not yet supported. Currently only Turn 1 (discovery) and Turn 2 (verification) are implemented.\n"+
				"Turn 1: gsc claude scout start --turn 1 ...\n"+
				"Turn 2: gsc claude scout start --turn 2 ... (after Turn 1 completes)",
			flags.Turn,
		)
	}

	// Determine intent (from flag or file)
	intent := flags.Intent
	if flags.IntentFile != "" {
		content, err := os.ReadFile(flags.IntentFile)
		if err != nil {
			return fmt.Errorf("failed to read intent file: %w", err)
		}
		intent = string(content)
	}

	// Generate a unique session ID and handle turn-aware session logic
	sessionID := flags.SessionID
	if sessionID == "" {
		// Auto-generate if not provided
		sessionID = uuid.New().String()[:12]
	} else {
		// Validate session ID format (already done in ValidateStartFlags)
		// Check if session already exists
		config, _ := claudescout.NewSessionConfig(sessionID)

		// TURN-AWARE SESSION HANDLING
		if flags.Turn == 1 {
			// Turn 1: Error if session exists (unless --force)
			if config.SessionExists() {
				if !flags.Force {
					return fmt.Errorf("session '%s' already exists. Use --force to overwrite", sessionID)
				}
				// Delete existing session only for Turn 1
				if err := config.CleanupSessionDir(); err != nil {
					return fmt.Errorf("failed to cleanup existing session: %w", err)
				}
			}
		} else if flags.Turn == 2 {
			// Turn 2: Session must exist (will load it)
			if !config.SessionExists() {
				return fmt.Errorf(
					"session '%s' not found. Please run Turn 1 discovery first:\n"+
						"  gsc claude scout start --session-id %s --turn 1 --intent-file <intent-file> --workdir <workdir>",
					sessionID, sessionID,
				)
			}
			// For Turn 2, load existing session instead of creating new
			tempManager, err := claudescout.LoadSession(sessionID)
			if err != nil {
				return fmt.Errorf("failed to load existing session: %w", err)
			}

			// Validate Turn 1 is complete
			lastCompleted, err := tempManager.GetLastCompletedTurn()
			if err != nil {
				return fmt.Errorf("failed to check session status: %w", err)
			}
			if lastCompleted < 1 {
				return fmt.Errorf(
					"Turn 1 discovery has not completed yet. Check status with:\n"+
						"  gsc claude scout status -s %s",
					sessionID,
				)
			}
		}
	}

	// Parse working directories and reference files
	workdirs, err := ParseWorkdirs(flags.WorkingDirectories)
	if err != nil {
		return fmt.Errorf("failed to parse working directories: %w", err)
	}

	refFiles, err := ParseRefFiles(flags.ReferenceFiles)
	if err != nil {
		return fmt.Errorf("failed to parse reference files: %w", err)
	}

	// Create or load scout manager based on turn
	var manager *claudescout.Manager
	if flags.Turn == 1 {
		// Turn 1: Create new manager
		var err error
		manager, err = claudescout.NewManager(sessionID)
		if err != nil {
			return fmt.Errorf("failed to create scout manager: %w", err)
		}

		// Initialize the session for Turn 1
		if err := manager.InitializeSession(intent, workdirs, refFiles, flags.AutoReview); err != nil {
			return fmt.Errorf("failed to initialize session: %w", err)
		}
	} else if flags.Turn == 2 {
		// Turn 2: Load existing manager
		var err error
		manager, err = claudescout.LoadSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		// Turn 2 will use existing session data from Turn 1
	}

	// Execute based on turn
	switch flags.Turn {
	case 1:
		if err := manager.StartTurn1Discovery(); err != nil {
			return fmt.Errorf("failed to start discovery: %w", err)
		}
	case 2:
		// For Turn 2, start verification with existing session
		if err := manager.StartTurn2Verification(nil); err != nil {
			return fmt.Errorf("failed to start verification: %w", err)
		}
	default:
		// Should not reach here due to early validation, but keep as safety net
		return fmt.Errorf("invalid turn: %d (must be 1 or 2)", flags.Turn)
	}

	// Close the event writer
	if err := manager.CloseEventWriter(); err != nil {
		return fmt.Errorf("failed to close event writer: %w", err)
	}

	// Output session information
	fmt.Fprintf(cmd.OutOrStdout(), "Scout session started\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", FormatSessionPath(sessionID))
	fmt.Fprintf(cmd.OutOrStdout(), "Turn: %d\n", flags.Turn)

	if flags.Turn == 1 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nWhen discovery completes, proceed to verification with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout start --session-id %s --turn 2\n", sessionID)
	} else if flags.Turn == 2 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor verification progress with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)
	}

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
	return fmt.Sprintf("scout:%s", strings.TrimSpace(sessionID))
}

// GetSessionShortID returns a shortened session ID for display
func GetSessionShortID(sessionID string) string {
	if len(sessionID) <= 8 {
		return sessionID
	}
	return sessionID[:8]
}
