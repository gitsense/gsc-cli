/**
 * Component: Verification Result Parser
 * Block-UUID: 4a068996-bd7b-448b-bd68-1a090a2ec5f6
 * Parent-UUID: 01198ac0-d534-4ede-8f4e-0f23b7cffdfe
 * Version: 1.1.0
 * Description: Parses JSON results from Claude verification turns (both rich and legacy formats) into generic TurnResults. Fixed struct field access errors.
 * Language: Go
 * Created-at: 2026-04-15T16:10:27.942Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// richVerificationResult represents the new, detailed verification format
type richVerificationResult struct {
	VerificationSummary struct {
		SessionIntent           string `json:"session_intent"`
		TurnNumber              int    `json:"turn_number"`
		TotalCandidatesReviewed int    `json:"total_candidates_reviewed"`
		VerifiedCandidatesCount int    `json:"verified_candidates_count"`
		CriticalFinding         string `json:"critical_finding"`
	} `json:"verification_summary"`
	VerifiedCandidates       []RichVerifiedCandidate   `json:"verified_candidates"`
	CriticalMissingCandidate *CriticalMissingCandidate `json:"critical_missing_candidate"`
	KeywordAssessment       RichKeywordAssessment     `json:"keyword_assessment"`
	SummaryAndRecommendations SummaryAndRecommendations `json:"summary_and_recommendations"`
}

// oldVerificationResult represents the legacy verification format
type oldVerificationResult struct {
	VerifiedCandidates []VerificationUpdate `json:"verified_candidates"`
	VerificationSummary struct {
		TotalCandidates      int     `json:"total_candidates"`
		Verified             int     `json:"verified"`
		HighlyRelevant       int     `json:"highly_relevant"`
		Relevant             int     `json:"relevant"`
		TangentiallyRelevant int     `json:"tangentially_relevant"`
		CriticalFinding      string  `json:"critical_finding"`
	} `json:"verification_summary"`
	CriticalMissingFile struct {
		FilePath  string `json:"file_path"`
		Reason    string `json:"reason"`
		Evidence  string `json:"evidence"`
		Relevance string `json:"relevance"`
	} `json:"critical_missing_file"`
	KeywordEffectivenessAssessment struct {
		DiscoveryKeywords []string `json:"discovery_keywords"`
		Effectiveness     map[string]struct {
			Rating     string   `json:"rating"`
			Explanation string  `json:"explanation"`
			Matches    []string `json:"matches"`
		} `json:"effectiveness"`
		NewKeywordsDiscovered []string `json:"new_keywords_discovered"`
		Recommendations       struct {
			ForFutureDiscovery []string `json:"for_future_discovery"`
			ImprovementActions  []string `json:"improvement_actions"`
		} `json:"recommendations"`
	} `json:"keyword_effectiveness_assessment"`
	SummaryAndConfidence struct {
		Confidence string `json:"confidence"`
		Conclusion string `json:"conclusion"`
	} `json:"summary_and_confidence"`
	Summary struct {
		TotalVerified        int     `json:"total_verified"`
		CandidatesPromoted   int     `json:"candidates_promoted"`
		CandidatesDemoted    int     `json:"candidates_demoted"`
		CandidatesRemoved    int     `json:"candidates_removed"`
		AverageVerifiedScore float64 `json:"average_verified_score"`
		TopCandidatesCount   int     `json:"top_candidates_count"`
	} `json:"summary"`
}

// ParseVerificationResult attempts to parse a JSON string as a verification result.
// It tries the rich format first, then falls back to the legacy format.
func ParseVerificationResult(jsonContent string) (*TurnResults, error) {
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

	// Try rich verification format first
	var richResult richVerificationResult
	if err := json.Unmarshal([]byte(content), &richResult); err == nil && len(richResult.VerifiedCandidates) > 0 {
		return parseRichVerification(richResult)
	}

	// Try old verification format
	var oldResult oldVerificationResult
	if err := json.Unmarshal([]byte(content), &oldResult); err == nil && len(oldResult.VerifiedCandidates) > 0 {
		return parseOldVerification(oldResult)
	}

	return nil, fmt.Errorf("failed to parse verification result (neither rich nor old format matched)")
}

// parseRichVerification converts the rich result struct to TurnResults
func parseRichVerification(result richVerificationResult) (*TurnResults, error) {
	// Convert rich candidates to simple candidates
	verifiedCandidates := make([]Candidate, len(result.VerifiedCandidates))
	for i, richCand := range result.VerifiedCandidates {
		verifiedCandidates[i] = Candidate{
			FilePath:  richCand.FilePath,
			Score:     richCand.VerifiedScore,
			Reasoning: fmt.Sprintf("%s\n\n%s", richCand.Relevance, richCand.Reasoning),
		}
	}

	// Build verification log
	var verificationLog *VerificationLog
	if result.VerificationSummary.TotalCandidatesReviewed > 0 {
		var missingFiles []MissingFile
		if result.CriticalMissingCandidate != nil {
			missingFiles = append(missingFiles, MissingFile{
				FilePath:  result.CriticalMissingCandidate.FilePath,
				Reason:    result.CriticalMissingCandidate.Reasoning,
				Evidence:  result.CriticalMissingCandidate.CodeVerification.ConfirmedPattern,
				Relevance: result.CriticalMissingCandidate.Relevance,
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
		for _, cand := range result.VerifiedCandidates {
			discoveryReviewed = append(discoveryReviewed, cand.FilePath)
		}

		var criticalFindings []string
		if result.VerificationSummary.CriticalFinding != "" {
			criticalFindings = append(criticalFindings, result.VerificationSummary.CriticalFinding)
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

		verificationLog = &VerificationLog{
			DiscoveryReviewed:      discoveryReviewed,
			CriticalFindings:       criticalFindings,
			MissingFilesIdentified: missingFiles,
			KeywordAssessment: KeywordAssessment{
				DiscoveryKeywords: result.KeywordAssessment.DiscoveryIntentKeywords,
				Effectiveness:     effectivenessMap,
				NewKeywords:       newKeywords,
				Recommendations:   allRecommendations,
			},
			VerificationMethod: "Code inspection and semantic analysis of discovery candidates",
			TotalVerified:      result.VerificationSummary.VerifiedCandidatesCount,
			Confidence:         result.SummaryAndRecommendations.Verdict,
		}
	}

	// Build verification summary
	verificationSummary := &VerificationSummary{
		SessionIntent:           result.VerificationSummary.SessionIntent,
		TurnNumber:              result.VerificationSummary.TurnNumber,
		TotalCandidatesReviewed: result.VerificationSummary.TotalCandidatesReviewed,
		VerifiedCandidatesCount: result.VerificationSummary.VerifiedCandidatesCount,
		CriticalFinding:         result.VerificationSummary.CriticalFinding,
		TotalVerified:           result.VerificationSummary.VerifiedCandidatesCount,
		CandidatesPromoted:      0, // Calculated externally if needed
		CandidatesDemoted:       0, // Calculated externally if needed
		CandidatesRemoved:       0, // Calculated externally if needed
		AverageVerifiedScore:    0.0, // Calculated externally if needed
		TopCandidatesCount:      len(verifiedCandidates),
		VerificationLog:         verificationLog,
	}

	return &TurnResults{
		Candidates:          verifiedCandidates,
		VerificationSummary: verificationSummary,
	}, nil
}

// parseOldVerification converts the legacy result struct to TurnResults
func parseOldVerification(result oldVerificationResult) (*TurnResults, error) {
	// Convert updates to candidates (simple conversion, merging happens in stream layer)
	verifiedCandidates := make([]Candidate, len(result.VerifiedCandidates))
	for i, update := range result.VerifiedCandidates {
		verifiedCandidates[i] = Candidate{
			WorkdirID:   update.WorkdirID,
			FilePath:    update.FilePath,
			Score:       update.VerifiedScore,
			Reasoning:   update.Reason,
		}
	}

	// Build verification log
	var verificationLog *VerificationLog
	if result.VerificationSummary.TotalCandidates > 0 {
		var missingFiles []MissingFile
		if result.CriticalMissingFile.FilePath != "" {
			missingFiles = append(missingFiles, MissingFile{
				FilePath:  result.CriticalMissingFile.FilePath,
				Reason:    result.CriticalMissingFile.Reason,
				Evidence:  result.CriticalMissingFile.Evidence,
				Relevance: result.CriticalMissingFile.Relevance,
			})
		}

		effectivenessMap := make(map[string]KeywordEffectiveness)
		for keyword, eff := range result.KeywordEffectivenessAssessment.Effectiveness {
			effectivenessMap[keyword] = KeywordEffectiveness{
				Rating:     eff.Rating,
				Explanation: eff.Explanation,
				Matches:    eff.Matches,
			}
		}

		var allRecommendations []string
		allRecommendations = append(allRecommendations, result.KeywordEffectivenessAssessment.Recommendations.ForFutureDiscovery...)
		allRecommendations = append(allRecommendations, result.KeywordEffectivenessAssessment.Recommendations.ImprovementActions...)

		var discoveryReviewed []string
		for _, cand := range result.VerifiedCandidates {
			discoveryReviewed = append(discoveryReviewed, cand.FilePath)
		}

		var criticalFindings []string
		if result.VerificationSummary.CriticalFinding != "" {
			criticalFindings = append(criticalFindings, result.VerificationSummary.CriticalFinding)
		}
		if result.SummaryAndConfidence.Conclusion != "" {
			criticalFindings = append(criticalFindings, result.SummaryAndConfidence.Conclusion)
		}

		verificationLog = &VerificationLog{
			DiscoveryReviewed:      discoveryReviewed,
			CriticalFindings:       criticalFindings,
			MissingFilesIdentified: missingFiles,
			KeywordAssessment: KeywordAssessment{
				DiscoveryKeywords: result.KeywordEffectivenessAssessment.DiscoveryKeywords,
				Effectiveness:     effectivenessMap,
				NewKeywords:       result.KeywordEffectivenessAssessment.NewKeywordsDiscovered,
				Recommendations:   allRecommendations,
			},
			VerificationMethod: "Code inspection and semantic analysis of discovery candidates",
			TotalVerified:      result.VerificationSummary.Verified,
			Confidence:         result.SummaryAndConfidence.Confidence,
		}
	}

	// Build verification summary
	verificationSummary := &VerificationSummary{
		TotalVerified:        result.VerificationSummary.Verified,
		CandidatesPromoted:   result.VerificationSummary.HighlyRelevant,
		CandidatesDemoted:    result.VerificationSummary.Relevant,
		CandidatesRemoved:    result.VerificationSummary.TangentiallyRelevant,
		AverageVerifiedScore: result.Summary.AverageVerifiedScore,
		TopCandidatesCount:   result.Summary.TopCandidatesCount,
		VerificationLog:      verificationLog,
	}

	return &TurnResults{
		Candidates:          verifiedCandidates,
		VerificationSummary: verificationSummary,
	}, nil
}
