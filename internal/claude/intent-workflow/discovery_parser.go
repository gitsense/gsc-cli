/**
 * Component: Discovery Result Parser
 * Block-UUID: 8f7a3b2c-1d4e-4f5a-9b8c-7d6e5f4a3b2c
 * Parent-UUID: 6dee9aa8-5752-4543-bdd6-8f4cf5763028
 * Version: 2.9.0
 * Description: Parses JSON results from Claude discovery turns into generic TurnResults. Added three-tier fallback parsing strategy: direct unmarshal, regex extraction, and markdown parser fallback. Added extractJSONFromMarkdown(), findMatchingBrace(), and extractJSONUsingMarkdownParser() helper functions. Fixed regex patterns to use regular string literals instead of raw strings with backticks. Added hierarchical validation for discovery_mode and brain_effectiveness fields to support hybrid discovery strategy. Added SuccinctResponse field to discoveryResult struct and updated buildTurnResults to pass through succinct_natural_language_response. Added succinct_natural_language_response to knownDiscoveryTopLevelFields whitelist.
 * Language: Go
 * Created-at: 2026-04-26T18:01:01.205Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v2.0.0), Gemini 2.5 Flash Lite (v2.1.0), Gemini 2.5 Flash Lite (v2.2.0), GLM-4.7 (v2.3.0), GLM-4.7 (v2.4.0), GLM-4.7 (v2.5.0), GLM-4.7 (v2.6.0), GLM-4.7 (v2.7.0), GLM-4.7 (v2.8.0), GLM-4.7 (v2.9.0)
 */


package intent_workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gitsense/gsc-cli/internal/markdown"
)

// discoveryResult represents the JSON structure expected from a discovery turn
type discoveryResult struct {
	Status              string              `json:"status,omitempty"` // "complete", "out_of_scope", "failed"
	SuccinctResponse    string              `json:"succinct_natural_language_response,omitempty"` // Optional natural language summary
	Candidates          []Candidate         `json:"candidates"`
	MissingFiles        []MissingFile       `json:"missing_files,omitempty"`
	KeywordAssessment   *KeywordAssessment  `json:"keyword_assessment,omitempty"`
	Duration            *int64              `json:"duration,omitempty"`
	Cost                *float64            `json:"cost,omitempty"`
	Usage               *Usage              `json:"usage,omitempty"`
	TotalFound          int                 `json:"total_found"`
	Coverage            string              `json:"coverage"`
	DiscoveryLog        *DiscoveryLog       `json:"discovery_log"`
	DiscoveryMode       string              `json:"discovery_mode"`     // "experts" or "generic"
	BrainEffectiveness  *BrainEffectiveness `json:"brain_effectiveness,omitempty"` // nil when DiscoveryMode == "generic"
}

// ParseDiscoveryResult attempts to parse a JSON string as a discovery result.
// It uses a three-tier fallback strategy:
// 1. Direct JSON unmarshal (fast path for pure JSON)
// 2. Regex extraction for markdown fences
// 3. Markdown parser fallback for complex markdown
// Returns a populated TurnResults struct or an error if all tiers fail.
func ParseDiscoveryResult(jsonContent string) (*TurnResult, error) {
	content := strings.TrimSpace(jsonContent)
	
	// Tier 1: Try direct JSON unmarshal (most common case)
	var result discoveryResult
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return buildTurnResults(content, result), nil
	}
	
	// Tier 2: Try regex extraction for markdown fences
	extracted := extractJSONFromMarkdown(content)
	if extracted != "" {
		if err := json.Unmarshal([]byte(extracted), &result); err == nil {
			return buildTurnResults(extracted, result), nil
		}
	}
	
	// Tier 3: Try markdown parser fallback
	extracted = extractJSONUsingMarkdownParser(content)
	if extracted != "" {
		if err := json.Unmarshal([]byte(extracted), &result); err == nil {
			return buildTurnResults(extracted, result), nil
		}
	}
	
	// All tiers failed
	return nil, fmt.Errorf("failed to parse discovery result: all parsing tiers failed")
}

// buildTurnResults constructs a TurnResults struct from parsed discovery data
func buildTurnResults(rawJSON string, result discoveryResult) *TurnResult {
	return &TurnResult{
		Discovery: &DiscoveryResult{
			Candidates:                      result.Candidates,
			TotalFound:                      result.TotalFound,
			MissingFiles:                    result.MissingFiles,
			KeywordAssessment:               result.KeywordAssessment,
			DiscoveryLog:                    result.DiscoveryLog,
			Coverage:                        result.Coverage,
			DiscoveryMode:                   result.DiscoveryMode,
			BrainEffectiveness:              result.BrainEffectiveness,
			SuccinctNaturalLanguageResponse: result.SuccinctResponse,
		},
		RawJSON: rawJSON,
	}
}

