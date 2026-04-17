/**
 * Component: Validation Result Parser
 * Block-UUID: 4ecea211-1740-4666-b921-37955305c84b
 * Parent-UUID: 4a068996-bd7b-448b-bd68-1a090a2ec5f6
 * Version: 1.2.0
 * Description: Parses JSON results from Claude validation turns (both rich and legacy formats) into generic TurnResults. Fixed struct field access errors.
 * Language: Go
 * Created-at: 2026-04-17T16:51:19.898Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 2.5 Flash Lite (v1.2.0)
 */


package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// richValidationResult represents the new, detailed validation format
type richValidationResult struct {
	ValidationSummary struct {
		SessionIntent            string `json:"session_intent"`
		TurnNumber               int    `json:"turn_number"`
		TotalCandidatesReviewed  int    `json:"total_candidates_reviewed"`
		ValidatedCandidatesCount int    `json:"validated_candidates_count"`
		CriticalFinding          string `json:"critical_finding"`
	} `json:"validation_summary"`
	ValidatedCandidates       []RichValidatedCandidate  `json:"validated_candidates"`
	CriticalMissingCandidate  *CriticalMissingCandidate `json:"critical_missing_candidate"`
	KeywordAssessment         RichKeywordAssessment     `json:"keyword_assessment"`
	SummaryAndRecommendations SummaryAndRecommendations `json:"summary_and_recommendations"`
}

// oldValidationResult represents the legacy validation format
type oldValidationResult struct {
	ValidatedCandidates []ValidationUpdate `json:"validated_candidates"`
	ValidationSummary struct {
		TotalCandidates      int     `json:"total_candidates"`
		Validated            int     `json:"validated"`
		HighlyRelevant       int     `json:"highly_relevant"`
		Relevant             int     `json:"relevant"`
		TangentiallyRelevant int     `json:"tangentially_relevant"`
		CriticalFinding      string  `json:"critical_finding"`
	} `json:"validation_summary"`
	CriticalMissingFile struct {
		FilePath  string `json:"file_path"`
		Reason    string `json:"reason"`
		Evidence  string `json:"evidence"`
		Relevance string `json:"relevance"`
	} `json:"critical_missing_file"`
	KeywordEffectivenessAssessment struct {
		DiscoveryKeywords []string `json:"discovery_keywords"`
		Effectiveness    map[string]struct {
			Rating       string  `json:"rating"`
			Explanation  string  `json:"explanation"`
			Matches     []string `json:"matches"`
		} `json:"effectiveness"`
		NewKeywordsDiscovered []string `json:"new_keywords_discovered"`
		Recommendations struct {
			ForFutureDiscovery  []string `json:"for_future_discovery"`
			ImprovementActions  []string `json:"improvement_actions"`
		} `json:"recommendations"`
	} `json:"keyword_effectiveness_assessment"`
	SummaryAndConfidence struct {
		Confidence string `json:"confidence"`
		Conclusion string `json:"conclusion"`
	} `json:"summary_and_confidence"`
	Summary struct {
		TotalValidated        int     `json:"total_validated"`
		CandidatesPromoted    int     `json:"candidates_promoted"`
		CandidatesDemoted     int     `json:"candidates_demoted"`
		CandidatesRemoved     int     `json:"candidates_removed"`
		AverageValidatedScore float64 `json:"average_validated_score"`
		TopCandidatesCount    int     `json:"top_candidates_count"`
	} `json:"summary"`
}

// ParseValidationResult attempts to parse a JSON string as a validation result.
// It tries the rich format first, then falls back to the legacy format.
func ParseValidationResult(jsonContent string) (*TurnResults, error) {
	// Strip markdown code fences if present
	content := strings.TrimSpace(jsonContent)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSpace(content)
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// Try rich validation format first
	var richResult richValidationResult
	if err := json.Unmarshal([]byte(content), &richResult); err == nil && len(richResult.ValidatedCandidates) > 0 {
		return parseRichValidation(richResult)
	}

	// Try old validation format
	var oldResult oldValidationResult
	if err := json.Unmarshal([]byte(content), &oldResult); err == nil && len(oldResult.ValidatedCandidates) > 0 {
		return parseOldValidation(oldResult)
	}

	return nil, fmt.Errorf("failed to parse validation result (neither rich nor old format matched)")
}

