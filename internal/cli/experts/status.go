/**
 * Component: Experts Status Command
 * Block-UUID: f7a34c6a-2efb-424f-bb1b-574b4c11766c
 * Parent-UUID: cb567a91-a62f-45b8-9482-459b4de847bb
 * Version: 1.0.3
 * Description: Updated template paths from data/templates to cli/templates to separate CLI-specific data from app-specific data.
 * Language: Go
 * Created-at: 2026-05-01T23:43:28.541Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1), Gemini 2.5 Flash Lite (v1.0.2), GLM-4.7 (v1.0.3)
 */


package experts

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/registry"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

// NewStatusCmd creates and returns the 'gsc experts status' command.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the status and staleness of the expert context",
		Long: `Analyzes the 'experts-context.md' file to determine if it is up-to-date
with the current Brain data and system templates.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	repoRoot, err := git.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	contextPath := experts.ContextFilePath(repoRoot)
	info, err := os.Stat(contextPath)
	if os.IsNotExist(err) {
		fmt.Println("❌ Expert context not initialized. Run 'gsc experts init'.")
		return nil
	}
	contextTime := info.ModTime()

	// 1. Check Registry/Brains Staleness
	reg, _ := registry.LoadRegistry()
	isStale := false
	stalenessReason := ""

	for _, entry := range reg.Databases {
		if entry.UpdatedAt.After(contextTime) {
			isStale = true
			stalenessReason = fmt.Sprintf("Brain '%s' was updated on %s", entry.DatabaseName, entry.UpdatedAt.Format(time.RFC822))
			break
		}
	}

	// 2. Check Template Staleness (Local only)
	if !isStale {
		gscHome, err := settings.GetGSCHome(false)
		if err == nil {
			templatePath := filepath.Join(gscHome, "cli", "templates", "experts", "GSC_EXPERTS_SYSTEM_PROMPT.md")
			if tInfo, err := os.Stat(templatePath); err == nil {
				if tInfo.ModTime().After(contextTime) {
					isStale = true
					stalenessReason = "System templates in $GSC_HOME were updated"
				}
			}
		}
	}

	// 3. Render Status Table
	fmt.Printf("Expert Context: %s\n", contextPath)
	fmt.Printf("Generated:      %s\n\n", contextTime.Format(time.RFC822))

	statusStr := "✅ Current"
	notes := "-"
	if isStale {
		statusStr = "⚠️  Stale"
		notes = stalenessReason
	}

	headers := []string{"Component", "Status", "Notes"}
	rows := [][]string{{"Expertise", statusStr, notes}}
	fmt.Print(output.FormatTable(headers, rows))

	if isStale {
		fmt.Println("\nRun 'gsc experts init --force' to refresh the context.")
	}

	return nil
}
