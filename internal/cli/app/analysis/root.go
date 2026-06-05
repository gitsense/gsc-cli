/**
 * Component: Analysis Command Root
 * Block-UUID: 94bd4a02-6c5b-491b-aa87-87c505ce7658
 * Parent-UUID: ceb2ccd8-71b9-49eb-ac60-854a4aee2930
 * Version: 1.5.0
 * Description: Defines the root command for the 'gsc app analysis' group, managing AI-generated metadata, analysis results, and export/import operations. v1.2.0: Registered the new 'load' command to restore analysis data from JSONL files. v1.3.0: Hid the 'copy' command to reduce maintenance burden. v1.4.0: Removed duplicate command listing from help text. v1.5.0: Unhid the 'copy' command after rewriting it to use dump-then-load approach.
 * Language: Go
 * Created-at: 2026-05-17T02:26:46.495Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package analysis

import (
	"github.com/spf13/cobra"
)

// AnalysisCmd represents the analysis command group
var AnalysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "Manage application analysis data",
	Long:  "The analysis command provides tools for managing analyzer metadata and analysis results within the GitSense Chat application.",
}

// RegisterCommand adds the analysis command group to the parent command
func RegisterCommand(parent *cobra.Command) {
	// Register subcommands
	AnalysisCmd.AddCommand(CopyCmd)
	AnalysisCmd.AddCommand(DumpCmd)
	AnalysisCmd.AddCommand(LoadCmd)

	// Register analysis group to parent
	parent.AddCommand(AnalysisCmd)
}
