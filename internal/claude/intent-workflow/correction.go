/**
 * Component: Intent Workflow Correction
 * Block-UUID: db12a45c-4092-48b7-b2b7-febc1836b1ab
 * Parent-UUID: b412d44f-9261-495b-95f7-e9cd534792af
 * Version: 1.4.0
 * Description: Spawning and handling correction subprocesses - building prompts, executing the correction turn, and parsing results. Fixed extractStreamCost to check root level for total_cost_usd before checking inside usage object. FIXED response_format.md copy logic to always copy the file (not just if it doesn't exist) to ensure turn directory has the latest version.
 * Language: Go
 * Created-at: 2026-04-25T12:47:40.091Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 3 Flash (v1.3.0), GLM-4.7 (v1.4.0)
 */


package intent_workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
)


// buildCorrectionSystemPrompt returns the static system prompt injected into
// every correction turn subprocess.
func buildCorrectionSystemPrompt() string {
	return "You are a JSON format correction assistant. Your only task is to fix a " +
		"malformed JSON response to match the exact schema described in response-format.md.\n\n" +
		"CRITICAL OUTPUT REQUIREMENT:\n" +
		"You must wrap the corrected JSON inside a specific wrapper object. " +
		"Do NOT return the corrected JSON directly at the top level.\n\n" +
		"Required Wrapper Structure:\n" +
		"{\n" +
		"  \"status\": \"success\" | \"insufficient_data\" | \"failed\",\n" +
		"  \"corrected_output\": { <--- THE FIXED JSON GOES HERE },\n" +
		"  \"reasoning\": \"Explanation of what was fixed\",\n" +
		"  \"errors_fixed\": [\"List of errors fixed\"],\n" +
		"  \"errors_remaining\": [\"List of errors that could not be fixed\"]\n" +
		"}\n\n" +
		"EXAMPLE:\n" +
		"Input (Malformed):\n" +
		"{\n" +
		"  \"status\": \"complete\",\n" +
		"  \"candidates\": [...],\n" +
		"  \"keyword_assessment\": {\n" +
		"    \"effectiveness\": {\n" +
		"      \"contract\": { \"matches\": \"invalid_string_type\" }\n" +
		"    }\n" +
		"  }\n" +
		"}\n\n" +
		"Output (Corrected):\n" +
		"{\n" +
		"  \"status\": \"success\",\n" +
		"  \"corrected_output\": {\n" +
		"    \"status\": \"complete\",\n" +
		"    \"candidates\": [...],\n" +
		"    \"keyword_assessment\": {\n" +
		"      \"effectiveness\": {\n" +
		"        \"contract\": { \"matches\": [\"file1.go\", \"file2.go\"] }\n" +
		"      }\n" +
		"    }\n" +
		"  },\n" +
		"  \"reasoning\": \"Fixed 'matches' field from string to array of strings.\",\n" +
		"  \"errors_fixed\": [\"keyword_assessment.effectiveness[\\\"contract\\\"]: matches must be an array\"],\n" +
		"  \"errors_remaining\": []\n" +
		"}\n\n" +
		"Rules:\n" +
		"- Use the Read tool only to read files in the current directory.\n" +
		"- Output ONLY the wrapper JSON object - no markdown fences, no prose.\n" +
		"- DO NOT wrap the output in markdown code blocks (```json ... ```)\n" +
		"- Ensure the 'corrected_output' field contains the fully fixed discovery/change schema.\n"
}

// buildCorrectionPrompt assembles the user-facing prompt for a correction
// subprocess. It embeds the detected format errors and the raw malformed
// response so the model has full context without extra file reads.
func buildCorrectionPrompt(attempt int, formatErrorsJSON, badResponseJSON string) string {
	return fmt.Sprintf(
		"Correction attempt %d.\n\n"+
			"Read \"response-format.md\" in this directory for the required JSON schema.\n\n"+
			"## Format Errors\n\n%s\n\n"+
			"## Malformed Response\n\n%s\n\n"+
			"Correct the JSON to match the schema exactly and return the correction result.",
		attempt, formatErrorsJSON, badResponseJSON,
	)
}

