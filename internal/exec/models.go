/**
 * Component: Exec Data Models
 * Block-UUID: e60a2595-3005-4007-88e3-dc8ac8267b37
 * Parent-UUID: be89efd0-a5eb-466c-a1a5-882c80fde459
 * Version: 1.1.0
 * Description: Defines the data structures for persisting execution metadata and flags, enabling the recovery and resend features of the exec command.
 * Language: Go
 * Created-at: 2026-02-28T17:11:31.131Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package exec

// ExecMetadata stores the context of a command execution.
// It is serialized to JSON alongside the output log file.
type ExecMetadata struct {
	Version   string      `json:"version"`
	Command   string      `json:"command"`
	ExitCode  int         `json:"exit_code"`
	Timestamp string      `json:"timestamp"`
	Duration  string      `json:"duration_seconds"`
	Flags     ExecFlags   `json:"flags"`
}

// ExecFlags captures the specific flags used during the execution.
type ExecFlags struct {
	NoStdout bool `json:"no_stdout"`
	NoStderr bool `json:"no_stderr"`
	TimeoutSeconds int  `json:"timeout_seconds"`
}

// ExecOutput represents a saved execution output for listing purposes.
// This is a simplified view used by the CLI list command.
type ExecOutput struct {
	ID        string
	Command   string
	ExitCode  int
	Timestamp string
}
