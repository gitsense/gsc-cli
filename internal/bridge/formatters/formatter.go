/**
 * Component: Formatter Framework
 * Block-UUID: 6b0d5338-bb10-4748-8d70-333b5db61eeb
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the Formatter interface and the resolution logic for selecting between external scripts, internal implementations, and raw execution.
 * Language: Go
 * Created-at: 2026-02-28T16:46:33.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package formatters

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// FormatterContext provides context for formatting a command.
type FormatterContext struct {
	Command string // The binary name (e.g., "cat")
	Args    []string
	WorkDir string
}

// Formatter defines the interface for command output formatters.
// It allows modification of command arguments before execution and transformation of output after execution.
type Formatter interface {
	// PreProcess modifies the command arguments before execution.
	// Returns the modified arguments.
	PreProcess(args []string) []string

	// PostProcess transforms the raw output after execution.
	// Returns the formatted output.
	PostProcess(rawOutput string) (string, error)
}

// ResolveFormatter determines which formatter to use for a given command.
// Priority: External Script -> Internal Registry -> None (Raw)
func ResolveFormatter(command string) Formatter {
	// 1. Check External Script
	// We look for an executable in $GSC_HOME/formatters/<command>
	gscHome, err := settings.GetGSCHome(false)
	if err == nil {
		formatterPath := filepath.Join(gscHome, "formatters", command)
		// Check if the file exists and is executable
		if info, err := exec.LookPath(formatterPath); err == nil {
			logger.Debug("Found external formatter", "command", command, "path", info)
			return &ExternalFormatter{Path: info}
		}
	}

	// 2. Check Internal Registry
	if f, ok := internalRegistry[command]; ok {
		logger.Debug("Found internal formatter", "command", command)
		return f
	}

	// 3. No formatter found
	return nil
}

// internalRegistry maps command names to internal formatters.
var internalRegistry = map[string]Formatter{}

// RegisterFormatter adds a formatter to the internal registry.
// This is called by the init() functions of specific formatters (e.g., cat.go).
func RegisterFormatter(command string, f Formatter) {
	internalRegistry[command] = f
}

// ExternalFormatter wraps an external script/executable found in $GSC_HOME/formatters.
type ExternalFormatter struct {
	Path string
}

// PreProcess for external formatters is currently a no-op.
// External scripts are assumed to handle output transformation only in this phase.
func (ef *ExternalFormatter) PreProcess(args []string) []string {
	return args
}

// PostProcess executes the external script, piping the raw output to stdin and capturing stdout.
func (ef *ExternalFormatter) PostProcess(rawOutput string) (string, error) {
	cmd := exec.Command(ef.Path)
	
	// Pipe raw output to the script's stdin
	cmd.Stdin = strings.NewReader(rawOutput)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Run()
	if err != nil {
		// If the script fails, we might want to return the raw output or the error.
		// For now, let's log the error and return the raw output to be safe.
		logger.Warning("External formatter failed, returning raw output", "path", ef.Path, "error", err, "stderr", stderr.String())
		return rawOutput, nil
	}
	
	return stdout.String(), nil
}