// parseRichValidation converts the rich result struct to TurnResults
func parseRichValidation(result richValidationResult) (*TurnResults, error) {
	// Convert rich candidates to simple candidates
	validatedCandidates := make([]Candidate, len(result.ValidatedCandidates))
	for i, richCand := range result.ValidatedCandidates {
		validatedCandidates[i] = Candidate{
			FilePath:  richCand.FilePath,
			Score:     richCand.ValidatedScore,
			Reasoning: fmt.Sprintf("%s\n\n%s", richCand.Relevance, richCand.Reasoning),
		}
	}

	// Build validation log
	var validationLog *ValidationLog
	if result.ValidationSummary.TotalCandidatesReviewed > 0 {
		var missingFiles []MissingFile
		if result.CriticalMissingCandidate != nil {
			missingFiles = append(missingFiles, MissingFile{
				FilePath:  result.CriticalMissingCandidate.FilePath,
				Score:     result.CriticalMissingCandidate.Score,
				Reasoning: result.CriticalMissingCandidate.Reasoning,
				CodeValidation: &CodeValidation{
					ConfirmedPatterns:     []string{result.CriticalMissingCandidate.CodeValidation.ConfirmedPattern},
					ImplementationDetails: result.CriticalMissingCandidate.Relevance,
				},
			})
		}

		effectivenessMap := make(map[string]KeywordEffectiveness)
		for keyword, eff := range result.KeywordAssessment.KeywordEffectiveness {
			effectivenessMap[keyword] = KeywordEffectiveness{
				Rating:     eff.Effectiveness,
				Explanation: eff.Explanation,
				Matches:    eff.ExampleMatches,
			}
		}

		var allRecommendations []string
		allRecommendations = append(allRecommendations, result.KeywordAssessment.KeywordRecommendations.ShouldAdd...)
		allRecommendations = append(allRecommendations, result.KeywordAssessment.KeywordRecommendations.ShouldRefine...)
		allRecommendations = append(allRecommendations, result.KeywordAssessment.KeywordRecommendations.FutureDiscoveryStrategy)

		var discoveryReviewed []string
		for _, cand := range result.ValidatedCandidates {
			discoveryReviewed = append(discoveryReviewed, cand.FilePath)
		}

		var criticalFindings []string
		if result.ValidationSummary.CriticalFinding != "" {
			criticalFindings = append(criticalFindings, result.ValidationSummary.CriticalFinding)
		}
		if result.SummaryAndRecommendations.DiscoveryQualityAssessment != "" {
			criticalFindings = append(criticalFindings, result.SummaryAndRecommendations.DiscoveryQualityAssessment)
		}
		if result.SummaryAndRecommendations.Verdict != "" {
			criticalFindings = append(criticalFindings, fmt.Sprintf("Verdict: %s", result.SummaryAndRecommendations.Verdict))
		}

		var newKeywords []string
		for keyword := range result.KeywordAssessment.NewKeywordsDiscovered {
			newKeywords = append(newKeywords, keyword)
		}

		validationLog = &ValidationLog{
			DiscoveryReviewed:      discoveryReviewed,
			CriticalFindings:       criticalFindings,
			MissingFilesIdentified: missingFiles,
			KeywordAssessment: KeywordAssessment{
				DiscoveryKeywords: result.KeywordAssessment.DiscoveryIntentKeywords,
				Effectiveness:     effectivenessMap,
				NewKeywords:       newKeywords,
				Recommendations:   allRecommendations,
			},
			ValidationMethod: "Code inspection and semantic analysis of discovery candidates",
			TotalValidated:   result.ValidationSummary.ValidatedCandidatesCount,
			Confidence:       result.SummaryAndRecommendations.Verdict,
		}
	}

	// Build validation summary
	validationSummary := &ValidationSummary{
		SessionIntent:            result.ValidationSummary.SessionIntent,
		TurnNumber:               result.ValidationSummary.TurnNumber,
		TotalCandidatesReviewed:  result.ValidationSummary.TotalCandidatesReviewed,
		ValidatedCandidatesCount: result.ValidationSummary.ValidatedCandidatesCount,
		CriticalFinding:          result.ValidationSummary.CriticalFinding,
		TotalValidated:           result.ValidationSummary.ValidatedCandidatesCount,
		CandidatesPromoted:       0, // Calculated externally if needed
		CandidatesDemoted:        0, // Calculated externally if needed
		CandidatesRemoved:        0, // Calculated externally if needed
		AverageValidatedScore:    0.0, // Calculated externally if needed
		TopCandidatesCount:       len(validatedCandidates),
		ValidationLog:            validationLog,
	}

	return &TurnResults{
		Candidates:          validatedCandidates,
		ValidationSummary: validationSummary,
	}, nil
}