// extractCorrectionResultText scans NDJSON output from a stream-json Claude
// subprocess and returns the assistant's final text block, which is expected
// to contain the CorrectionResult JSON. Returns an error if no text is found.
func extractCorrectionResultText(ndjsonOutput []byte) (string, error) {
	var lastText string
	for _, raw := range strings.Split(string(ndjsonOutput), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			continue
		}
		switch event["type"] {
		case "assistant":
			msg, _ := event["message"].(map[string]interface{})
			content, _ := msg["content"].([]interface{})
			for _, block := range content {
				b, _ := block.(map[string]interface{})
				if b["type"] == "text" {
					if text, ok := b["text"].(string); ok {
						lastText = text
					}
				}
			}
		case "result":
			if result, ok := event["result"].(string); ok && result != "" {
				lastText = result
			}
		}
	}
	if lastText == "" {
		return "", fmt.Errorf("no text content found in correction subprocess output")
	}
	lastText = strings.TrimSpace(lastText)
	for _, fence := range []string{"```json", "```"} {
		if strings.HasPrefix(lastText, fence) {
			lastText = strings.TrimSpace(strings.TrimPrefix(lastText, fence))
			lastText = strings.TrimSpace(strings.TrimSuffix(lastText, "```"))
			break
		}
	}
	return lastText, nil
}

// extractStreamCost returns the total cost in USD from the result event in
// stream-json NDJSON output, or 0 if the event is absent or malformed.
// It checks the root level first (current CLI format) and falls back to
// checking inside the usage object (legacy format).
func extractStreamCost(ndjsonOutput []byte) float64 {
	for _, raw := range strings.Split(string(ndjsonOutput), "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			continue
		}
		if t, _ := event["type"].(string); t != "result" {
			continue
		}
		
		// Check root level first (current CLI format)
		if cost, ok := event["total_cost_usd"].(float64); ok {
			return cost
		}
		
		// Fallback: Check inside usage (legacy format)
		if usage, ok := event["usage"].(map[string]interface{}); ok {
			if cost, ok := usage["total_cost_usd"].(float64); ok {
				return cost
			}
		}
	}
	return 0
}

