/**
 * Component: Ripgrep Search Engine
 * Block-UUID: df7edd10-46ac-4a38-81a6-c75f5cf6a8c0
 * Parent-UUID: f07fa5a4-de49-43df-8d17-0f7f515fb0b1
 * Version: 2.4.1
 * Description: Added FindBlockByUUID and EnsureUUIDUniqueness methods to support the Contract feature. Implemented formatBlockUUIDPattern helper to enforce strict header formatting (leading space, single trailing space).
 * Language: Go
 * Created-at: 2026-02-26T05:22:23.250Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0), Gemini 3 Flash (v2.3.0), Gemini 3 Flash (v2.3.1), GLM-4.7 (v2.4.0), Gemini 3 Flash (v2.4.1)
 */


package search

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/pkg/logger"
)

// RipgrepEngine implements SearchEngine using the ripgrep (rg) binary.
type RipgrepEngine struct{}

// Search executes ripgrep and parses the JSON output into SearchResult.
func (e *RipgrepEngine) Search(ctx context.Context, options SearchOptions) (SearchResult, error) {
	startTime := time.Now()

	// 1. Check if ripgrep is installed
	if _, err := exec.LookPath("rg"); err != nil {
		return SearchResult{}, fmt.Errorf("ripgrep is not installed or not in PATH. Please install ripgrep: https://github.com/BurntSushi/ripgrep")
	}

	// 2. Get ripgrep version
	version, err := getRipgrepVersion()
	if err != nil {
		logger.Warning("Failed to get ripgrep version", "error", err)
		version = "unknown"
	}

	// 3. Build ripgrep command
	args := e.buildArgs(options)
	logger.Debug("Executing ripgrep", "pattern", options.Pattern, "args", strings.Join(args, " "))

	// 4. Create command
	cmd := exec.CommandContext(ctx, "rg", args...)

	// 5. Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return SearchResult{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// 6. Start command
	if err := cmd.Start(); err != nil {
		return SearchResult{}, fmt.Errorf("failed to start ripgrep: %w", err)
	}

	// 7. Parse JSON output
	matches, err := e.parseJSONOutput(stdout)
	if err != nil {
		return SearchResult{}, err
	}

	// 8. Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// Ripgrep returns exit code 1 if no matches found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			logger.Debug("Ripgrep found no matches")
			return SearchResult{
				Matches:     []RawMatch{},
				ToolName:    "ripgrep",
				ToolVersion: version,
				DurationMs:  int(time.Since(startTime).Milliseconds()),
			}, nil
		}
		return SearchResult{}, fmt.Errorf("ripgrep execution failed: %w", err)
	}

	duration := int(time.Since(startTime).Milliseconds())
	logger.Debug("Ripgrep execution completed", "matches", len(matches), "duration_ms", duration)

	return SearchResult{
		Matches:     matches,
		ToolName:    "ripgrep",
		ToolVersion: version,
		DurationMs:  duration,
	}, nil
}

// FindBlockByUUID searches the workdir for a specific Block-UUID.
// It returns the absolute file path and the line number of the match.
 // It enforces strict formatting: 5920cf5a-39a5-4f18-b758-57b66c5c3746
func (e *RipgrepEngine) FindBlockByUUID(ctx context.Context, workdir string, uuid string) (string, int, error) {
	pattern := formatBlockUUIDPattern(uuid)
	
	args := []string{"--json", "-i", pattern}
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = workdir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", 0, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", 0, fmt.Errorf("failed to start ripgrep: %w", err)
	}

	var matches []struct {
		Path string
		Line int
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var message map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}

		if msgType, ok := message["type"].(string); ok && msgType == "match" {
			if data, ok := message["data"].(map[string]interface{}); ok {
				if path, ok := data["path"].(map[string]interface{}); ok {
					if text, ok := path["text"].(string); ok {
						if num, ok := data["line_number"].(float64); ok {
							matches = append(matches, struct {
								Path string
								Line int
							}{text, int(num)})
						}
					}
				}
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// No matches found
			return "", 0, fmt.Errorf("Block-UUID %s not found in workdir", uuid)
		}
		return "", 0, fmt.Errorf("ripgrep execution failed: %w", err)
	}

	if len(matches) == 0 {
		return "", 0, fmt.Errorf("Block-UUID %s not found in workdir", uuid)
	}

	if len(matches) > 1 {
		return "", 0, fmt.Errorf("data integrity violation: Block-UUID %s found in %d locations", uuid, len(matches))
	}

	return matches[0].Path, matches[0].Line, nil
}

