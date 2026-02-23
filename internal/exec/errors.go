/**
 * Component: Exec Error Definitions
 * Block-UUID: 702c23c3-0342-43fe-855c-1708a967281c
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines custom error types for the exec command to distinguish between execution failures, signal interruptions, and recovery scenarios.
 * Language: Go
 * Created-at: 2026-02-23T03:10:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package exec

import "fmt"

// ExecError represents an error that occurred during command execution.
// It includes a machine-readable Code and a human-readable Message.
type ExecError struct {
	Code    string
	Message string
	Err     error
}

func (e *ExecError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *ExecError) Unwrap() error {
	return e.Err
}

// Error codes
const (
	ErrCodeCommandNotFound = "COMMAND_NOT_FOUND"
	ErrCodeSignalInterrupt = "SIGNAL_INTERRUPT"
	ErrCodeExecutionFailed = "EXECUTION_FAILED"
	ErrCodeFileIO          = "FILE_IO"
)

// NewExecError creates a new ExecError.
func NewExecError(code, message string, err error) *ExecError {
	return &ExecError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
