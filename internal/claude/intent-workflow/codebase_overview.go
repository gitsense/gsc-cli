/**
 * Component: Intent Workflow Codebase Overview Builder
 * Block-UUID: 2e4dd87b-2240-4475-b1c4-008a7db5ce33
 * Parent-UUID: 3cd87c64-7555-497f-82f0-2a1514db73fe
 * Version: 2.2.0
 * Description: Generates codebase overview by checking for experts-context.md file. Removed brain database checks and insights gathering. Simplified to only detect experts mode capability. Added disableExperts parameter to BuildCodebaseOverview and analyzeWorkdir to support --disable-experts flag for forcing generic discovery mode.
 * Language: Go
 * Created-at: 2026-05-02T16:27:05.573Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.1.0), GLM-4.7 (v2.2.0)
 */


package intent_workflow

import (
	"fmt"
	"os"
	"time"

	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/gitsense/gsc-cli/internal/git"
)

// CodebaseOverview represents the complete codebase overview
type CodebaseOverview struct {
	Metadata           OverviewMetadata     `json:"metadata"`
	WorkingDirectories []WorkdirAnalysis `json:"working_directories"`
}

// OverviewMetadata contains metadata about the codebase overview
type OverviewMetadata struct {
	Version        string `json:"version"`
	CreatedAt      string `json:"created_at"`
	AgentSessionID string `json:"agent_session_id"`
	GSCHome        string `json:"gsc_home"`
}

// WorkdirAnalysis contains analysis data for a working directory
type WorkdirAnalysis struct {
	ID                int                 `json:"id"`
	Name              string              `json:"name"`
	Path              string              `json:"path"`
	ExpertsEnabled    bool                `json:"experts_enabled"`
	ExpertsContextPath string             `json:"experts_context_path,omitempty"`
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
	Top50Keywords     []KeywordStat `json:"top_50_keywords"`
	AllParentKeywords []KeywordStat `json:"all_parent_keywords"`
}

// KeywordStat represents a single keyword statistic
type KeywordStat struct {
	Value      string  `json:"value"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// BuildCodebaseOverview generates the codebase overview by checking for experts context
func BuildCodebaseOverview(sessionID string, workdirs []WorkingDirectory, disableExperts bool) (*CodebaseOverview, error) {
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
			AgentSessionID: sessionID,
			GSCHome:        gscHome,
		},
		WorkingDirectories: []WorkdirAnalysis{},
	}

	// Analyze each workdir
	for _, wd := range workdirs {
		analysis, err := analyzeWorkdir(wd, disableExperts)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze workdir %s: %w", wd.Name, err)
		}
		overview.WorkingDirectories = append(overview.WorkingDirectories, *analysis)
	}

	return overview, nil
}

// analyzeWorkdir analyzes a single working directory
func analyzeWorkdir(wd WorkingDirectory, disableExperts bool) (*WorkdirAnalysis, error) {
	analysis := &WorkdirAnalysis{
		ID:             wd.ID,
		Name:           wd.Name,
		Path:           wd.Path,
		ExpertsEnabled: false,
	}

	// Find git root using internal/git package
	gitRoot, err := git.FindGitRootFrom(wd.Path)
	if err != nil {
		// Not a git repository - return analysis with experts disabled
		return analysis, nil
	}

	// Check for experts-context.md only when experts mode is not disabled
	if !disableExperts {
		expertsContextPath := fmt.Sprintf("%s/.gitsense/experts-context.md", gitRoot)
		if _, err := os.Stat(expertsContextPath); err == nil {
			analysis.ExpertsEnabled = true
			analysis.ExpertsContextPath = expertsContextPath
		}
	}

	return analysis, nil
}
