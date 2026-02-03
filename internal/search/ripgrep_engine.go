/*
 * Component: Ripgrep Search Engine
 * Block-UUID: cd944dd6-3a8a-44da-a5d2-72a0c65b591f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the SearchEngine interface using ripgrep. Parses JSON output to extract matches and context lines.
 * Language: Go
 * Created-at: 2026-02-03T18:06:35.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package search

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/yourusername/gsc-cli/pkg/logger"
)

// RipgrepEngine implements SearchEngine using the ripgrep (rg) binary.
type RipgrepEngine struct{}

// Search executes ripgrep and parses the JSON output into RawMatch objects.
func (e *RipgrepEngine) Search(ctx context.Context, options SearchOptions) ([]RawMatch, error) {
	// 1. Check if ripgrep is installed
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, fmt.Errorf("ripgrep is not installed or not in PATH. Please install ripgrep: https://github.com/BurntSushi/ripgrep")
	}

	// 2. Build ripgrep command
	args := e.buildArgs(options)
	logger.Info("Executing ripgrep", "pattern", options.Pattern, "args", strings.Join(args, " "))

	// 3. Create command
	cmd := exec.CommandContext(ctx, "rg", args...)

	// 4. Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// 5. Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ripgrep: %w", err)
	}

	// 6. Parse JSON output
	matches, err := e.parseJSONOutput(stdout)
	if err != nil {
		return nil, err
	}

	// 7. Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// Ripgrep returns exit code 1 if no matches found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			logger.Info("Ripgrep found no matches")
			return []RawMatch{}, nil
		}
		return nil, fmt.Errorf("ripgrep execution failed: %w", err)
	}

	logger.Info("Ripgrep execution completed", "matches", len(matches))
	return matches, nil
}

// buildArgs constructs the argument list for ripgrep.
func (e *RipgrepEngine) buildArgs(options SearchOptions) []string {
	args := []string{
		"--json",       // Output in JSON format
		"--no-heading", // Don't group matches by file
	}

	// Add context lines
	if options.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", options.ContextLines))
	}

	// Add case sensitivity
	if !options.CaseSensitive {
		args = append(args, "--smart-case")
	}

	// Add file type filter
	if options.FileType != "" {
		args = append(args, fmt.Sprintf("--type=%s", options.FileType))
	}

	// Add the pattern
	args = append(args, options.Pattern)

	return args
}

// parseJSONOutput reads the JSON stream from ripgrep and constructs RawMatch objects.
// It handles the logic of associating context lines with matches.
func (e *RipgrepEngine) parseJSONOutput(stdout interface{}) ([]RawMatch, error) {
	// We expect stdout to be an io.Reader, but cmd.StdoutPipe() returns a ReadCloser.
	// The type assertion here is a bit loose, but in practice it works with cmd.StdoutPipe().
	// Ideally, we'd pass the reader directly.
	reader, ok := stdout.(interface{ Read([]byte) (int, error) })
	if !ok {
		return nil, fmt.Errorf("invalid stdout type")
	}

	scanner := bufio.NewScanner(reader)
	
	var matches []RawMatch
	var currentContext []string
	var lastMatch *RawMatch

	for scanner.Scan() {
		line := scanner.Text()
		
		var message map[string]interface{}
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			logger.Warning("Failed to parse ripgrep JSON line: %v", err)
			continue
		}

		msgType, _ := message["type"].(string)

		switch msgType {
		case "begin":
			// Start of a new file context
			currentContext = []string{}
			lastMatch = nil

		case "context":
			// Context line
			if lines, ok := message["lines"].(map[string]interface{}); ok {
				if text, ok := lines["text"].(string); ok {
					currentContext = append(currentContext, text)
				}
			}

		case "match":
			// Found a match
			// The context lines accumulated so far are "before" this match
			beforeContext := make([]string, len(currentContext))
			copy(beforeContext, currentContext)

			// Extract match details
			var match RawMatch
			if path, ok := message["path"].(map[string]interface{}); ok {
				if text, ok := path["text"].(string); ok {
					match.FilePath = text
				}
			}
			
			if lines, ok := message["lines"].(map[string]interface{}); ok {
				if text, ok := lines["text"].(string); ok {
					match.LineText = text
				}
				if num, ok := lines["line_number"].(float64); ok {
					match.LineNumber = int(num)
				}
			}

			// If there was a previous match, the current context buffer is its "after" context
			if lastMatch != nil {
				lastMatch.ContextAfter = make([]string, len(currentContext))
				copy(lastMatch.ContextAfter, currentContext)
			}

			// Add the new match
			matches = append(matches, match)
			lastMatch = &matches[len(matches)-1]

			// Reset context buffer for the next match's "before" context
			currentContext = []string{}

		case "end":
			// End of file context
			// Any remaining context lines are "after" the last match
			if lastMatch != nil && len(currentContext) > 0 {
				lastMatch.ContextAfter = make([]string, len(currentContext))
				copy(lastMatch.ContextAfter, currentContext)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read ripgrep output: %w", err)
	}

	return matches, nil
}
