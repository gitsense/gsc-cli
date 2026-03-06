/**
 * Component: Exec Command Executor
 * Block-UUID: d87b45c2-4c23-4a3f-9232-d29bc65f7871
 * Parent-UUID: ce7afdb7-20b2-4c7e-b8d7-822806fee2db
 * Version: 1.6.0
 * Description: Added Env field to Executor struct and updated Run() method to merge custom environment variables with the system environment, enabling shadow workspace context injection.
 * Language: Go
 * Created-at: 2026-03-02T17:04:29.076Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0), GLM-4.7 (v1.6.0)
 */


package exec

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// Executor manages the lifecycle of a command execution.
type Executor struct {
	Command string
	Flags   ExecFlags
	Workdir string // Workdir specifies the working directory for the command
	Env     []string // Env specifies environment variables to set for the command
}

// Result contains the outcome of an execution.
type Result struct {
	ID        string
	ExitCode  int
	Duration  time.Duration
	Output    string // Combined stdout/stderr
	LogPath   string
	MetaPath  string
}

// NewExecutor creates a new Executor instance.
func NewExecutor(command string, flags ExecFlags, workdir string, env []string) *Executor {
	return &Executor{
		Command: command,
		Flags:   flags,
		Workdir: workdir,
		Env:     env,
	}
}

// Run executes the command, streams output, and saves results to disk.
func (e *Executor) Run() (*Result, error) {
	startTime := time.Now()
	
	// 1. Prepare Output Directory
	outputDir, err := settings.GetExecOutputsDir()
	if err != nil {
		return nil, NewExecError(ErrCodeFileIO, "failed to resolve output directory", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, NewExecError(ErrCodeFileIO, "failed to create output directory", err)
	}

	// 2. Generate Unique ID
	id := generateID()
	logPath := filepath.Join(outputDir, id+".log")
	metaPath := filepath.Join(outputDir, id+".json")

	// 3. Setup Staging File
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, NewExecError(ErrCodeFileIO, "failed to create log file", err)
	}
	defer logFile.Close()

	// 4. Setup Multi-Writers (Terminal + File + Buffer)
	var stdoutBuf, stderrBuf bytes.Buffer
	
	var stdoutWriter, stderrWriter io.Writer

	if e.Flags.Silent {
		// Only write to log file and buffer, skip os.Stdout/Stderr
		stdoutWriter = io.MultiWriter(logFile, &stdoutBuf)
		stderrWriter = io.MultiWriter(logFile, &stderrBuf)
	} else {
		// Standard behavior: Terminal + File + Buffer
		stdoutWriter = io.MultiWriter(os.Stdout, logFile, &stdoutBuf)
		stderrWriter = io.MultiWriter(os.Stderr, logFile, &stderrBuf)
	}

	// 5. Prepare Context and Command with Timeout
	// Use the timeout from flags, or default to 60s if not specified
	timeoutDuration := 60 * time.Second
	if e.Flags.TimeoutSeconds > 0 {
		timeoutDuration = time.Duration(e.Flags.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	shell, shellFlag := resolveShell()
	cmd := exec.CommandContext(ctx, shell, shellFlag, e.Command)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	
	// FIX: Use os.DevNull instead of nil for Stdin
	// Setting Stdin to nil can cause shell commands to fail silently.
	// Using os.DevNull ensures the shell has a valid input stream that is always empty.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, NewExecError(ErrCodeFileIO, "failed to open /dev/null", err)
	}
	defer devNull.Close()
	cmd.Stdin = devNull
	
	// Set Working Directory if provided
	if e.Workdir != "" {
		cmd.Dir = e.Workdir
		logger.Debug("Setting working directory", "dir", e.Workdir)
	}

	// Set Environment Variables if provided
	if len(e.Env) > 0 {
		cmd.Env = append(os.Environ(), e.Env...)
		logger.Debug("Injecting environment variables", "count", len(e.Env))
	}
	
	// Unix-specific for process group management
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// 6. Start Command
	logger.Debug("Starting command execution", "command", e.Command, "timeout", timeoutDuration, "shell", shell, "workdir", e.Workdir)
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start command", "command", e.Command, "error", err)
		return nil, NewExecError(ErrCodeCommandNotFound, "failed to start command", err)
	}
	logger.Debug("Command started successfully", "pid", cmd.Process.Pid)

	// 7. Signal Handling
	sigChan := make(chan os.Signal, 1)
	signalNotify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig, ok := <-sigChan
		if !ok {
			return
		}
		logger.Debug("Signal received, forwarding to child process", "signal", sig, "pid", cmd.Process.Pid)
		
		if runtime.GOOS == "windows" {
			cmd.Process.Signal(sig)
		} else {
			// Kill the entire process group
			syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
		}
	}()

	// 8. Wait for Completion
	err = cmd.Wait()
	
	// Stop listening for signals
	signalStop(sigChan)
	close(sigChan)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			logger.Debug("Command exited with error", "exitCode", exitCode, "error", err)
		} else {
			// Check if the error is due to context timeout
			if ctx.Err() == context.DeadlineExceeded {
				exitCode = 124 // Standard exit code for timeout (like GNU timeout)
				logger.Debug("Command execution timed out", "error", err)
			} else {
				// Likely a signal interruption or context cancellation
				exitCode = 1
				logger.Debug("Command execution failed or interrupted", "error", err)
			}
		}
	} else {
		logger.Debug("Command completed successfully", "exitCode", exitCode)
	}

	duration := time.Since(startTime)

	// 9. Save Metadata
	metadata := ExecMetadata{
		Version:   "1.0",
		Command:   e.Command,
		ExitCode:  exitCode,
		Timestamp: startTime.Format(time.RFC3339),
		Duration:  fmt.Sprintf("%.2f", duration.Seconds()),
		Flags:     e.Flags,
	}

	metaData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		logger.Warning("Failed to marshal metadata", "error", err)
	} else {
		if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
			logger.Warning("Failed to write metadata file", "error", err)
		}
	}

	// 10. Return Result
	combinedOutput := stdoutBuf.String() + stderrBuf.String()
	logger.Debug("Execution result", "outputSize", len(combinedOutput), "duration", duration)

	return &Result{
		ID:       id,
		ExitCode: exitCode,
		Duration: duration,
		Output:   combinedOutput,
		LogPath:  logPath,
		MetaPath: metaPath,
	}, nil
}

