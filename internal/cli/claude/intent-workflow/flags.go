/**
 * Component: Intent Workflow CLI Flags
 * Block-UUID: ecf12857-f602-4fe7-b34a-ba3418874cdf
 * Parent-UUID: 81da2d73-5c1d-4204-9816-780c76a18d36
 * Version: 1.1.0
 * Description: Flag definitions and validation functions for Agent CLI commands.
 * Language: Go
 * Created-at: 2026-04-21T22:47:08.744Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 2.5 Flash Lite (v1.1.0)
 */


package intentworkflowcli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// StatusFlags holds flags for the status command
type StatusFlags struct {
	Session string
	Follow  bool
	Format  string
	Verbose bool
}

// RegisterStatusFlags registers flags for the status command
func RegisterStatusFlags(cmd *cobra.Command, flags *StatusFlags) {
	cmd.Flags().StringVarP(&flags.Session, "session", "s", "", "Session ID (default: latest)")
	cmd.Flags().BoolVarP(&flags.Follow, "follow", "f", false, "Follow session events in real-time")
	cmd.Flags().StringVarP(&flags.Format, "format", "", "pretty", "Output format (json, table, pretty)")
	cmd.Flags().BoolVarP(&flags.Verbose, "verbose", "v", false, "Show detailed output")
}

// ValidateStatusFlags validates status command flags
func ValidateStatusFlags(flags *StatusFlags) error {
	// If session not provided, find the latest session
	if flags.Session == "" {
		latest, err := findLatestSession()
		if err != nil {
			return fmt.Errorf("no session specified and failed to find latest session: %w", err)
		}
		flags.Session = latest
	}

	// Validate format
	if flags.Format != "json" && flags.Format != "table" && flags.Format != "pretty" {
		return fmt.Errorf("invalid format: %s (must be json, table, or pretty)", flags.Format)
	}

	return nil
}

// findLatestSession finds the most recent intent workflow session directory
func findLatestSession() (string, error) {
	// Get GSC_HOME
	gscHome := os.Getenv("GSC_HOME")
	if gscHome == "" {
		return "", fmt.Errorf("GSC_HOME environment variable not set")
	}

	// Path to intent workflow sessions directory
	sessionsDir := filepath.Join(gscHome, "data", "agent-sessions")

	// Read directory entries
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no intent workflow sessions found")
		}
		return "", fmt.Errorf("failed to read sessions directory: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no intent workflow sessions found")
	}

	// Find the most recent session by modification time
	var latestSession string
	var latestModTime int64

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Unix() > latestModTime {
			latestModTime = info.ModTime().Unix()
			latestSession = entry.Name()
		}
	}

	if latestSession == "" {
		return "", fmt.Errorf("no intent workflow sessions found")
	}

	return latestSession, nil
}

// StartFlags holds flags for the agent start command that control correction
// behaviour.
type StartFlags struct {
	CorrectTries int
	CorrectModel string
}

// RegisterStartFlags registers the correction-phase flags on cmd and binds
// them to flags.
func RegisterStartFlags(cmd *cobra.Command, flags *StartFlags) {
	cmd.Flags().IntVar(
		&flags.CorrectTries,
		"correct-tries",
		3,
		"Number of automatic correction attempts (0 to disable)",
	)
	cmd.Flags().StringVar(
		&flags.CorrectModel,
		"correct-model",
		"haiku",
		"Model family used for correction turns: haiku, sonnet, or opus",
	)
}

// ValidateStartFlags validates the correction-phase flags.
func ValidateStartFlags(flags *StartFlags) error {
	if flags.CorrectTries > 0 {
		switch flags.CorrectModel {
		case "haiku", "sonnet", "opus":
			// valid
		default:
			return fmt.Errorf(
				"--correct-model: invalid model family %q (must be haiku, sonnet, or opus)",
				flags.CorrectModel,
			)
		}
	}
	return nil
}
