/*
 * Component: Change CLI Start Command
 * Block-UUID: c344d01d-5674-4fdf-8131-fb5b3f69083e
 * Parent-UUID: 34f4ca2d-1dd5-49ef-97a3-ece4ff6283bb
 * Version: 1.21.0
 * Description: Implements 'gsc claude change start' command for in-place code editing. Added --enable-code-provenance flag to support ephemeral header injection and provenance recording. Updated background worker orchestration to persist and propagate the provenance enablement state. Added --skip-discovery flag to support skipping discovery turns and proceeding directly to change. Fixed critical bug where --skip-discovery would fail for new sessions due to session-not-found check occurring before skip-discovery logic. Refactored to use shared agent helper utilities, removing duplicated code for intent resolution, working directory parsing, background worker spawning, and session initialization. Updated InitializeSession call to include disableExperts parameter (false by default for change turns).
 * Language: Go
 * Created-at: 2026-04-29T02:47:10.446Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), Gemini 3 Flash (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v1.16.0), Gemini 3 Flash (v1.17.0), GLM-4.7 (v1.18.0), GLM-4.7 (v1.19.0), GLM-4.7 (v1.20.0), GLM-4.7 (v1.21.0)
 */


package change

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/claude/intent-workflow"
	"github.com/gitsense/gsc-cli/internal/cli/claude/shared"
	"github.com/spf13/cobra"
)