// spawnCorrectionSubprocess prepares correction-turn files, executes the
// Claude correction subprocess synchronously, and on success calls
// updateSessionWithCorrectedResults to persist the corrected data.
//
// stream.go must have written bad-response.json and format-errors.json to
// the turn directory before this method is called.
func (m *Manager) spawnCorrectionSubprocess(turnNumber int, modelID string) error {
	turnState := m.getTurnState(turnNumber)
	if turnState == nil {
		return fmt.Errorf("correction: turn %d not found in session", turnNumber)
	}
	turnDir := m.config.GetTurnDir(turnNumber)

	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// Copy response-format.md to turn directory
	// FIXED: Always copy the file to ensure the turn directory has the latest version
	formatSrc := getFormatFile(gscHome, turnState.TurnType)
	if formatSrc == "" {
		return fmt.Errorf("no format file defined for turn type %q", turnState.TurnType)
	}
	formatDest := filepath.Join(turnDir, "response-format.md")
	if err := copyFile(formatSrc, formatDest); err != nil {
		return fmt.Errorf("failed to copy response-format.md: %w", err)
	}

	// Write static correction system prompt.
	systemPromptPath := filepath.Join(turnDir, "correction-system-prompt.md")
	if err := os.WriteFile(systemPromptPath, []byte(buildCorrectionSystemPrompt()), 0644); err != nil {
		return fmt.Errorf("failed to write correction system prompt: %w", err)
	}

	// Read input files previously written by stream.go.
	badResponse, err := os.ReadFile(filepath.Join(turnDir, "bad-response.json"))
	if err != nil {
		return fmt.Errorf("failed to read bad-response.json: %w", err)
	}
	formatErrors, err := os.ReadFile(filepath.Join(turnDir, "format-errors.json"))
	if err != nil {
		return fmt.Errorf("failed to read format-errors.json: %w", err)
	}

	// Write correction user prompt.
	prompt := buildCorrectionPrompt(turnState.CorrectionAttempts, string(formatErrors), string(badResponse))
	promptPath := filepath.Join(turnDir, "correction-prompt.md")
	if err := os.WriteFile(promptPath, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("failed to write correction prompt: %w", err)
	}

	// Build the subprocess command.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		promptContent, readErr := os.ReadFile(promptPath)
		if readErr != nil {
			return fmt.Errorf("failed to read correction prompt: %w", readErr)
		}
		args := []string{
			"--allowedTools", "Read",
			"--verbose",
			"--include-partial-messages",
			"--output-format", "stream-json",
			"--append-system-prompt-file", systemPromptPath,
			"--model", modelID,
			"-p", string(promptContent),
		}
		cmd = exec.Command("claude", args...)
	} else {
		logPath := filepath.Join(turnDir,
			fmt.Sprintf("correction-raw-stream-%d.ndjson", time.Now().UnixNano()))
		scriptContent := fmt.Sprintf(`#!/bin/bash
set -e
cd "%s"
claude --allowedTools Read \
--verbose \
--include-partial-messages \
--output-format stream-json \
--append-system-prompt-file correction-system-prompt.md \
--model %s \
-p "$(cat correction-prompt.md)" | tee "%s"
`, turnDir, modelID, logPath)
		scriptPath := filepath.Join(turnDir, "run-correction.sh")
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return fmt.Errorf("failed to write correction script: %w", err)
		}
		cmd = exec.Command("/bin/bash", scriptPath)
	}
	cmd.Dir = turnDir

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = os.Stderr

	m.debugLogger.Log("DEBUG", fmt.Sprintf(
		"Starting correction subprocess for turn %d (attempt %d, model %s)",
		turnNumber, turnState.CorrectionAttempts, modelID,
	))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("correction subprocess exited with error: %w", err)
	}

	// Extract Claude's text response from the NDJSON stream.
	resultText, err := extractCorrectionResultText(outputBuf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to extract correction result text: %w", err)
	}

	// Persist the raw result for auditability.
	resultPath := filepath.Join(turnDir,
		fmt.Sprintf("correction-result-%d.json", turnState.CorrectionAttempts))
	_ = os.WriteFile(resultPath, []byte(resultText), 0644)

	// Parse, validate, and apply the corrected results.
	correctionResult, err := ParseCorrectionResult(resultText)
	if err != nil {
		return fmt.Errorf("failed to parse correction result: %w", err)
	}
	if correctionResult.Status != CorrectionStatusSuccess {
		return fmt.Errorf("correction %s: %s", correctionResult.Status, correctionResult.Reasoning)
	}
	correctedJSON, err := correctionResult.GetCorrectedDiscoveryJSON()
	if err != nil {
		return fmt.Errorf("failed to extract corrected JSON: %w", err)
	}
	correctedResults, err := ParseDiscoveryResult(correctedJSON)
	if err != nil {
		return fmt.Errorf("corrected output failed re-parse: %w", err)
	}
	if validationErrs := ValidateDiscoveryResult(correctedResults); len(validationErrs) > 0 {
		return fmt.Errorf("corrected output has %d remaining format errors", len(validationErrs))
	}

	return m.updateSessionWithCorrectedResults(turnNumber, correctedResults, extractStreamCost(outputBuf.Bytes()))
}

// SpawnMetadataCorrectionSubprocess spawns a Haiku subprocess to fix malformed metadata files.
// It reads the bad-metadata-files.json and writes corrected content back to the original files.
func SpawnMetadataCorrectionSubprocess(turnDir, badMetaPath string) error {
	// Resolve the correction model ID from the default family
	modelID, err := GetModelID(DefaultCorrectionModel)
	if err != nil {
		return fmt.Errorf("invalid correction model: %w", err)
	}

	prompt := fmt.Sprintf(`Read the file at %s. It contains a list of malformed JSON files.
For each file, fix the JSON syntax and structure to match the GSCFileData schema.
Return a JSON object mapping file paths to corrected JSON content.
Do NOT wrap the output in markdown code blocks.`, badMetaPath)

	cmd := exec.Command("claude", "-p", prompt, "--model", modelID)
	cmd.Dir = turnDir

	// Use Output instead of CombinedOutput to prevent stderr from corrupting JSON
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("correction command failed: %w, output: %s", err, string(output))
	}

	// Parse correction result
	var corrections map[string]string
	if err := json.Unmarshal(output, &corrections); err != nil {
		return fmt.Errorf("failed to parse correction output: %w", err)
	}

	// Write corrected files
	for filePath, correctedContent := range corrections {
		if err := os.WriteFile(filePath, []byte(correctedContent), 0644); err != nil {
			return fmt.Errorf("failed to write corrected file %s: %w", filePath, err)
		}
	}

	return nil
}