// ListOutputs retrieves all saved execution outputs.
func ListOutputs() ([]ExecOutput, error) {
	outputDir, err := settings.GetExecOutputsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ExecOutput{}, nil
		}
		return nil, err
	}

	var outputs []ExecOutput
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		metaPath := filepath.Join(outputDir, entry.Name())
		
		data, err := os.ReadFile(metaPath)
		if err != nil {
			logger.Debug("Failed to read metadata file", "path", metaPath, "error", err)
			continue
		}

		var meta ExecMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			logger.Debug("Failed to parse metadata file", "path", metaPath, "error", err)
			continue
		}

		outputs = append(outputs, ExecOutput{
			ID:        id,
			Command:   meta.Command,
			ExitCode:  meta.ExitCode,
			Timestamp: meta.Timestamp,
		})
	}

	return outputs, nil
}

// GetOutput retrieves the content and metadata of a specific execution.
func GetOutput(id string) (*Result, error) {
	outputDir, err := settings.GetExecOutputsDir()
	if err != nil {
		return nil, err
	}

	logPath := filepath.Join(outputDir, id+".log")
	metaPath := filepath.Join(outputDir, id+".json")

	// Check if files exist
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("output ID %s not found", id)
	}

	// Read Log
	logData, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Read Metadata
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta ExecMetadata
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &Result{
		ID:       id,
		Output:   string(logData),
		LogPath:  logPath,
		MetaPath: metaPath,
		ExitCode: meta.ExitCode,
	}, nil
}

// DeleteOutput removes the files associated with a specific execution ID.
func DeleteOutput(id string) error {
	outputDir, err := settings.GetExecOutputsDir()
	if err != nil {
		return err
	}

	logPath := filepath.Join(outputDir, id+".log")
	metaPath := filepath.Join(outputDir, id+".json")

	// Remove Log
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove Metadata
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// ClearOutputs removes all saved execution outputs.
func ClearOutputs() error {
	outputDir, err := settings.GetExecOutputsDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(outputDir, entry.Name())
		if err := os.Remove(path); err != nil {
			logger.Warning("Failed to delete output file", "path", path, "error", err)
		}
	}

	return nil
}

// generateID creates a unique ID based on timestamp and random characters.
func generateID() string {
	timestamp := time.Now().Format("20060102-150405")
	b := make([]byte, 3)
	rand.Read(b)
	return fmt.Sprintf("%s-%x", timestamp, b)
}

// resolveShell returns the appropriate shell and flag for the current OS.
func resolveShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd", "/c"
	}
	return "sh", "-c"
}

// signalNotify wraps signal.Notify for easier testing/mocking if needed.
var signalNotify = func(c chan<- os.Signal, sig ...os.Signal) {
	signal.Notify(c, sig...)
}

// signalStop wraps signal.Stop.
var signalStop = func(c chan<- os.Signal) {
	signal.Stop(c)
}
