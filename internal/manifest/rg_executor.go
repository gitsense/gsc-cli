/*
 * Component: Ripgrep Executor
 * Block-UUID: 2cf4136b-58e5-4d29-8b97-8c7c1eb7fb09
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Executes ripgrep as a subprocess and parses its JSON output to extract file matches.
 * Language: Go
 * Created-at: 2026-02-02T18:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
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
func ExecuteRipgrep(options RgOptions) ([]RgMatch, error) {
	// 1. Build ripgrep command
	args := buildRipgrepArgs(options)
	
	logger.Info("Executing ripgrep", "pattern", options.Pattern, "args", strings.Join(args, " "))

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
			logger.Warning("Failed to parse ripgrep JSON line: %v", err)
			continue
		}

		// Check if this is a match message
		if msgType, ok := rgMessage["type"].(string); ok && msgType == "match" {
			match, err := parseRipgrepMatch(rgMessage)
			if err != nil {
				logger.Warning("Failed to parse match: %v", err)
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

// buildRipgrepArgs constructs the argument list for the ripgrep command.
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
