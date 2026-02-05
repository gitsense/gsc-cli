/**
 * Component: Ripgrep Executor
 * Block-UUID: 2d5dc862-5334-4c7c-9368-ca35d36ad535
 * Parent-UUID: cc5c385a-d23c-4761-80ee-637657bd9c27
 * Version: 1.2.0
 * Description: Executes ripgrep as a subprocess. Added ExecuteRipgrepRaw to support the dual-pass workflow, preserving terminal colors and standard formatting for the display pass. Refactored all logger calls to use structured Key-Value pairs instead of format strings.
 * Language: Go
 * Created-at: 2026-02-02T19:09:26.833Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package manifest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/yourusername/gsc-cli/pkg/logger"
)

// ExecuteRipgrep runs ripgrep with the specified options and returns the raw matches.
// This is used for the "Discovery" pass (JSON mode).
func ExecuteRipgrep(options RgOptions) ([]RgMatch, error) {
	// 0. Check if ripgrep is installed
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, fmt.Errorf("ripgrep is not installed or not in PATH. Please install ripgrep: https://github.com/BurntSushi/ripgrep")
	}

	// 1. Build ripgrep command
	args := buildRipgrepArgs(options)
	
	logger.Info("Executing ripgrep (Discovery Pass)", "pattern", options.Pattern, "args", strings.Join(args, " "))

	// 2. Create command
	cmd := exec.Command("rg", args...)

	// 3. Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// 4. Start command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ripgrep: %w", err)
	}

	// 5. Parse JSON output line by line
	var matches []RgMatch
	scanner := bufio.NewScanner(stdout)
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// Ripgrep JSON output contains different message types
		// We only care about "match" messages
		var rgMessage map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rgMessage); err != nil {
			logger.Warning("Failed to parse ripgrep JSON line", "error", err)
			continue
		}

		// Check if this is a match message
		if msgType, ok := rgMessage["type"].(string); ok && msgType == "match" {
			match, err := parseRipgrepMatch(rgMessage)
			if err != nil {
				logger.Warning("Failed to parse match", "error", err)
				continue
			}
			matches = append(matches, match)
		}
	}

	// 6. Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// Ripgrep returns exit code 1 if no matches found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			logger.Info("Ripgrep found no matches")
			return []RgMatch{}, nil
		}
		return nil, fmt.Errorf("ripgrep execution failed: %w", err)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read ripgrep output: %w", err)
	}

	logger.Info("Ripgrep execution completed", "matches", len(matches))
	return matches, nil
}

// ExecuteRipgrepRaw runs ripgrep with color output enabled and returns the raw stdout string.
// This is used for the "Display" pass to preserve terminal formatting.
func ExecuteRipgrepRaw(pattern string, contextLines int, caseSensitive bool, fileType string) (string, error) {
	// 0. Check if ripgrep is installed
	if _, err := exec.LookPath("rg"); err != nil {
		return "", fmt.Errorf("ripgrep is not installed or not in PATH. Please install ripgrep: https://github.com/BurntSushi/ripgrep")
	}

	// 1. Build ripgrep command for raw output
	args := buildRawRipgrepArgs(pattern, contextLines, caseSensitive, fileType)
	
	logger.Info("Executing ripgrep (Display Pass)", "pattern", pattern, "args", strings.Join(args, " "))

	// 2. Create command
	cmd := exec.Command("rg", args...)

	// 3. Run and capture output
	output, err := cmd.Output()
	if err != nil {
		// Ripgrep returns exit code 1 if no matches found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			logger.Info("Ripgrep found no matches")
			return "", nil
		}
		return "", fmt.Errorf("ripgrep execution failed: %w", err)
	}

	return string(output), nil
}

// buildRipgrepArgs constructs the argument list for the ripgrep command (JSON mode).
func buildRipgrepArgs(options RgOptions) []string {
	args := []string{
		"--json",           // Output in JSON format
		"--no-heading",     // Don't group matches by file
		"--no-line-number", // We'll parse line numbers from JSON
	}

	// Add context lines if specified
	if options.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", options.ContextLines))
	}

	// Add case sensitivity
	if !options.CaseSensitive {
		args = append(args, "--smart-case")
	}

	// Add file type filter if specified
	if options.FileType != "" {
		args = append(args, fmt.Sprintf("--type=%s", options.FileType))
	}

	// Add the pattern
	args = append(args, options.Pattern)

	return args
}

// buildRawRipgrepArgs constructs the argument list for the ripgrep command (Raw/Color mode).
func buildRawRipgrepArgs(pattern string, contextLines int, caseSensitive bool, fileType string) []string {
	args := []string{
		"--color=always", // Force color output even when piped
	}

	// Add context lines if specified
	if contextLines > 0 {
		args = append(args, fmt.Sprintf("-C%d", contextLines))
	}

	// Add case sensitivity
	if !caseSensitive {
		args = append(args, "--smart-case")
	}

	// Add file type filter if specified
	if fileType != "" {
		args = append(args, fmt.Sprintf("--type=%s", fileType))
	}

	// Add the pattern
	args = append(args, pattern)

	return args
}

// parseRipgrepMatch parses a ripgrep JSON match message into an RgMatch struct.
func parseRipgrepMatch(message map[string]interface{}) (RgMatch, error) {
	var match RgMatch

	// Extract file path
	if path, ok := message["path"].(map[string]interface{}); ok {
		if text, ok := path["text"].(string); ok {
			match.FilePath = text
		}
	}

	// Extract line number
	if lines, ok := message["lines"].(map[string]interface{}); ok {
		if lineMap, ok := lines["line_number"].(float64); ok {
			match.LineNumber = int(lineMap)
		}
	}

	// Extract line text
	if lines, ok := message["lines"].(map[string]interface{}); ok {
		if text, ok := lines["text"].(string); ok {
			match.LineText = text
		}
	}

	// Extract submatches (the actual matched text)
	if submatches, ok := message["submatches"].([]interface{}); ok && len(submatches) > 0 {
		if firstMatch, ok := submatches[0].(map[string]interface{}); ok {
			if matchText, ok := firstMatch["match"].(map[string]interface{}); ok {
				if text, ok := matchText["text"].(string); ok {
					match.MatchText = text
				}
			}
		}
	}

	// Fallback: if no submatch text, use the full line text
	if match.MatchText == "" {
		match.MatchText = match.LineText
	}

	return match, nil
}
