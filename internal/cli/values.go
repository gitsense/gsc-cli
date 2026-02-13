/**
 * Component: Values Command
 * Block-UUID: f87f9ef9-9b1c-4b21-bd17-56f8922c0b3d
 * Parent-UUID: 3497ae90-8770-4551-988c-6a1663c87196
 * Version: 1.1.0
 * Description: Provides a top-level shortcut for listing unique metadata values. This command maps directly to 'gsc query list --db <db> <field>'.
 * Language: Go
 * Created-at: 2026-02-12T05:18:55.857Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge"
)

var (
	valuesFormat string
	valuesQuiet  bool
)

// valuesCmd represents the values shortcut command
var valuesCmd = &cobra.Command{
	Use:   "values <database> <field>",
	Short: "List unique values for a specific metadata field",
	Long: `A shortcut command to discover unique values within a database.
This is equivalent to running 'gsc query list --db <database> <field>'.`,
	Example: `  # List all risk levels in the security database
  gsc values security risk_level

  # List all topics in the lead-architect database
  gsc values lead-architect topics`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		startTime := time.Now()

		// Show help if no arguments provided
		if len(args) == 0 {
			cmd.Help()
			return nil
		}

		// Validate argument count
		if len(args) != 2 {
			return fmt.Errorf("requires exactly 2 arguments: <database> and <field>")
		}

		dbName := args[0]
		fieldName := args[1]

		// Early Validation for Bridge
		if bridgeCode != "" {
			if err := bridge.ValidateCode(bridgeCode, bridge.StageDiscovery); err != nil {
				cmd.SilenceUsage = true
				return err
			}
		}

		// Delegate to the hierarchical list handler in query.go
		// We pass all=false because we are targeting a specific field
		outputStr, resolvedDB, err := handleHierarchicalList(cmd.Context(), dbName, fieldName, valuesFormat, valuesQuiet, false)
		if err != nil {
			return err
		}

		if bridgeCode != "" {
			// 1. Print to stdout
			fmt.Print(outputStr)

			// 2. Hand off to bridge orchestrator
			cmdStr := filepath.Base(os.Args[0]) + " " + strings.Join(os.Args[1:], " ")
			return bridge.Execute(bridgeCode, outputStr, valuesFormat, cmdStr, time.Since(startTime), resolvedDB, forceInsert)
		}

		// Standard Output Mode
		fmt.Println(outputStr)
		return nil
	},
	SilenceUsage: false,
}

func init() {
	valuesCmd.Flags().StringVarP(&valuesFormat, "format", "o", "table", "Output format (json, table)")
	valuesCmd.Flags().BoolVar(&valuesQuiet, "quiet", false, "Suppress headers and hints")
}