// StartCmd creates the "change start" subcommand
func StartCmd() *cobra.Command {
	flags := &StartFlags{}

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a change turn for in-place code editing",
		Long: `Start a change turn to apply code changes based on discovery results.

The change turn will:
1. Validate that discovery is complete
2. Spawn Claude subprocess to edit files in place
3. Generate .change-meta.json files for each modified file
4. Process changelog and validate all changes
5. Enrich metadata with git provenance (SHAs, change type, language)
6. Write result.json with change summary and changelog

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
	// If --watch-worker flag is set, this is the background process
	if flags.WatchWorker {
		return runBackgroundWorker(cmd, flags)
	}

	// Validate that unsupported flags are not set
	if err := ValidateChangeFlags(cmd); err != nil {
		cmd.SilenceUsage = true
		return err
	}

	// Validate flags
	if err := ValidateStartFlags(flags); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("invalid flags: %w", err)
	}

	// Resolve intent (from flag or file)
	intent, err := shared.ResolveIntent(flags.Intent, flags.IntentFile)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}

	// Session ID is required for change turns
	sessionID := flags.Session
	if sessionID == "" {
		cmd.SilenceUsage = true
		return fmt.Errorf("--session is required for change turns")
	}

	// Create session config to check if session exists
	config, _ := intent_workflow.NewSessionConfig(sessionID)

	// Parse working directories
	workdirs, err := shared.ParseWorkdirs(flags.WorkingDirectories)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to parse working directories: %w", err)
	}

	// Load or create session
	var manager *intent_workflow.Manager
	
	if !config.SessionExists() {
		// Session doesn't exist
		if !flags.SkipDiscovery {
			// Normal flow: require discovery first
			cmd.SilenceUsage = true
			return fmt.Errorf(
				"session '%s' not found. Please run discovery turn first:\n"+
					"  gsc claude scout start --session-id %s --turn-type discovery --intent-file <intent-file> --workdir <workdir>",
				sessionID, sessionID,
			)
		}
		
		// --skip-discovery: create and initialize the session
		manager, err = intent_workflow.NewManagerWithDebug(sessionID, flags.Debug)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		
		// Initialize the session (creates directory structure and session.json)
		// Change turns use experts mode by default (disableExperts = false)
		if err := manager.InitializeSession(intent, workdirs, []intent_workflow.ReferenceFileContext{}, false, flags.Model, false); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to initialize session: %w", err)
		}
		
		// Add the skipped discovery turn
		if err := manager.AddSkippedDiscoveryTurn(); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to add skipped discovery turn: %w", err)
		}
	} else {
		// Session exists: load it
		manager, err = intent_workflow.LoadSession(sessionID)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to load session: %w", err)
		}
	}

	// Update session model if specified
	if flags.Model != "" {
		session := manager.GetSession()
		session.Model = flags.Model
		if err := manager.WriteSessionState(); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to update session model: %w", err)
		}
	}

	// Update session working directories if specified
	if len(flags.WorkingDirectories) > 0 {
		session := manager.GetSession()
		session.WorkingDirectories = workdirs
		if err := manager.WriteSessionState(); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to update session working directories: %w", err)
		}
	}

	// Update provenance enablement if specified
	if flags.EnableCodeProvenance {
		session := manager.GetSession()
		session.EnableCodeProvenance = true
		if err := manager.WriteSessionState(); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to update session provenance state: %w", err)
		}
	}

	// Validate session status
	status, err := manager.GetSessionStatus()
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to get session status: %w", err)
	}

	if status.Status != "discovery_complete" {
		if !flags.SkipDiscovery {
			cmd.SilenceUsage = true
			return fmt.Errorf(
				"session status is %s, expected discovery_complete. Please complete discovery first:\n"+
					"  gsc claude scout start --session-id %s --turn-type discovery",
				status.Status, sessionID,
			)
		}
		// --skip-discovery: add the virtual turn before proceeding
		if err := manager.AddSkippedDiscoveryTurn(); err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to add skipped discovery turn: %w", err)
		}
	}

	// Build args for background worker
	args := []string{"claude", "change", "start"}
	args = append(args, "--session", flags.Session)
	args = append(args, "--watch-worker")
	if flags.Intent != "" {
		args = append(args, "--intent", flags.Intent)
	}
	if flags.IntentFile != "" {
		args = append(args, "--intent-file", flags.IntentFile)
	}
	if flags.KeepChangeMetaFile {
		args = append(args, "--keep-change-meta-file")
	}
	if flags.EnableCodeProvenance {
		args = append(args, "--enable-code-provenance")
	}
	if flags.SkipDiscovery {
		args = append(args, "--skip-discovery")
	}
	if flags.Debug {
		args = append(args, "--debug")
	}

	// Spawn background worker with --watch-worker flag
	workerPID, err := shared.SpawnBackgroundWorker(args, flags.Session)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to spawn background worker: %w", err)
	}

	// Output based on format
	if flags.Format == "json" {
		response := shared.StartResponse{
			SessionID:  sessionID,
			Turn:       manager.GetNextTurnNumber(),
			Status:     "in_progress",
			ProcessPID: workerPID,
			Message:    "Change turn started successfully",
		}

		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to marshal JSON response: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
	} else {
		// Existing text output
		fmt.Fprintf(cmd.OutOrStdout(), "Change turn started\n")
		fmt.Fprintf(cmd.OutOrStdout(), "Session ID: %s\n", shared.FormatSessionLabel("scout", sessionID))
		fmt.Fprintf(cmd.OutOrStdout(), "Intent: %s\n", truncateForDisplay(intent, 100))

		fmt.Fprintf(cmd.OutOrStdout(), "\nMonitor progress with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s\n", sessionID)
		fmt.Fprintf(cmd.OutOrStdout(), "\nFollow in real-time with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude scout status -s %s -f\n", sessionID)

		fmt.Fprintf(cmd.OutOrStdout(), "\nStop the session with:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  gsc claude change stop -s %s\n", sessionID)
	}

	return nil
}

// runBackgroundWorker executes the change turn in the background worker process
func runBackgroundWorker(cmd *cobra.Command, flags *StartFlags) error {
	// Use shared helper to initialize worker session
	manager, debugLogger, err := shared.InitWorkerSession(flags.Session, flags.Debug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return err
	}
	defer debugLogger.Close()

	debugLogger.Log("WORKER", "Background worker started")
	debugLogger.Log("WORKER", fmt.Sprintf("Session ID: %s", flags.Session))
	debugLogger.Log("WORKER", "Change turn")

	debugLogger.Log("WORKER", "Session loaded successfully")

	// Validate session status
	debugLogger.Log("WORKER", "Getting session status...")
	status, err := manager.GetSessionStatus()
	if err != nil {
		debugLogger.LogError("Failed to get session status", err)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get session status: %v\n", err)
		return fmt.Errorf("failed to get session status: %w", err)
	}
	debugLogger.Log("WORKER", fmt.Sprintf("Session status: %s", status.Status))

	// Validate session state for change
	if status.Status != "discovery_complete" {
		if !flags.SkipDiscovery {
			err := fmt.Errorf("session status is %s, expected discovery_complete for change turn", status.Status)
			debugLogger.LogError("Invalid session status", err)
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return err
		}
		// --skip-discovery: add the virtual turn before proceeding
		if err := manager.AddSkippedDiscoveryTurn(); err != nil {
			debugLogger.LogError("Failed to add skipped discovery turn", err)
			fmt.Fprintf(os.Stderr, "ERROR: Failed to add skipped discovery turn: %v\n", err)
			return fmt.Errorf("failed to add skipped discovery turn: %w", err)
		}
		// Reload status after adding skipped turn
		status, err = manager.GetSessionStatus()
		if err != nil {
			debugLogger.LogError("Failed to reload session status", err)
			fmt.Fprintf(os.Stderr, "ERROR: Failed to reload session status: %v\n", err)
			return fmt.Errorf("failed to reload session status: %w", err)
		}
	}

	debugLogger.Log("WORKER", fmt.Sprintf("Session has %d turns", len(status.Turns)))

	nextTurn := manager.GetNextTurnNumber()

	// Validate that the last turn is complete
	lastTurn := status.Turns[len(status.Turns)-1]
	debugLogger.Log("WORKER", fmt.Sprintf("Last turn: %d (type: %s, status: %s)",
		lastTurn.TurnNumber, lastTurn.TurnType, lastTurn.Status))

	if lastTurn.Status != "complete" && lastTurn.Status != "skipped" {
		err := fmt.Errorf("last turn %d is not complete (status: %s)", lastTurn.TurnNumber, lastTurn.Status)
		debugLogger.LogError("Last turn not complete", err)
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		return err
	}
	debugLogger.Log("WORKER", fmt.Sprintf("Starting Turn %d (previous turn complete)", nextTurn))

	// Determine intent
	intent, err := shared.ResolveIntent(flags.Intent, flags.IntentFile)
	if err != nil {
		debugLogger.LogError("Failed to read intent file", err)
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read intent file: %v\n", err)
		return fmt.Errorf("failed to read intent file: %w", err)
	}

	// Execute the change turn (this blocks until complete)
	debugLogger.Log("WORKER", "Starting change turn...")
	if err := manager.StartChangeTurn(intent); err != nil {
		debugLogger.LogError("Change turn failed", err)
		fmt.Fprintf(os.Stderr, "ERROR: Change turn failed: %v\n", err)
		return fmt.Errorf("change turn failed: %w", err)
	}

	// Run centralized post-processing
	// This handles logging, validation, enrichment, correction, and result writing
	debugLogger.Log("WORKER", "Starting post-processing...")
	pp := intent_workflow.NewPostProcessor(manager, nextTurn, status.Turns[len(status.Turns)-1].StartedAt, flags.KeepChangeMetaFile, manager.GetSession().EnableCodeProvenance, debugLogger)
	if err := pp.Run(); err != nil {
		debugLogger.LogError("Post-processing failed", err)
		fmt.Fprintf(os.Stderr, "ERROR: Post-processing failed: %v\n", err)
		return fmt.Errorf("post-processing failed: %w", err)
	}

	debugLogger.Log("WORKER", "Change turn completed successfully")
	return nil
}

// truncateForDisplay truncates a string for display purposes
func truncateForDisplay(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// StartFlags contains flags for the change start command
type StartFlags struct {
	Intent             string
	IntentFile         string
	Session            string
	Debug              bool
	Format             string
	WatchWorker        bool
	KeepChangeMetaFile bool
	EnableCodeProvenance bool
	Model              string
	WorkingDirectories []string
	SkipDiscovery      bool
}

// RegisterStartFlags registers flags for the start command
func RegisterStartFlags(cmd *cobra.Command, flags *StartFlags) {
	cmd.Flags().StringVarP(
		&flags.Intent,
		"intent", "i",
		"",
		"The change request/intent for Claude to apply",
	)

	cmd.Flags().StringVarP(
		&flags.IntentFile,
		"intent-file", "f",
		"",
		"Read intent from a file (alternative to --intent)",
	)

	cmd.Flags().StringVar(
		&flags.Session,
		"session",
		"",
		"Session ID (required for change turns)",
	)
	cmd.MarkFlagRequired("session")

	cmd.Flags().StringVar(
		&flags.Format,
		"format",
		"text",
		"Output format: text or json",
	)

	cmd.Flags().BoolVar(
		&flags.WatchWorker,
		"watch-worker",
		false,
		"Run as background worker process",
	)
	cmd.Flags().MarkHidden("watch-worker")

	cmd.Flags().BoolVar(
		&flags.KeepChangeMetaFile,
		"keep-change-meta-file",
		false,
		"Keep .change-meta.json files after successful turn (for debugging)",
	)

	cmd.Flags().BoolVar(
		&flags.EnableCodeProvenance,
		"enable-code-provenance",
		false,
		"Inject code block headers and record provenance in .gitsense/provenance.jsonl",
	)

	cmd.Flags().BoolVar(
		&flags.SkipDiscovery,
		"skip-discovery",
		false,
		"Skip the discovery turn and proceed directly to change",
	)

	cmd.Flags().BoolVar(
		&flags.Debug,
		"debug",
		false,
		"Enable debug logging to session directory",
	)

	cmd.Flags().StringVar(
		&flags.Model,
		"model",
		"",
		"Claude model family: haiku, sonnet, or opus",
	)

	cmd.Flags().StringSliceVarP(
		&flags.WorkingDirectories,
		"workdir", "w",
		[]string{},
		"Working directories to search (can be specified multiple times)",
	)
}

// ValidateStartFlags validates the start command flags
func ValidateStartFlags(flags *StartFlags) error {
	// Ensure either --intent or --intent-file is provided (but not both)
	intent := flags.Intent
	if intent == "" && flags.IntentFile == "" {
		return &shared.FlagError{Flag: "intent", Message: "either --intent or --intent-file is required"}
	}
	if intent != "" && flags.IntentFile != "" {
		return &shared.FlagError{Flag: "intent", Message: "cannot specify both --intent and --intent-file"}
	}

	// Validate intent file exists if provided
	if flags.IntentFile != "" {
		if _, err := os.Stat(flags.IntentFile); err != nil {
			return &shared.FlagError{Flag: "intent-file", Message: fmt.Sprintf("intent file not found: %s", flags.IntentFile)}
		}
	}

	// Validate format
	validFormats := map[string]bool{
		"text": true,
		"json": true,
	}
	if !validFormats[flags.Format] {
		return &shared.FlagError{Flag: "format", Message: "format must be 'text' or 'json'"}
	}

	return nil
}

// ValidateChangeFlags checks that unsupported flags are not set
func ValidateChangeFlags(cmd *cobra.Command) error {
	// Check --code flag (from root gsc command)
	if code, _ := cmd.Flags().GetString("code"); code != "" {
		return &shared.FlagError{Flag: "code", Message: "--code flag is not supported for change commands"}
	}

	// Check --uuid flag (from gsc claude command)
	if uuid, _ := cmd.Flags().GetString("uuid"); uuid != "" {
		return &shared.FlagError{Flag: "uuid", Message: "--uuid flag is not supported for change commands"}
	}

	// Check --parent-id flag (from gsc claude command)
	if parentID, _ := cmd.Flags().GetInt64("parent-id"); parentID != 0 {
		return &shared.FlagError{Flag: "parent-id", Message: "--parent-id flag is not supported for change commands"}
	}

	return nil
}
