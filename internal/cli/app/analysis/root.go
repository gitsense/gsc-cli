/**
 * Component: Analysis Command Root
 * Block-UUID: df0719b7-c5bd-43c5-95bc-36e4e465a4b3
 * Parent-UUID: f0dde8b1-8952-4e4b-90bd-335c94bec087
 * Version: 1.7.0
 * Description: Defines the root command for the 'gsc app analysis' group, managing AI-generated metadata, analysis results, and export/import operations. v1.2.0: Registered the new 'load' command to restore analysis data from JSONL files. v1.3.0: Hid the 'copy' command to reduce maintenance burden. v1.4.0: Removed duplicate command listing from help text. v1.5.0: Unhid the 'copy' command after rewriting it to use dump-then-load approach. v1.6.0: Registered 'get' and 'set' subcommands for the consumer API interface.
 * Language: Go
 * Created-at: 2026-06-10T03:09:43.580Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), MiMo-v2.5-Pro (v1.6.0), GLM-4.7 (v1.7.0)
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
	AnalysisCmd.AddCommand(GetCmd)
	AnalysisCmd.AddCommand(LoadCmd)
	AnalysisCmd.AddCommand(SetCmd)
	AnalysisCmd.AddCommand(StatusCmd)

	// Register analysis group to parent
	parent.AddCommand(AnalysisCmd)
}
