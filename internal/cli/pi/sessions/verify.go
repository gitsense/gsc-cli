/**
 * Component: Pi Sessions Verify Command
 * Block-UUID: 9b3c4d5e-6f7a-8b9c-0d1e-2f3a4b5c6d7e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc pi sessions verify --lossless for byte-for-byte round-trip verification.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package sessions

import (
	"encoding/json"
	"fmt"
	"os"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

func verifyCmd() *cobra.Command {
	var lossless bool
	var sessionID string
	var dbPath string
	var format string

	cmd := &cobra.Command{
		Use:          "verify",
		Short:        "Verify Pi session import fidelity",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !lossless {
				return fmt.Errorf("--lossless is required for phase 1")
			}
			if sessionID == "" {
				return fmt.Errorf("--session-id is required")
			}
			resolvedDB, err := resolvePiSessionsDBPath(dbPath)
			if err != nil {
				return err
			}
			report, err := pisessions.VerifyLosslessWithDB(cmd.Context(), resolvedDB, sessionID)
			if err != nil {
				return err
			}
			return writeVerifyReport(report, format)
		},
	}
	cmd.Flags().BoolVar(&lossless, "lossless", false, "Verify byte-for-byte round-trip fidelity")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session UUID to verify")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi/pi-sessions.sqlite3)")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	return cmd
}

func writeVerifyReport(report *pisessions.LosslessReport, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case "human", "":
		if report.SourceMissing {
			fmt.Printf("Lossless verification: SOURCE MISSING\n")
			fmt.Printf("  Session ID: %s\n", report.SessionID)
			fmt.Printf("  Source: %s\n", report.SessionFile)
			return nil
		}
		if report.Error != "" {
			fmt.Printf("Lossless verification: ERROR\n")
			fmt.Printf("  Session ID: %s\n", report.SessionID)
			fmt.Printf("  Error: %s\n", report.Error)
			return nil
		}
		status := "FAIL"
		if report.Match {
			status = "PASS"
		}
		fmt.Printf("Lossless verification: %s\n", status)
		fmt.Printf("  Session ID: %s\n", report.SessionID)
		fmt.Printf("  Source: %s\n", report.SessionFile)
		fmt.Printf("  Bytes: %d\n", report.SourceBytes)
		fmt.Printf("  SHA-256: %s\n", report.SourceSHA256)
		if !report.Match {
			fmt.Printf("  Reconstructed bytes: %d\n", report.ReconstructedBytes)
			fmt.Printf("  Reconstructed SHA-256: %s\n", report.ReconstructedSHA256)
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
