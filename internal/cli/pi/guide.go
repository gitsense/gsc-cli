/**
 * Component: Pi Guide Command
 * Block-UUID: b2c3d4e5-f6a7-8901-bcde-f23456789012
 * Parent-UUID: d8b3f5a1-2e74-4c69-8f30-1a7d9c4e6b25
 * Version: 1.0.0
 * Description: Implements the 'gsc pi guide' command. Loads the GSC_PI_GUIDE.md template and prints it to stdout, providing detailed documentation for the gsc pi command group.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package pi

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gitsense/gsc-cli/internal/experts"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

// NewGuideCmd creates and returns the 'gsc pi guide' command.
func NewGuideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Print detailed reference documentation for gsc pi",
		Long: `Prints a comprehensive reference guide covering the gsc pi command group,
including the interactive resume picker, session statistics, HUD mode,
session sync, querying, and common workflows.

The guide is designed for both human and agent consumption.`,
		SilenceUsage: true,
		RunE:         runPiGuide,
	}
	return cmd
}

func runPiGuide(cmd *cobra.Command, args []string) error {
	content, err := loadPiGuideTemplate()
	if err != nil {
		return fmt.Errorf("failed to load pi guide: %w", err)
	}

	// Strip provenance headers for cleaner output
	output := experts.StripProvenanceHeaders(string(content))
	fmt.Print(output)
	return nil
}

// loadPiGuideTemplate loads the GSC_PI_GUIDE.md template using the
// "Local First, Embedded Fallback" strategy: prefer a live copy under
// $GSC_HOME/cli/templates/experts, otherwise fall back to the embedded copy.
func loadPiGuideTemplate() ([]byte, error) {
	// Try local $GSC_HOME first
	gscHome, err := settings.GetGSCHome(false)
	if err == nil {
		localPath := filepath.Join(gscHome, "cli", "templates", "experts", "GSC_PI_GUIDE.md")
		content, err := os.ReadFile(localPath)
		if err == nil {
			return content, nil
		}
	}

	// Fallback to embedded filesystem
	return settings.TemplateFS.ReadFile("templates/experts/GSC_PI_GUIDE.md")
}
