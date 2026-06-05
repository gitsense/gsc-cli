/**
 * Component: Exec Data Models
 * Block-UUID: 9f8e7d6c-5b4a-4a2d-8c1e-3f5a6b7c8d9e
 * Parent-UUID: e60a2595-3005-4007-88e3-dc8ac8267b37
 * Version: 1.2.0
 * Description: Defines the data structures for persisting execution metadata and flags, enabling the recovery and resend features of the exec command.
 * Language: Go
 * Created-at: 2026-03-02T17:12:10.805Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
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
	Silent         bool `json:"silent"`
}

// ExecOutput represents a saved execution output for listing purposes.
// This is a simplified view used by the CLI list command.
type ExecOutput struct {
	ID        string
	Command   string
	ExitCode  int
	Timestamp string
}
