/**
 * Component: Scout CLI Flags and Options
 * Block-UUID: 76d0cc95-7def-4f8f-8327-14656c55cd7d
 * Parent-UUID: 79c1162b-0371-44ca-985d-e09eb7beabc1
 * Version: 1.4.0
 * Description: Shared flag definitions for Scout CLI commands (start, status, stop) with turn and force support
 * Language: Go
 * Created-at: 2026-03-31T01:53:14.040Z
 * Authors: GLM-4.7 (v1.3.0), claude-haiku-4-5-20251001 (v1.4.0)
 */


package scoutcli

import (
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// StartFlags contains flags for the scout start command
type StartFlags struct {
	Intent             string
	IntentFile         string
	AutoReview         bool
	WorkingDirectories []string
	ReferenceFiles     []string
	SessionID          string // Optional session ID
	Turn               int    // Required: 1 or 2
	Force              bool   // Force overwrite existing session
	Format             string // Output format: text or json
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

// ResultsFlags contains flags for the scout results command
type ResultsFlags struct {
	SessionID string
	Turn      int
	Format    string // json, text
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

	cmd.Flags().StringVarP(
		&flags.IntentFile,
		"intent-file", "f",
		"",
		"Read intent from a file (alternative to --intent)",
	)

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

	cmd.Flags().StringVar(
		&flags.SessionID,
		"session-id",
		"",
		"Optional session ID (auto-generated if not provided)",
	)

	cmd.Flags().IntVar(
		&flags.Turn,
		"turn",
		0,
		"Turn to execute (1=discovery, 2=verification)",
	)
	cmd.MarkFlagRequired("turn")

	cmd.Flags().BoolVar(
		&flags.Force,
		"force",
		false,
		"Overwrite existing session if it exists",
	)

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"text",
		"Output format: text or json",
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

// RegisterResultsFlags registers flags for the results command
func RegisterResultsFlags(cmd *cobra.Command, flags *ResultsFlags) {
	cmd.Flags().StringVarP(
		&flags.SessionID,
		"session", "s",
		"",
		"Scout session ID to retrieve results for",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().IntVar(
		&flags.Turn,
		"turn",
		0,
		"Turn number to retrieve results for (1=discovery, 2=verification)",
	)
	cmd.MarkFlagRequired("turn")

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"json",
		"Output format: json or text",
	)
}

// ValidateStartFlags validates the start command flags
func ValidateStartFlags(flags *StartFlags) error {
	// Ensure either --intent or --intent-file is provided (but not both)
	if flags.Intent == "" && flags.IntentFile == "" {
		return &FlagError{Flag: "intent", Message: "either --intent or --intent-file is required"}
	}
	if flags.Intent != "" && flags.IntentFile != "" {
		return &FlagError{Flag: "intent", Message: "cannot specify both --intent and --intent-file"}
	}

	// Validate intent file exists if provided
	if flags.IntentFile != "" {
		if _, err := os.Stat(flags.IntentFile); err != nil {
			return &FlagError{Flag: "intent-file", Message: fmt.Sprintf("intent file not found: %s", flags.IntentFile)}
		}
	}

	if flags.Turn != 1 && flags.Turn != 2 {
		return &FlagError{Flag: "turn", Message: "turn must be 1 or 2"}
	}

	if len(flags.WorkingDirectories) == 0 {
		return &FlagError{Flag: "workdir", Message: "at least one working directory is required"}
	}

	// Validate working directories exist
	for _, wd := range flags.WorkingDirectories {
		if _, err := os.Stat(wd); err != nil {
			return &FlagError{Flag: "workdir", Message: fmt.Sprintf("working directory not found: %s", wd)}
		}
	}

	// Validate reference files exist (if provided)
	for _, rf := range flags.ReferenceFiles {
		if _, err := os.Stat(rf); err != nil {
			return &FlagError{Flag: "reference", Message: fmt.Sprintf("reference file not found: %s", rf)}
		}
	}

	// Validate session ID format if provided
	if flags.SessionID != "" {
		if err := ValidateSessionID(flags.SessionID); err != nil {
			return &FlagError{Flag: "session-id", Message: err.Error()}
		}
	}

	// Validate format
	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	if !validFormats[flags.Format] {
		return &FlagError{Flag: "format", Message: "format must be 'text' or 'json'"}
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

// ValidateResultsFlags validates the results command flags
func ValidateResultsFlags(flags *ResultsFlags) error {
	if flags.SessionID == "" {
		return &FlagError{Flag: "session", Message: "session ID is required"}
	}

	if flags.Turn != 1 && flags.Turn != 2 {
		return &FlagError{Flag: "turn", Message: "turn must be 1 or 2"}
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[flags.Format] {
		return &FlagError{Flag: "format", Message: "format must be json or text"}
	}

	// Validate session ID format
	return ValidateSessionID(flags.SessionID)
}

// ValidateScoutFlags checks that unsupported flags are not set
func ValidateScoutFlags(cmd *cobra.Command) error {
	// Check --code flag (from root gsc command)
	if code, _ := cmd.Flags().GetString("code"); code != "" {
		return &FlagError{Flag: "code", Message: "--code flag is not supported for scout commands"}
	}

	// Check --uuid flag (from gsc claude command)
	if uuid, _ := cmd.Flags().GetString("uuid"); uuid != "" {
		return &FlagError{Flag: "uuid", Message: "--uuid flag is not supported for scout commands"}
	}

	// Check --parent-id flag (from gsc claude command)
	if parentID, _ := cmd.Flags().GetInt64("parent-id"); parentID != 0 {
		return &FlagError{Flag: "parent-id", Message: "--parent-id flag is not supported for scout commands"}
	}

	return nil
}

// ValidateSessionID validates the session ID format
func ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Check for invalid characters (only allow alphanumeric, hyphens, underscores)
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, sessionID)
	if !matched {
		return fmt.Errorf("session ID can only contain letters, numbers, hyphens, and underscores")
	}

	// Check length (reasonable limit)
	if len(sessionID) > 64 {
		return fmt.Errorf("session ID too long (max 64 characters)")
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
	return refs, nil // Reference files are optional; validation happens in ValidateStartFlags
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
