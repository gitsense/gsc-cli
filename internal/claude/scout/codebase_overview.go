/**
 * Component: Scout Codebase Overview Builder
 * Block-UUID: fa9f0a36-3910-474a-85fc-ede58bfc5b3e
 * Parent-UUID: e1c65f76-f9f1-4a1e-b487-9168f9efdbf0
 * Version: 1.1.0
 * Description: Generates codebase overview for Scout Turn 1 by running gsc brains and gsc insights commands
 * Language: Go
 * Created-at: 2026-03-28T21:59:16.171Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0)
 */


package scout

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
)

// CodebaseOverview represents the complete codebase overview for Turn 1
type CodebaseOverview struct {
	Metadata           OverviewMetadata     `json:"metadata"`
	WorkingDirectories []WorkdirAnalysis `json:"working_directories"`
}

// OverviewMetadata contains metadata about the codebase overview
type OverviewMetadata struct {
	Version        string `json:"version"`
	CreatedAt      string `json:"created_at"`
	ScoutSessionID string `json:"scout_session_id"`
	GSCHome        string `json:"gsc_home"`
}

// WorkdirAnalysis contains analysis data for a working directory
type WorkdirAnalysis struct {
	ID                int                 `json:"id"`
	Name              string              `json:"name"`
	Path              string              `json:"path"`
	BrainAvailable    bool                `json:"brain_available"`
	Reason            string              `json:"reason,omitempty"`
	Remediation       string              `json:"remediation,omitempty"`
	AnalyzedFiles     *int                `json:"analyzed_files,omitempty"`
	TotalFilesInScope *int                `json:"total_files_in_scope,omitempty"`
	FileExtensions    *FileExtensionStats `json:"file_extensions,omitempty"`
	Insights          *InsightsData       `json:"insights,omitempty"`
}

// FileExtensionStats contains file extension statistics
type FileExtensionStats struct {
	All  []FileExtensionStat `json:"all"`
	Top5 []FileExtensionStat `json:"top_5"`
}