// parseOldValidation converts the legacy result struct to TurnResults
func parseOldValidation(result oldValidationResult) (*TurnResults, error) {
	// Convert updates to candidates (simple conversion, merging happens in stream layer)
	validatedCandidates := make([]Candidate, len(result.ValidatedCandidates))
	for i, update := range result.ValidatedCandidates {
		validatedCandidates[i] = Candidate{
			WorkdirID:   update.WorkdirID,
			FilePath:    update.FilePath,
			Score:       update.ValidatedScore,
			Reasoning:   update.Reason,
		}
	}

	// Build validation log
	var validationLog *ValidationLog
	if result.ValidationSummary.TotalCandidates > 0 {
		var missingFiles []MissingFile
		if result.CriticalMissingFile.FilePath != "" {
			missingFiles = append(missingFiles, MissingFile{
				FilePath:  result.CriticalMissingFile.FilePath,
				Reasoning: result.CriticalMissingFile.Reason,
				CodeValidation: &CodeValidation{
					ConfirmedPatterns:     []string{result.CriticalMissingFile.Evidence},
					ImplementationDetails: result.CriticalMissingFile.Relevance,
				},
			})
		}

		effectivenessMap := make(map[string]KeywordEffectiveness)
		for keyword, eff := range result.KeywordEffectivenessAssessment.Effectiveness {
			effectivenessMap[keyword] = KeywordEffectiveness{
				Rating:      eff.Rating,
				Explanation: eff.Explanation,
				Matches:     eff.Matches,
			}
		}

		var allRecommendations []string
		allRecommendations = append(allRecommendations, result.KeywordEffectivenessAssessment.Recommendations.ForFutureDiscovery...)
		allRecommendations = append(allRecommendations, result.KeywordEffectivenessAssessment.Recommendations.ImprovementActions...)

		var discoveryReviewed []string
		for _, cand := range result.ValidatedCandidates {
			discoveryReviewed = append(discoveryReviewed, cand.FilePath)
		}

		var criticalFindings []string
		if result.ValidationSummary.CriticalFinding != "" {
			criticalFindings = append(criticalFindings, result.ValidationSummary.CriticalFinding)
		}
		if result.SummaryAndConfidence.Conclusion != "" {
			criticalFindings = append(criticalFindings, result.SummaryAndConfidence.Conclusion)
		}

		validationLog = &ValidationLog{
			DiscoveryReviewed:      discoveryReviewed,
			CriticalFindings:       criticalFindings,
			MissingFilesIdentified: missingFiles,
			KeywordAssessment: KeywordAssessment{
				DiscoveryKeywords:  result.KeywordEffectivenessAssessment.DiscoveryKeywords,
				Effectiveness:      effectivenessMap,
				NewKeywords:        result.KeywordEffectivenessAssessment.NewKeywordsDiscovered,
				Recommendations:    allRecommendations,
			},
			ValidationMethod: "Code inspection and semantic analysis of discovery candidates",
			TotalValidated:   result.ValidationSummary.Validated,
			Confidence:       result.SummaryAndConfidence.Confidence,
		}
	}

	// Build validation summary
	validationSummary := &ValidationSummary{
		TotalValidated:        result.ValidationSummary.Validated,
		CandidatesPromoted:    result.ValidationSummary.HighlyRelevant,
		CandidatesDemoted:     result.ValidationSummary.Relevant,
		CandidatesRemoved:     result.ValidationSummary.TangentiallyRelevant,
		AverageValidatedScore: result.Summary.AverageValidatedScore,
		TopCandidatesCount:    result.Summary.TopCandidatesCount,
		ValidationLog:         validationLog,
	}

	return &TurnResults{
		Candidates:        validatedCandidates,
		ValidationSummary: validationSummary,
	}, nil
}
