/**
 * Component: Contract CLI Directories
 * Block-UUID: 7c2f8bba-461a-4d33-b49c-8f3b0a1e9c27
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI commands for managing working directories within a contract (add, list, remove, set-primary).
 * Language: Go
 * Created-at: 2026-03-26T21:28:00.000Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
)

// ==========================================
// Global Flags
// ==========================================

var (
	dirPath   string
	dirName   string
	dirUUID   string
	dirFormat string
)

// ==========================================
// Commands
// ==========================================

// addWorkdirCmd handles 'gsc contract add-workdir'
var addWorkdirCmd = &cobra.Command{
	Use:   "add-workdir --path <path> [--name <name>]",
	Short: "Add a secondary working directory to the contract",
	Long: `Adds a new working directory to the current contract.
The path must be a valid Git repository. If no name is provided,
the directory's basename will be used.`,
	Example: `  # Add a directory with a custom name
  gsc contract add-workdir --path ../shared-utils --name "shared-utils"

  # Add a directory (defaults name to folder name)
  gsc contract add-workdir --path /opt/libs/api-gateway`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Contract UUID
		uuid, err := resolveContractUUID(dirUUID)
		if err != nil {
			return err
		}

		// 2. Validate Path
		if dirPath == "" {
			return fmt.Errorf("--path is required")
		}

		// 3. Call Manager
		if err := contract.AddWorkdir(uuid, dirPath, dirName); err != nil {
			return err
		}

		// 4. Success Message
		displayName := dirName
		if displayName == "" {
			displayName = filepath.Base(dirPath)
		}
		fmt.Printf("Successfully added workdir '%s' to contract %s\n", displayName, uuid)
		return nil
	},
}

// listWorkdirsCmd handles 'gsc contract list-workdirs'
var listWorkdirsCmd = &cobra.Command{
	Use:   "list-workdirs",
	Short: "List all working directories in the contract",
	Long: `Displays all working directories associated with the contract,
indicating which is primary and which are secondary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Contract UUID
		uuid, err := resolveContractUUID(dirUUID)
		if err != nil {
			return err
		}

		// 2. Get Contract Info
		info, err := contract.GetContractInfo(uuid, false)
		if err != nil {
			return err
		}

		// 3. Format Output
		if dirFormat == "json" {
			bytes, err := json.MarshalIndent(info.Workdirs, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			fmt.Println(string(bytes))
		} else {
			renderWorkdirsList(info)
		}

		return nil
	},
}

// removeWorkdirCmd handles 'gsc contract remove-workdir'
var removeWorkdirCmd = &cobra.Command{
	Use:   "remove-workdir --name <name>",
	Short: "Remove a secondary working directory from the contract",
	Long: `Removes a working directory from the contract.
The primary workdir cannot be removed. Use 'set-primary-workdir' to change the primary first.`,
	Example: `  gsc contract remove-workdir --name "shared-utils"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Contract UUID
		uuid, err := resolveContractUUID(dirUUID)
		if err != nil {
			return err
		}

		// 2. Validate Name
		if dirName == "" {
			return fmt.Errorf("--name is required")
		}

		// 3. Call Manager
		if err := contract.RemoveWorkdir(uuid, dirName); err != nil {
			return err
		}

		fmt.Printf("Successfully removed workdir '%s' from contract %s\n", dirName, uuid)
		return nil
	},
}

// setPrimaryWorkdirCmd handles 'gsc contract set-primary-workdir'
var setPrimaryWorkdirCmd = &cobra.Command{
	Use:   "set-primary-workdir --name <name>",
	Short: "Change the primary working directory",
	Long: `Swaps the specified workdir with the current primary workdir (index 0).
This operation validates that the target directory is not already the primary
of another active contract to prevent conflicts.`,
	Example: `  gsc contract set-primary-workdir --name "user-service"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve Contract UUID
		uuid, err := resolveContractUUID(dirUUID)
		if err != nil {
			return err
		}

		// 2. Validate Name
		if dirName == "" {
			return fmt.Errorf("--name is required")
		}

		// 3. Call Manager
		if err := contract.SetPrimaryWorkdir(uuid, dirName); err != nil {
			return err
		}

		fmt.Printf("Successfully set '%s' as primary workdir for contract %s\n", dirName, uuid)
		return nil
	},
}

// ==========================================
// Initialization
// ==========================================

func init() {
	// Add Workdir Flags
	addWorkdirCmd.Flags().StringVar(&dirPath, "path", "", "Path to the working directory (required)")
	addWorkdirCmd.Flags().StringVar(&dirName, "name", "", "Display name for the workdir (optional, defaults to basename)")
	addWorkdirCmd.Flags().StringVar(&dirUUID, "uuid", "", "Contract UUID (optional, defaults to current directory)")
	addWorkdirCmd.MarkFlagRequired("path")

	// List Workdirs Flags
	listWorkdirsCmd.Flags().StringVarP(&dirFormat, "format", "f", "human", "Output format (human, json)")
	listWorkdirsCmd.Flags().StringVar(&dirUUID, "uuid", "", "Contract UUID (optional, defaults to current directory)")

	// Remove Workdir Flags
	removeWorkdirCmd.Flags().StringVar(&dirName, "name", "", "Name of the workdir to remove (required)")
	removeWorkdirCmd.Flags().StringVar(&dirUUID, "uuid", "", "Contract UUID (optional, defaults to current directory)")
	removeWorkdirCmd.MarkFlagRequired("name")

	// Set Primary Workdir Flags
	setPrimaryWorkdirCmd.Flags().StringVar(&dirName, "name", "", "Name of the workdir to set as primary (required)")
	setPrimaryWorkdirCmd.Flags().StringVar(&dirUUID, "uuid", "", "Contract UUID (optional, defaults to current directory)")
	setPrimaryWorkdirCmd.MarkFlagRequired("name")

	// Register Subcommands
	contractCmd.AddCommand(addWorkdirCmd)
	contractCmd.AddCommand(listWorkdirsCmd)
	contractCmd.AddCommand(removeWorkdirCmd)
	contractCmd.AddCommand(setPrimaryWorkdirCmd)
}

// ==========================================
// Helper Functions
// ==========================================

// renderWorkdirsList formats the workdirs for human-readable output
func renderWorkdirsList(info *contract.ContractInfoResult) {
	fmt.Printf("Contract: %s\n", info.UUID)
	fmt.Printf("Description: %s\n\n", info.Description)

	if len(info.Workdirs) == 0 {
		fmt.Println("No working directories found.")
		return
	}

	// Header
	fmt.Printf("%-12s %-20s %-50s %-8s\n", "Type", "Name", "Path", "Status")
	fmt.Printf("%-12s %-20s %-50s %-8s\n", strings.Repeat("-", 12), strings.Repeat("-", 20), strings.Repeat("-", 50), strings.Repeat("-", 8))

	// Rows
	for i, w := range info.Workdirs {
		workdirType := "Secondary"
		if i == 0 {
			workdirType = "Primary"
		}

		// Truncate long paths for display
		displayPath := w.Path
		if len(displayPath) > 50 {
			displayPath = "..." + displayPath[len(displayPath)-47:]
		}

		fmt.Printf("%-12s %-20s %-50s %-8s\n", workdirType, w.Name, displayPath, w.Status)
	}
}