// FileExtensionStat represents a single file extension statistic
type FileExtensionStat struct {
	Value      string  `json:"value"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// InsightsData contains keyword insights
type InsightsData struct {
	Top20Keywords     []KeywordStat `json:"top_20_keywords"`
	AllParentKeywords []KeywordStat `json:"all_parent_keywords"`
}

// KeywordStat represents a single keyword statistic
type KeywordStat struct {
	Value      string  `json:"value"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// GSCBrainsResponse represents response from gsc brains command
type GSCBrainsResponse struct {
	DatabaseName string `json:"database_name"`
	Analyzers    []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"analyzers"`
}

// GSCInsightsResponse represents response from gsc insights command
type GSCInsightsResponse struct {
	Insights struct {
		FileExtension  []FileExtensionStat `json:"file_extension"`
		Keywords       []KeywordStat       `json:"keywords"`
		ParentKeywords []KeywordStat       `json:"parent_keywords"`
	} `json:"insights"`
	Summary struct {
		TotalFilesInScope int `json:"total_files_in_scope"`
		FilesWithMetadata struct {
			Keywords int `json:"keywords"`
		} `json:"files_with_metadata"`
	} `json:"summary"`
}

// BrainUnavailableError represents an error when brain is not available
type BrainUnavailableError struct {
	Reason  string
	Message string
}

func (e *BrainUnavailableError) Error() string {
	return e.Message
}

// BuildCodebaseOverview generates the codebase overview by running gsc commands for each workdir
func BuildCodebaseOverview(sessionID string, workdirs []WorkingDirectory) (*CodebaseOverview, error) {
	// Get GSC_HOME
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// Build overview
	overview := &CodebaseOverview{
		Metadata: OverviewMetadata{
			Version:        "1.0.0",
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			ScoutSessionID: sessionID,
			GSCHome:        gscHome,
		},
		WorkingDirectories: []WorkdirAnalysis{},
	}

	// Analyze each workdir
	for _, wd := range workdirs {
		analysis, err := analyzeWorkdir(wd)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze workdir %s: %w", wd.Name, err)
		}
		overview.WorkingDirectories = append(overview.WorkingDirectories, *analysis)
	}

	return overview, nil
}

// analyzeWorkdir analyzes a single working directory
func analyzeWorkdir(wd WorkingDirectory) (*WorkdirAnalysis, error) {
	analysis := &WorkdirAnalysis{
		ID:             wd.ID,
		Name:           wd.Name,
		Path:           wd.Path,
		BrainAvailable: false,
	}

	// Check brain availability
	_, err := runGSCBrains(wd.Path)
	if err != nil {
		// Brain not available
		if brainErr, ok := err.(*BrainUnavailableError); ok {
			analysis.Reason = brainErr.Reason
			analysis.Remediation = "Run: gsc manifest init"
		} else {
			analysis.Reason = "Failed to check brain availability"
			analysis.Remediation = err.Error()
		}
		return analysis, nil
	}

	// Brain is available, get insights
	analysis.BrainAvailable = true

	// Run insights for keywords and file extensions (limited to 20)
	insightsResponse, err := runGSCInsights(wd.Path, "keywords,file_extension", 20)
	if err != nil {
		return nil, fmt.Errorf("failed to get insights for %s: %w", wd.Name, err)
	}

	// Run insights for all parent keywords (no limit)
	parentKeywordsResponse, err := runGSCInsights(wd.Path, "parent_keywords", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent keywords for %s: %w", wd.Name, err)
	}

	// Extract data
	analyzedFiles := insightsResponse.Summary.FilesWithMetadata.Keywords
	totalFiles := insightsResponse.Summary.TotalFilesInScope
	analysis.AnalyzedFiles = &analyzedFiles
	analysis.TotalFilesInScope = &totalFiles

	// Build file extensions
	allExtensions := insightsResponse.Insights.FileExtension
	top5Extensions := extractTop5Extensions(allExtensions)
	analysis.FileExtensions = &FileExtensionStats{
		All:  allExtensions,
		Top5: top5Extensions,
	}

	// Build insights
	analysis.Insights = &InsightsData{
		Top20Keywords:     insightsResponse.Insights.Keywords,
		AllParentKeywords: parentKeywordsResponse.Insights.ParentKeywords,
	}

	return analysis, nil
}

// runGSCBrains runs gsc brains command to check brain availability
func runGSCBrains(workdirPath string) (*GSCBrainsResponse, error) {
	cmd := exec.Command("gsc", "brains", "tiny-overview", "--format", "json")
	cmd.Dir = workdirPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Parse error message to determine reason
		errMsg := string(output)
		reason := "Unknown error"

		if strings.Contains(errMsg, "GitSense Chat can only be used within a Git repository") {
			reason = "Not a Git repository"
		} else if strings.Contains(errMsg, "GitSense workspace not found") {
			reason = "GitSense workspace not initialized"
		} else if strings.Contains(errMsg, "Brain 'tiny-overview' not found") {
			reason = "Tiny Overview brain not found"
		}

		return nil, &BrainUnavailableError{
			Reason:  reason,
			Message: errMsg,
		}
	}

	// Parse JSON response
	var response GSCBrainsResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse brains response: %w", err)
	}

	return &response, nil
}

// runGSCInsights runs gsc insights command to get insights data
func runGSCInsights(workdirPath string, fields string, limit int) (*GSCInsightsResponse, error) {
	args := []string{
		"insights",
		"--db", "tiny-overview",
		"--fields", fields,
		"--format", "json",
	}

	if limit > 0 {
		args = append(args, "--limit", fmt.Sprintf("%d", limit))
	}

	cmd := exec.Command("gsc", args...)
	cmd.Dir = workdirPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gsc insights failed: %w, output: %s", err, string(output))
	}

	// Parse JSON response
	var response GSCInsightsResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse insights response: %w", err)
	}

	return &response, nil
}

// extractTop5Extensions returns the top 5 file extensions
func extractTop5Extensions(allExtensions []FileExtensionStat) []FileExtensionStat {
	if len(allExtensions) <= 5 {
		return allExtensions
	}
	return allExtensions[:5]
}

// checkAllBrainsUnavailable checks if all working directories have unavailable brains
func checkAllBrainsUnavailable(analyses []WorkdirAnalysis) bool {
	for _, analysis := range analyses {
		if analysis.BrainAvailable {
			return false
		}
	}
	return true
}