// EnsureUUIDUniqueness checks if a Block-UUID already exists in the workdir.
// It returns an error if the UUID is found, ensuring global uniqueness.
func (e *RipgrepEngine) EnsureUUIDUniqueness(ctx context.Context, workdir string, uuid string) error {
	pattern := formatBlockUUIDPattern(uuid)
	
	args := []string{"--json", "-i", pattern}
	cmd := exec.CommandContext(ctx, "rg", args...)
	cmd.Dir = workdir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ripgrep: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var message map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}

		if msgType, ok := message["type"].(string); ok && msgType == "match" {
			// Found at least one match
			_ = cmd.Wait() // Clean up process
			return fmt.Errorf("Block-UUID %s already exists in workdir", uuid)
		}
	}

	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// No matches found, which is success
			return nil
		}
		return fmt.Errorf("ripgrep execution failed: %w", err)
	}

	return nil
}

// formatBlockUUIDPattern constructs the strict search pattern for a Block-UUID.
func formatBlockUUIDPattern(uuid string) string {
 	return fmt.Sprintf(" Block-UUID: %s", uuid)
}

// getRipgrepVersion executes 'rg --version' and returns the version string.
func getRipgrepVersion() (string, error) {
	cmd := exec.Command("rg", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Output is typically "ripgrep 13.0.0\n..."
	// We just need the first line
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		// Extract version number (e.g., "13.0.0")
		re := regexp.MustCompile(`\d+\.\d+\.\d+`)
		matches := re.FindString(lines[0])
		if matches != "" {
			return matches, nil
		}
		return strings.TrimSpace(lines[0]), nil
	}

	return "", fmt.Errorf("could not parse ripgrep version")
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
			logger.Warning("Failed to parse ripgrep JSON line", "error", err)
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
			if data, ok := message["data"].(map[string]interface{}); ok {
				if lines, ok := data["lines"].(map[string]interface{}); ok {
					if text, ok := lines["text"].(string); ok {
						currentContext = append(currentContext, text)
					}
				}
			}

		case "match":
			// Found a match
			// The context lines accumulated so far are "before" this match
			beforeContext := make([]string, len(currentContext))
			copy(beforeContext, currentContext)

			// Extract match details
			var match RawMatch
			if data, ok := message["data"].(map[string]interface{}); ok {
				if path, ok := data["path"].(map[string]interface{}); ok {
					if text, ok := path["text"].(string); ok {
						match.FilePath = text
					}
				}
				
				if lines, ok := data["lines"].(map[string]interface{}); ok {
					if text, ok := lines["text"].(string); ok {
						match.LineText = text
					}
				}

				if num, ok := data["line_number"].(float64); ok {
					match.LineNumber = int(num)
				} else {
					logger.Debug("Line number not found in standard location for match", "text", match.LineText)
				}

				// Extract submatches for highlighting
				if submatches, ok := data["submatches"].([]interface{}); ok {
					for _, sm := range submatches {
						if smMap, ok := sm.(map[string]interface{}); ok {
							start, okStart := smMap["start"].(float64)
							end, okEnd := smMap["end"].(float64)
							if okStart && okEnd {
								match.Submatches = append(match.Submatches, MatchOffset{
									Start: int(start),
									End:   int(end),
								})
							}
						}
					}
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
