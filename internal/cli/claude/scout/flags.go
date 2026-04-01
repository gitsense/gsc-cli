/**
 * Component: Scout CLI Flags and Options
 * Block-UUID: ebe96a57-6e66-4d1b-879b-175aec543497
 * Parent-UUID: af333178-cbd8-4e05-861f-22849f382e9b
 * Version: 1.8.1
 * Description: Shared flag definitions for Scout CLI commands (start, status, stop) with turn and force support. Removed MarkFlagRequired("intent") to allow --intent-file as alternative.
 * Language: Go
 * Created-at: 2026-03-31T22:21:02.632Z
 * Authors: claude-haiku-4-5-20251001 (v1.8.0), GLM-4.7 (v1.8.1)
 */


package scoutcli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	claudescout "github.com/gitsense/gsc-cli/internal/claude/scout"
)

// StartFlags contains flags for the scout start command
type StartFlags struct {
	Intent             string
	IntentFile         string
	AutoReview         bool
	WorkingDirectories []string
	ReferenceFilesJSON string
	SessionID          string // Optional session ID
	Turn               int    // Required: 1 or 2
	Force              bool   // Force overwrite existing session
	Format             string // Output format: text or json
	Model              string // Claude model family: haiku, sonnet, opus
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

	cmd.Flags().StringVarP(
		&flags.ReferenceFilesJSON,
		"reference-files", "R",
		"",
		"Path to NDJSON file containing reference files from imported chat context",
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

	cmd.Flags().StringVar(
		&flags.Model,
		"model",
		"",
		"Claude model family: haiku, sonnet, or opus",
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
	intent := strings.TrimSpace(flags.Intent)
	if intent == "" && flags.IntentFile == "" {
		return &FlagError{Flag: "intent", Message: "either --intent or --intent-file is required"}
	}
	if intent != "" && flags.IntentFile != "" {
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

	// For Turn 1, workdirs are required; for Turn 2, they're loaded from existing session
	if flags.Turn == 1 && len(flags.WorkingDirectories) == 0 {
		return &FlagError{Flag: "workdir", Message: "at least one working directory is required"}
	}

	// Validate working directories exist
	for _, wd := range flags.WorkingDirectories {
		if _, err := os.Stat(wd); err != nil {
			return &FlagError{Flag: "workdir", Message: fmt.Sprintf("working directory not found: %s", wd)}
		}
	}

	// Validate reference files JSON file exists (if provided)
	if flags.ReferenceFilesJSON != "" {
		if _, err := os.Stat(flags.ReferenceFilesJSON); err != nil {
			return &FlagError{Flag: "reference-files", Message: fmt.Sprintf("reference files JSON not found: %s", flags.ReferenceFilesJSON)}
		}
	}

	// Validate reference files JSON is valid NDJSON format (if provided)
	if flags.ReferenceFilesJSON != "" {
		if err := validateReferenceFilesJSON(flags.ReferenceFilesJSON); err != nil {
			return &FlagError{Flag: "reference-files", Message: fmt.Sprintf("invalid reference files JSON: %v", err)}
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
	sessionIDPattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	matched := sessionIDPattern.MatchString(sessionID)
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

// ParseReferenceFilesJSON extracts and parses NDJSON reference files from flags
func ParseReferenceFilesJSON(flags *pflag.FlagSet) ([]claudescout.ReferenceFileContext, error) {
	filePath, err := flags.GetString("reference-files")
	if err != nil {
		return nil, err
	}

	if filePath == "" {
		return []claudescout.ReferenceFileContext{}, nil // Reference files are optional
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open reference files: %w", err)
	}
	defer file.Close()

	var refFiles []claudescout.ReferenceFileContext
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var ref claudescout.ReferenceFileContext
		if err := json.Unmarshal(scanner.Bytes(), &ref); err != nil {
			return nil, fmt.Errorf("invalid reference file line: %w", err)
		}
		refFiles = append(refFiles, ref)
	}

	return refFiles, scanner.Err()
}

// validateReferenceFilesJSON validates that the reference files JSON is valid NDJSON format
func validateReferenceFilesJSON(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open reference files: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		var ref claudescout.ReferenceFileContext
		if err := json.Unmarshal(scanner.Bytes(), &ref); err != nil {
			return fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	return scanner.Err()
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

// ParseTurn extracts the turn number from flags
func ParseTurn(flags *pflag.FlagSet) (int, error) {
	return flags.GetInt("turn")
}

// ParseForce extracts the force flag for stop command
func ParseForce(flags *pflag.FlagSet) (bool, error) {
	return flags.GetBool("force")
}
