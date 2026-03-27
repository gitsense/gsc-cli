/*
 * Component: Scout CLI Flags and Options
 * Block-UUID: 4d2c9e5f-3a7b-4e1c-9d5a-2c8f7e6d4b3a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Shared flag definitions for Scout CLI commands (start, status, stop)
 * Language: Go
 * Created-at: 2026-03-27T00:00:00.000Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0)
 */


package scout

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// StartFlags contains flags for the scout start command
type StartFlags struct {
	Intent             string
	AutoReview         bool
	WorkingDirectories []string
	ReferenceFiles     []string
}

// StatusFlags contains flags for the scout status command
type StatusFlags struct {
	SessionID string
	Follow    bool
	Format    string // json, table, pretty
}

// StopFlags contains flags for the scout stop command
type StopFlags struct {
	SessionID string
	Force     bool
}

// CommonFlags contains flags common to multiple commands
type CommonFlags struct {
	SessionID string
}

// RegisterStartFlags registers flags for the start command
func RegisterStartFlags(cmd *cobra.Command, flags *StartFlags) {
	cmd.Flags().StringVarP(
		&flags.Intent,
		"intent", "i",
		"",
		"The intent/question for Scout to discover relevant files",
	)
	cmd.MarkFlagRequired("intent")

	cmd.Flags().BoolVar(
		&flags.AutoReview,
		"auto-review",
		false,
		"Automatically proceed to verification without user selection",
	)

	cmd.Flags().StringSliceVarP(
		&flags.WorkingDirectories,
		"workdir", "w",
		[]string{},
		"Working directories to search (can be specified multiple times)",
	)

	cmd.Flags().StringSliceVarP(
		&flags.ReferenceFiles,
		"reference", "r",
		[]string{},
		"Reference files to guide discovery (can be specified multiple times)",
	)
}

// RegisterStatusFlags registers flags for the status command
func RegisterStatusFlags(cmd *cobra.Command, flags *StatusFlags) {
	cmd.Flags().StringVarP(
		&flags.SessionID,
		"session", "s",
		"",
		"Scout session ID to get status for",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().BoolVarP(
		&flags.Follow,
		"follow", "f",
		false,
		"Follow session progress in real-time (stream events)",
	)

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"pretty",
		"Output format: json, table, or pretty",
	)
}

// RegisterStopFlags registers flags for the stop command
func RegisterStopFlags(cmd *cobra.Command, flags *StopFlags) {
	cmd.Flags().StringVarP(
		&flags.SessionID,
		"session", "s",
		"",
		"Scout session ID to stop",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().BoolVar(
		&flags.Force,
		"force",
		false,
		"Force kill the process without cleanup",
	)
}

// RegisterCommonFlags registers flags common to multiple commands
func RegisterCommonFlags(cmd *cobra.Command, flags *CommonFlags) {
	cmd.Flags().StringVarP(
		&flags.SessionID,
		"session", "s",
		"",
		"Scout session ID",
	)
	cmd.MarkFlagRequired("session")
}

// ValidateStartFlags validates the start command flags
func ValidateStartFlags(flags *StartFlags) error {
	if flags.Intent == "" {
		return &FlagError{Flag: "intent", Message: "intent is required"}
	}

	if len(flags.WorkingDirectories) == 0 {
		return &FlagError{Flag: "workdir", Message: "at least one working directory is required"}
	}

	return nil
}

// ValidateStatusFlags validates the status command flags
func ValidateStatusFlags(flags *StatusFlags) error {
	if flags.SessionID == "" {
		return &FlagError{Flag: "session", Message: "session ID is required"}
	}

	validFormats := map[string]bool{
		"json":   true,
		"table":  true,
		"pretty": true,
	}
	if !validFormats[flags.Format] {
		return &FlagError{Flag: "format", Message: "format must be json, table, or pretty"}
	}

	return nil
}

// ValidateStopFlags validates the stop command flags
func ValidateStopFlags(flags *StopFlags) error {
	if flags.SessionID == "" {
		return &FlagError{Flag: "session", Message: "session ID is required"}
	}

	return nil
}

// FlagError represents a flag validation error
type FlagError struct {
	Flag    string
	Message string
}

// Error implements the error interface
func (e *FlagError) Error() string {
	return "flag " + e.Flag + ": " + e.Message
}

// IsFlagError checks if an error is a FlagError
func IsFlagError(err error) bool {
	_, ok := err.(*FlagError)
	return ok
}

// GetFlagErrorMessage returns the error message from a FlagError
func GetFlagErrorMessage(err error) string {
	if ferr, ok := err.(*FlagError); ok {
		return ferr.Error()
	}
	return ""
}

// ParseSessionID extracts and validates a session ID from flags
func ParseSessionID(flags *pflag.FlagSet) (string, error) {
	sessionID, err := flags.GetString("session")
	if err != nil {
		return "", err
	}

	if sessionID == "" {
		return "", &FlagError{Flag: "session", Message: "session ID is required"}
	}

	return sessionID, nil
}

// ParseIntent extracts and validates intent from flags
func ParseIntent(flags *pflag.FlagSet) (string, error) {
	intent, err := flags.GetString("intent")
	if err != nil {
		return "", err
	}

	if intent == "" {
		return "", &FlagError{Flag: "intent", Message: "intent is required"}
	}

	return intent, nil
}

// ParseWorkingDirectories extracts working directories from flags
func ParseWorkingDirectories(flags *pflag.FlagSet) ([]string, error) {
	workdirs, err := flags.GetStringSlice("workdir")
	if err != nil {
		return nil, err
	}

	if len(workdirs) == 0 {
		return nil, &FlagError{Flag: "workdir", Message: "at least one working directory is required"}
	}

	return workdirs, nil
}

// ParseReferenceFiles extracts reference files from flags
func ParseReferenceFiles(flags *pflag.FlagSet) ([]string, error) {
	refs, err := flags.GetStringSlice("reference")
	if err != nil {
		return nil, err
	}

	return refs, nil // Reference files are optional
}

// ParseAutoReview extracts auto-review flag
func ParseAutoReview(flags *pflag.FlagSet) (bool, error) {
	return flags.GetBool("auto-review")
}

// ParseFollow extracts the follow flag for status command
func ParseFollow(flags *pflag.FlagSet) (bool, error) {
	return flags.GetBool("follow")
}

// ParseFormat extracts the format flag for output
func ParseFormat(flags *pflag.FlagSet) (string, error) {
	return flags.GetString("format")
}

// ParseForce extracts the force flag for stop command
func ParseForce(flags *pflag.FlagSet) (bool, error) {
	return flags.GetBool("force")
}
