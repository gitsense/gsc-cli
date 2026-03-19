/**
 * Component: Docker Signal Watcher
 * Block-UUID: d57d2e80-f609-44e6-ad0a-242df210c372
 * Parent-UUID: c696e665-c7f9-411c-8bd1-23ed2976c52b
 * Version: 1.2.0
 * Description: Enhanced signal logging, implemented graceful shutdown with SIGTERM/SIGKILL, and increased scanner buffer to 1MB.
 * Language: Go
 * Created-at: 2026-03-19T02:28:34.992Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// DockerSignal represents the JSON payload extracted from the Signal Envelope.
type DockerSignal struct {
	Action string `json:"action"`         // e.g., "launch"
	Type   string `json:"type"`           // e.g., "terminal", "editor"
	Path   string `json:"path"`           // The container-side absolute path
	Alias  string `json:"alias,omitempty"` // The template alias to use (e.g., "iterm2", "code")
}

// WatchLogs tails the container logs and scans for the Signal Envelope.
func WatchLogs(ctx context.Context, dctx DockerContext) error {
	logger.Info("Starting Docker Signal Watcher", "container", dctx.ContainerName)
	fmt.Fprintf(os.Stderr, "🐳 [gsc] Watching logs for container '%s'...\n", dctx.ContainerName)
	fmt.Fprintln(os.Stderr, "   (Listening for terminal/editor launch signals. Ctrl+C to stop)")

	args := []string{"logs", "-f", "--tail", "0", dctx.ContainerName}
	cmd := exec.CommandContext(ctx, "docker", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start log tailing: %w", err)
	}

	// Graceful shutdown: send SIGTERM first, then SIGKILL if needed
	go func() {
		<-ctx.Done()
		logger.Debug("Context cancelled, initiating graceful shutdown of docker logs process")
		cmd.Process.Signal(syscall.SIGTERM)
		
		// If the process doesn't exit within 2 seconds, force kill
		time.Sleep(2 * time.Second)
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			logger.Debug("Process did not exit gracefully, forcing kill")
			cmd.Process.Kill()
		}
	}()

	// Regex to find the envelope: @@GSC_SIGNAL:{...}@@
	re := regexp.MustCompile(`@@GSC_SIGNAL:(\{.*?\})@@`)

	scanner := bufio.NewScanner(stdout)
	const maxCapacity = 1024 * 1024 // 1MB buffer for large signals
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()
		
		// Check for signal
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			signalJSON := matches[1]
			logger.Debug("Signal detected", "payload", signalJSON)
			
			if err := handleSignal(signalJSON, dctx); err != nil {
				logger.Error("Failed to handle signal", "error", err)
				fmt.Fprintf(os.Stderr, "❌ [watcher] Signal error: %v\n", err)
			}
		} else {
			// Print regular logs to stderr to keep stdout clean for potential piping
			fmt.Fprintln(os.Stderr, line)
		}
	}

	return cmd.Wait()
}

// handleSignal parses the signal JSON and executes the corresponding host action.
func handleSignal(signalJSON string, dctx DockerContext) error {
	var sig DockerSignal
	if err := json.Unmarshal([]byte(signalJSON), &sig); err != nil {
		return fmt.Errorf("failed to parse signal JSON: %w", err)
	}

	if sig.Action != "launch" {
		return fmt.Errorf("unsupported signal action: %s", sig.Action)
	}

	// 1. Translate Path (Container -> Host)
	hostPath, err := TranslatePathToHost(sig.Path, &dctx)
	if err != nil {
		return fmt.Errorf("failed to translate path: %w", err)
	}

	// 2. Resolve Command Template
	var template string
	var category string

	switch sig.Type {
	case "terminal":
		category = "terminal"
		template = settings.DefaultTerminalTemplates[sig.Alias]
		if template == "" {
			// Fallback to first available terminal if alias not found
			for _, t := range settings.DefaultTerminalTemplates {
				template = t
				break
			}
		}
	case "editor":
		category = "editor"
		template = settings.DefaultEditorTemplates[sig.Alias]
		if template == "" {
			// Fallback to first available editor if alias not found
			for _, e := range settings.DefaultEditorTemplates {
				template = e
				break
			}
		}
	default:
		return fmt.Errorf("unsupported launch type: %s", sig.Type)
	}

	if template == "" {
		return fmt.Errorf("no %s templates configured on host", category)
	}

	// 3. Construct and Execute Command
	// We assume the template uses %s for the path
	cmdStr := fmt.Sprintf(template, shellQuote(hostPath))
	
	fmt.Fprintf(os.Stderr, "🚀 [watcher] Launching %s: %s\n", category, hostPath)
	logger.Info("Executing host command", "command", cmdStr)

	// Execute using the host's shell
	shell, flag := resolveShell()
	cmd := exec.Command(shell, flag, cmdStr)
	
	// We don't wait for the command to finish (e.g., opening an editor shouldn't block the watcher)
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ [watcher] Failed to launch %s: %v\n", category, err)
		return fmt.Errorf("failed to launch %s: %w", category, err)
	}

	fmt.Fprintf(os.Stderr, "✅ [watcher] %s launched successfully at: %s\n", category, hostPath)
	logger.Info("Host command executed", "category", category, "path", hostPath, "command", cmdStr)

	return nil
}

// resolveShell returns the appropriate shell and flag for the current OS.
func resolveShell() (string, string) {
	// Re-implementing here to avoid circular dependency with internal/exec
	// In a real implementation, this would be in a shared internal/util package.
	if os.PathSeparator == '\\' {
		return "cmd", "/c"
	}
	return "sh", "-c"
}

// shellQuote escapes a string for safe use in a shell command.
// It wraps the string in single quotes and escapes any internal single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
