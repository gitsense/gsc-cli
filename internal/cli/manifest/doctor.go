/*
 * Component: Doctor Command
 * Block-UUID: c8efe193-47fa-450e-953c-d44b57b5185f
 * Parent-UUID: ccd42669-9980-4b46-b923-59e27102570b
 * Version: 1.1.0
 * Description: CLI command for running health checks on the .gitsense environment and databases. Removed unused getter function.
 * Language: Go
 * Created-at: 2026-02-02T07:58:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0)
 */


package manifest

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var doctorFix bool
var doctorVerbose bool

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on the .gitsense environment",
	Long: `Run health checks on the .gitsense environment to diagnose issues with 
the directory structure, registry file, and database connectivity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Running health checks...")

		// Call the logic layer to run diagnostics
		report, err := manifest.RunDoctor(cmd.Context(), doctorFix)
		if err != nil {
			logger.Error("Doctor check failed: %v", err)
			return err
		}

		// Format and output the results
		printDoctorReport(report, doctorVerbose)

		if !report.IsHealthy {
			// Return an error code if issues were found
			return fmt.Errorf("health checks failed")
		}

		return nil
	},
}

func init() {
	// Add flags
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to automatically fix issues (experimental)")
	doctorCmd.Flags().BoolVarP(&doctorVerbose, "verbose", "v", false, "Show detailed output for all checks")
}

// printDoctorReport formats and prints the doctor report
func printDoctorReport(report *manifest.DoctorReport, verbose bool) {
	if report.IsHealthy {
		logger.Success("All health checks passed.")
	} else {
		logger.Error("Health checks failed. See details below.")
	}

	fmt.Println()

	// Print individual checks
	for _, check := range report.Checks {
		// In non-verbose mode, only show warnings and errors
		if !verbose && check.Status == "ok" {
			continue
		}

		var statusIcon string
		var statusColor string

		switch check.Status {
		case "ok":
			statusIcon = "✓"
			statusColor = "\033[32m" // Green
		case "warning":
			statusIcon = "⚠"
			statusColor = "\033[33m" // Yellow
		case "error":
			statusIcon = "✗"
			statusColor = "\033[31m" // Red
		default:
			statusIcon = "?"
			statusColor = "\033[0m" // Reset
		}

		fmt.Printf("%s%s %s\033[0m %s\n", statusColor, statusIcon, check.Name, check.Message)
	}
}
