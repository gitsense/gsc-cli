/**
 * Component: Exec Command Executor
 * Block-UUID: c6f9c3d5-3227-49f5-83ad-a8aec9fca813
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Handles the execution of external commands, signal forwarding, output streaming to terminal and staging files, and persistence of execution metadata. Fixed unused context, missing parentheses in string conversion, and missing signal package imports.
 * Language: Go
 * Created-at: 2026-02-23T03:12:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
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
func NewExecutor(command string, flags ExecFlags) *Executor {
	return &Executor{
		Command: command,
		Flags:   flags,
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

	// 4. Setup Multi-Writers (Terminal + File)
	var stdoutBuf, stderrBuf bytes.Buffer
	
	// Writer for Stdout: Terminal + File + Buffer
	stdoutWriter := io.MultiWriter(os.Stdout, logFile, &stdoutBuf)
	
	// Writer for Stderr: Terminal + File + Buffer
	stderrWriter := io.MultiWriter(os.Stderr, logFile, &stderrBuf)

	// 5. Prepare Context and Command
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shell, shellFlag := resolveShell()
	cmd := exec.CommandContext(ctx, shell, shellFlag, e.Command)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	
	// Unix-specific for process group management
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// 6. Start Command
	logger.Debug("Starting command execution", "command", e.Command)
	if err := cmd.Start(); err != nil {
		return nil, NewExecError(ErrCodeCommandNotFound, "failed to start command", err)
	}

	// 7. Signal Handling
	sigChan := make(chan os.Signal, 1)
	signalNotify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig, ok := <-sigChan
		if !ok {
			return
		}
		logger.Debug("Signal received, forwarding to child process", "signal", sig)
		
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
		} else {
			// Likely a signal interruption or context cancellation
			exitCode = 1
			logger.Debug("Command execution failed or interrupted", "error", err)
		}
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
