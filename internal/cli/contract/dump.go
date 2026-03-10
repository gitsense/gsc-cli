/**
 * Component: Contract CLI Dump
 * Block-UUID: d4d094e8-3d83-400e-8eb4-b8fdf645ce40
 * Parent-UUID: ddd4996c-6d75-4a9e-b6a8-8cd6a80df714
 * Version: 1.1.0
 * Description: CLI commands for dumping chat history into filesystem structures (tree, merged, mapped) and managing shadow workspaces.
 * Language: Go
 * Created-at: 2026-03-10T14:33:21.390Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0), GLM-4.7 (v1.1.0)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ==========================================
// DUMP COMMANDS
// ==========================================

// dumpContractCmd is the parent command for dump strategies.
// It acts as a container and displays help if no subcommand is provided.
var dumpContractCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump chat history into a navigable filesystem tree",
	Long: `Extracts all code blocks and messages from chats associated with a contract 
and organizes them into a directory structure for local review and search.`,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, show help
		cmd.Help()
	},
}

// dumpTreeCmd handles 'gsc contract dump tree'
var dumpTreeCmd = &cobra.Command{
	Use:     "tree",
	Aliases: []string{"t"},
	Short:   "Dump strategy: Conversational tree",
	Long: `This strategy organizes artifacts by chat chronology, preserving the
original conversation flow. Each message gets its own directory containing
the raw message, code blocks, and patch results.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve UUID
		uuid, err := resolveDumpUUID(contractDumpUUID)
		if err != nil {
			return err
		}

		// 2. Verify Authcode (if provided)
		if err := verifyDumpAuthcode(uuid, contractDumpAuthcode); err != nil {
			return err
		}

		// 3. Resolve Output Directory (Type-Aware)
		outputDir := contractDumpOutput
		if outputDir == "" {
			outputDir = contract.GetDefaultHomeDir(uuid, "tree")
		}

		// 4. Select Strategy
		writer := &contract.TreeWriter{}

		// 5. Execute
		logger.Info("Generating conversational dump...", "type", "tree", "output", outputDir, "trim", !contractDumpRaw)
		
		_, err = contract.ExecuteDump(uuid, writer, outputDir, contractDumpIncludeSystem, !contractDumpRaw, "tree", "", contractDumpDebugPatch, 0, false, 0)
		
		// 6. Handle Output Format
		if contractDumpFormat == "json" {
			if err != nil {
				errorJSON := fmt.Sprintf(`{"success": false, "error": {"code": "EXECUTION_FAILED", "message": "%s"}}`, strings.ReplaceAll(err.Error(), `"`, `\"`))
				fmt.Println(errorJSON)
				return nil
			}
			fmt.Printf(`{"success": true, "message": "Dump generated successfully", "root_dir": "%s"}`, outputDir)
			return nil
		}
		
		// Human Mode
		if err != nil {
			return err
		}
		fmt.Printf("Dump complete: %s\n", outputDir)
		return nil
	},
}

// dumpMergedCmd handles 'gsc contract dump merged'
var dumpMergedCmd = &cobra.Command{
	Use:     "merged",
	Aliases: []string{"m", "s"},
	Short:   "Dump strategy: Merged/Squashed view",
	Long: `This strategy merges duplicate messages across chats and sorts them
by popularity or recency. It provides a condensed view of the conversation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve UUID
		uuid, err := resolveDumpUUID(contractDumpUUID)
		if err != nil {
			return err
		}

		// 2. Verify Authcode (if provided)
		if err := verifyDumpAuthcode(uuid, contractDumpAuthcode); err != nil {
			return err
		}

		// 3. Resolve Output Directory (Type-Aware)
		outputDir := contractDumpOutput
		if outputDir == "" {
			outputDir = contract.GetDefaultHomeDir(uuid, "merged")
		}

		// 4. Select Strategy
		writer := &contract.MergedWriter{}

		// 5. Execute
		logger.Info("Generating merged dump...", "type", "merged", "sort", contractDumpSort, "output", outputDir, "trim", !contractDumpRaw)
		
		_, err = contract.ExecuteDump(uuid, writer, outputDir, contractDumpIncludeSystem, !contractDumpRaw, "merged", contractDumpSort, contractDumpDebugPatch, 0, false, 0)
		
		// 6. Handle Output Format
		if contractDumpFormat == "json" {
			if err != nil {
				errorJSON := fmt.Sprintf(`{"success": false, "error": {"code": "EXECUTION_FAILED", "message": "%s"}}`, strings.ReplaceAll(err.Error(), `"`, `\"`))
				fmt.Println(errorJSON)
				return nil
			}
			fmt.Printf(`{"success": true, "message": "Dump generated successfully", "root_dir": "%s"}`, outputDir)
			return nil
		}
		
		// Human Mode
		if err != nil {
			return err
		}
		fmt.Printf("Dump complete: %s\n", outputDir)
		return nil
	},
}

// dumpMappedCmd handles 'gsc contract dump mapped'
var dumpMappedCmd = &cobra.Command{
	Use:     "mapped",
	Aliases: []string{"map", "shadow"},
	Short:   "Dump strategy: Shadow workspace",
	Long: `This strategy maps code blocks to their project paths, creating a shadow
workspace that shows how files evolved across the conversation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// 1. Resolve UUID
		uuid, err := resolveDumpUUID(contractDumpUUID)
		if err != nil {
			return err
		}

		// 2. Verify Authcode (if provided)
		if err := verifyDumpAuthcode(uuid, contractDumpAuthcode); err != nil {
			return err
		}

		// 3. Resolve Output Directory (Type-Aware)
		// Note: The dumper will append the message hash to this path
		outputDir := contractDumpOutput
		if outputDir == "" {
			outputDir = contract.GetDefaultHomeDir(uuid, "mapped")
		}

		// 4. Select Strategy
		writer := &contract.MappedWriter{}

		// 5. Execute
		logger.Info("Generating mapped dump...", "type", "mapped", "output", outputDir, "trim", !contractDumpRaw, "validate", contractDumpValidate)
		
		result, err := contract.ExecuteDump(uuid, writer, outputDir, contractDumpIncludeSystem, !contractDumpRaw, "mapped", "", contractDumpDebugPatch, contractDumpMessageID, contractDumpValidate, 0)
		
		// 6. Handle Output Format
		if contractDumpFormat == "json" {
			if err != nil {
				errorJSON := fmt.Sprintf(`{"success": false, "error": {"code": "EXECUTION_FAILED", "message": "%s"}}`, strings.ReplaceAll(err.Error(), `"`, `\"`))
				fmt.Println(errorJSON)
				return nil
			}
			
			if result == nil {
				fmt.Println(`{"success": false, "error": {"code": "INTERNAL_ERROR", "message": "No result returned for mapped dump"}}`)
				return nil
			}
			output.FormatJSON(result)
			return nil
		}
		
		// ==========================================
		// VALIDATION MODE (Human Output)
		// ==========================================
		if contractDumpValidate {
			if !result.Exists {
				fmt.Println("✗ No shadow workspace found for this message.")
				fmt.Printf("  Run 'gsc contract dump mapped --message-id %d' to initialize.\n", contractDumpMessageID)
				return nil
			}

			if result.Valid {
				fmt.Println("✓ Shadow workspace is valid")
				fmt.Printf("  Hash:       %s\n", result.Hash)
				fmt.Printf("  Location:   %s\n", result.RootDir)
				
				// Calculate time remaining
				expiresAt, _ := time.Parse(time.RFC3339, result.ExpiresAt)
				duration := time.Until(expiresAt)
				fmt.Printf("  Expires:    %s (in %s)\n", expiresAt.Format(time.RFC3339), duration.Round(time.Minute))
				
				fmt.Printf("  Files:      %d mapped, %d unmapped\n", result.Stats.Mappable, result.Stats.Unmappable)
			} else {
				// This case shouldn't happen with auto-extend, but handle it
				fmt.Println("✗ Shadow workspace is invalid")
				fmt.Printf("  Reason: %s\n", result.Error.Message)
			}
			return nil
		}

		// ==========================================
		// GENERATION MODE (Human Output)
		// ==========================================
		if result != nil {
			// Print human-readable summary for mapped dump
			fmt.Printf("✓ Mapped Dump Generated\n")
			fmt.Printf("  Hash:       %s\n", result.Hash)
			fmt.Printf("  Location:   %s\n\n", result.RootDir)
			
			fmt.Printf("  Mapped Files (%d):\n", result.Stats.Mappable)
			for _, f := range result.Files {
				if f.Status == "mapped" {
					fmt.Printf("    - %s\n", f.Path)
				}
			}
			
			if result.Stats.Unmappable > 0 {
				fmt.Printf("\n  Unmapped Files (%d):\n", result.Stats.Unmappable)
				for _, f := range result.Files {
					if f.Status == "unmapped" {
						fmt.Printf("    - %s (%s)\n", f.Path, f.Reason)
					}
				}
			}
		} else {
			fmt.Printf("Dump complete: %s\n", outputDir)
		}

		return nil
	},
}

// dumpPruneCmd handles 'gsc contract dump prune'
var dumpPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove expired shadow workspaces",
	Long: `Scans the dumps directory for shadow workspaces that have expired
and removes them to free up disk space.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		gscHome, _ := settings.GetGSCHome(false)
		homesRoot := filepath.Join(gscHome, settings.HomesRelPath)

		// Check if homes directory exists
		if _, err := os.Stat(homesRoot); os.IsNotExist(err) {
			fmt.Println("No homes directory found.")
			return nil
		}

		deletedCount := 0
		now := time.Now()

		// Walk the homes directory
		err := filepath.Walk(homesRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// We are looking for workspace.json files
			// These are now located at: homesRoot/<uuid>/mapped/<hash>/workspace.json
			if info.Name() != "workspace.json" {
				return nil
			}

			// Read the manifest
			data, err := os.ReadFile(path)
			if err != nil {
				logger.Warning("Failed to read workspace manifest", "path", path, "error", err)
				return nil
			}

			var ws contract.ShadowWorkspace
			if err := json.Unmarshal(data, &ws); err != nil {
				logger.Warning("Failed to parse workspace manifest", "path", path, "error", err)
				return nil
			}

			// Check expiration
			expiresAt, err := time.Parse(time.RFC3339, ws.ExpiresAt)
			if err != nil {
				logger.Warning("Invalid expiration date in manifest", "path", path, "error", err)
				return nil
			}

			if now.After(expiresAt) {
				// Delete the parent directory (the hash directory)
				// path is .../mapped/<hash>/workspace.json
				// parent is .../mapped/<hash>
				workspaceDir := filepath.Dir(path)
				if err := os.RemoveAll(workspaceDir); err != nil {
					logger.Warning("Failed to delete expired workspace", "path", workspaceDir, "error", err)
				} else {
					deletedCount++
					fmt.Printf("Deleted expired workspace: %s\n", filepath.Base(workspaceDir))
				}
			}

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk homes directory: %w", err)
		}

		fmt.Printf("\nPrune complete. Removed %d expired workspace(s).\n", deletedCount)
		return nil
	},
}

// resolveDumpUUID is a helper to resolve the contract UUID from flag or workdir.
func resolveDumpUUID(uuid string) (string, error) {
	if uuid != "" {
		return uuid, nil
	}
	return findContractUUIDByWorkdir()
}

func init() {
	// ==========================================
	// Dump Flags (Refactored)
	// ==========================================
	
	// Parent Flags (Shared by all dump types)
	dumpContractCmd.PersistentFlags().StringVar(&contractDumpUUID, "uuid", "", "Contract UUID (optional if in workdir)")
	dumpContractCmd.PersistentFlags().StringVarP(&contractDumpOutput, "output", "o", "", "Output directory (default: ~/.gitsense/homes/<uuid>/<type>)")
	dumpContractCmd.PersistentFlags().BoolVar(&contractDumpIncludeSystem, "include-system", false, "Include the system message in the dump (default: false)")
	dumpContractCmd.PersistentFlags().BoolVar(&contractDumpDebugPatch, "debug-patch", false, "Enable patch debugging (persists source and diff artifacts on failure)")
	dumpContractCmd.PersistentFlags().BoolVar(&contractDumpRaw, "raw", false, "Disable smart trimming (preserve exact LLM output)")
	dumpContractCmd.PersistentFlags().StringVarP(&contractDumpFormat, "format", "f", "human", "Output format: human or json (default: human)")

	dumpContractCmd.PersistentFlags().StringVar(&contractDumpAuthcode, "authcode", "", "Authorization code (required for backend requests)")

	// Merged Specific Flags
	dumpMergedCmd.Flags().StringVar(&contractDumpSort, "sort", "recency", "Sort mode for merged type: recency, popularity, chronological")

	// Mapped Specific Flags
	dumpMappedCmd.Flags().Int64Var(&contractDumpMessageID, "message-id", 0, "Filter dump to a specific message ID")
	dumpMappedCmd.Flags().BoolVar(&contractDumpValidate, "validate", false, "Validate an existing shadow workspace instead of generating a new one")

	// Register Subcommands
	dumpContractCmd.AddCommand(dumpTreeCmd)
	dumpContractCmd.AddCommand(dumpMergedCmd)
	dumpContractCmd.AddCommand(dumpMappedCmd)
	dumpContractCmd.AddCommand(dumpPruneCmd)

	// Register Dump Parent Command
	contractCmd.AddCommand(dumpContractCmd)
}