// extractJSONFromMarkdown attempts to extract JSON from markdown fences
// using simple regex patterns. Returns empty string if extraction fails.
func extractJSONFromMarkdown(content string) string {
	content = strings.TrimSpace(content)
	
	// Pattern 1: ```json ... ```
	// Note: Using regular string literal with escaped backslashes
	reJSON := regexp.MustCompile("(?s)```json\\s*\\n(.*?)\\n\\s*```")
	if matches := reJSON.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Pattern 2: ``` ... ``` (no language tag)
	// Note: Using regular string literal with escaped backslashes
	reGeneric := regexp.MustCompile("(?s)```\\s*\\n(.*?)\\n\\s*```")
	if matches := reGeneric.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Pattern 3: Find JSON object in prose (first { to last })
	if start := strings.Index(content, "{"); start != -1 {
		if end := findMatchingBrace(content, start); end != -1 {
			return strings.TrimSpace(content[start : end+1])
		}
	}
	
	return ""
}

// findMatchingBrace finds the closing brace for an opening brace
// using a depth-counting algorithm to handle nested objects correctly.
func findMatchingBrace(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractJSONUsingMarkdownParser uses the markdown parser to extract
// JSON from code blocks. Returns empty string if extraction fails.
func extractJSONUsingMarkdownParser(content string) string {
	// Use the markdown parser to extract all code blocks
	parseResult, err := markdown.ExtractCodeBlocks(content, false)
	if err != nil {
		return ""
	}
	
	// Look for the first code block that contains valid JSON
	for _, block := range parseResult.Blocks {
		// Try to unmarshal as JSON to verify it's valid
		var jsontest interface{}
		if err := json.Unmarshal([]byte(block.ExecutableCode), &jsontest); err == nil {
			return block.ExecutableCode
		}
	}
	
	return ""
}

// ValidateDiscoveryResult checks a parsed TurnResults for semantic format errors
// that can occur even when JSON unmarshal succeeds. Returns a slice of
// human-readable error strings; returns nil when the result is valid.
func ValidateDiscoveryResult(result *TurnResult) []string {
	var errs []string

	// DEBUG: Force format errors for testing correction process
	// Set GSC_AGENT_FORCE_FORMAT_ERROR=true to trigger automatic correction
	// This actually corrupts the JSON so the correction turn has real errors to fix
	if os.Getenv("GSC_AGENT_FORCE_FORMAT_ERROR") == "true" {
		// Corrupt the first candidate's metadata arrays to strings
		if result.Discovery != nil && len(result.Discovery.Candidates) > 0 {
			if len(result.Discovery.Candidates[0].BrainMetadata.Keywords) > 0 {
				keywordsJSON, _ := json.Marshal(result.Discovery.Candidates[0].BrainMetadata.Keywords)
				result.Discovery.Candidates[0].BrainMetadata.Keywords = []string{string(keywordsJSON)}
			}
			// Convert parent_keywords array to JSON string
			if len(result.Discovery.Candidates[0].BrainMetadata.ParentKeywords) > 0 {
				parentKeywordsJSON, _ := json.Marshal(result.Discovery.Candidates[0].BrainMetadata.ParentKeywords)
				result.Discovery.Candidates[0].BrainMetadata.ParentKeywords = []string{string(parentKeywordsJSON)}
			}
		}
		// Corrupt keyword assessment matches to empty array
		if result.Discovery != nil && result.Discovery.KeywordAssessment != nil && result.Discovery.KeywordAssessment.Effectiveness != nil {
			for keyword, eff := range result.Discovery.KeywordAssessment.Effectiveness {
				eff.Matches = []string{} // Empty array
				result.Discovery.KeywordAssessment.Effectiveness[keyword] = eff
			}
		}
	}

	// Validate status
	if result.Discovery == nil {
		return []string{"missing required field discovery"}
	}
	
	// Note: Status is now in TurnState, not in TurnResult
	// We don't validate status here anymore

	// Validate discovery_mode and brain_effectiveness (hierarchical validation)
	if result.Discovery.DiscoveryMode == "" {
		errs = append(errs, "missing required field discovery_mode")
	} else if result.Discovery.DiscoveryMode != "experts" && result.Discovery.DiscoveryMode != "generic" {
		errs = append(errs, fmt.Sprintf("invalid discovery_mode %q (must be 'experts' or 'generic')", result.Discovery.DiscoveryMode))
	}

	// Hierarchical validation based on discovery_mode
	if result.Discovery.DiscoveryMode == "experts" {
		// In experts mode, brain_effectiveness must be present
		if result.Discovery.BrainEffectiveness == nil {
			errs = append(errs, "brain_effectiveness is required when discovery_mode is 'experts'")
		} else {
			// Validate brain_effectiveness structure
			if result.Discovery.BrainEffectiveness.OverallScore < 0.0 || result.Discovery.BrainEffectiveness.OverallScore > 1.0 {
				errs = append(errs, fmt.Sprintf("brain_effectiveness.overall_score %.4f is out of range [0.0, 1.0]", result.Discovery.BrainEffectiveness.OverallScore))
			}
			if len(result.Discovery.BrainEffectiveness.Brains) == 0 {
				errs = append(errs, "brain_effectiveness.brains must contain at least one brain entry")
			}
			// Validate each brain entry
			for i, brain := range result.Discovery.BrainEffectiveness.Brains {
				prefix := fmt.Sprintf("brain_effectiveness.brains[%d]", i)
				if brain.Name == "" {
					errs = append(errs, fmt.Sprintf("%s: missing required field name", prefix))
				}
				if brain.Score < 0.0 || brain.Score > 1.0 {
					errs = append(errs, fmt.Sprintf("%s: score %.4f is out of range [0.0, 1.0]", prefix, brain.Score))
				}
				if brain.Feedback == "" {
					errs = append(errs, fmt.Sprintf("%s: missing required field feedback", prefix))
				}
			}
		}
	} else if result.Discovery.DiscoveryMode == "generic" {
		// In generic mode, brain_effectiveness must be absent
		if result.Discovery.BrainEffectiveness != nil {
			errs = append(errs, "brain_effectiveness must be absent when discovery_mode is 'generic'")
		}
	}

	// Validate candidates
	for i, cand := range result.Discovery.Candidates {
		prefix := fmt.Sprintf("candidate[%d]", i)

		if cand.WorkdirID == 0 {
			errs = append(errs, fmt.Sprintf("%s: missing required field workdir_id", prefix))
		}
		if cand.FilePath == "" {
			errs = append(errs, fmt.Sprintf("%s: missing required field file_path", prefix))
		}
		if cand.Score < 0.0 || cand.Score > 1.0 {
			errs = append(errs, fmt.Sprintf(
				"%s: score %.4f is out of range [0.0, 1.0]",
				prefix, cand.Score,
			))
		}
		if cand.WorkdirName == "" {
			errs = append(errs, fmt.Sprintf("%s: missing required field workdir_name", prefix))
		}
		kw := cand.BrainMetadata.Keywords
		if len(kw) == 1 && len(kw[0]) > 2 && kw[0][0] == '[' && kw[0][len(kw[0])-1] == ']' {
			errs = append(errs, fmt.Sprintf("%s: metadata.keywords appears to be a string-encoded JSON array", prefix))
		}
		pkw := cand.BrainMetadata.ParentKeywords
		if len(pkw) == 1 && len(pkw[0]) > 2 && pkw[0][0] == '[' && pkw[0][len(pkw[0])-1] == ']' {
			errs = append(errs, fmt.Sprintf("%s: metadata.parent_keywords appears to be a string-encoded JSON array", prefix))
		}
	}

	// Validate keyword assessment effectiveness entries
	if ka := result.Discovery.KeywordAssessment; ka != nil {
		for keyword, eff := range ka.Effectiveness {
			prefix := fmt.Sprintf("keyword_assessment.effectiveness[%q]", keyword)

			switch eff.Rating {
			case "HIGH", "MEDIUM", "LOW":
				// valid
			case "":
				errs = append(errs, fmt.Sprintf("%s: missing required field rating", prefix))
			default:
				errs = append(errs, fmt.Sprintf(
					"%s: invalid rating %q (must be HIGH, MEDIUM, or LOW)",
					prefix, eff.Rating,
				))
			}

			if eff.Explanation == "" {
				errs = append(errs, fmt.Sprintf("%s: missing required field explanation", prefix))
			}
			if len(eff.Matches) == 0 {
				errs = append(errs, fmt.Sprintf(
					"%s: matches must be a non-empty array of file path strings",
					prefix,
				))
			}
		}
	}

	for i, mf := range result.Discovery.MissingFiles {
		prefix := fmt.Sprintf("missing_files[%d]", i)
		if mf.FilePath == "" {
			errs = append(errs, fmt.Sprintf("%s: missing required field file_path", prefix))
		}
		if mf.Score < 0.0 || mf.Score > 1.0 {
			errs = append(errs, fmt.Sprintf(
				"%s: score %.4f is out of range [0.0, 1.0]",
				prefix, mf.Score,
			))
		}
	}

	if result.RawJSON != "" {
		errs = append(errs, detectUnknownDiscoveryFields(result.RawJSON)...)
	}
	return errs
}

var knownDiscoveryTopLevelFields = map[string]struct{}{
	"status": {}, "candidates": {}, "missing_files": {},
	"keyword_assessment": {}, "duration": {}, "cost": {},
	"usage": {}, "total_found": {}, "coverage": {}, "discovery_log": {},
	"discovery_mode": {}, "brain_effectiveness": {},
	"succinct_natural_language_response": {},
}

func detectUnknownDiscoveryFields(rawJSON string) []string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return nil
	}
	var errs []string
	for key := range raw {
		if _, ok := knownDiscoveryTopLevelFields[key]; !ok {
			errs = append(errs, fmt.Sprintf(
				"unknown top-level field %q is not in the discovery schema",
				key,
			))
		}
	}
	return errs
}
