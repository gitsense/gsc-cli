/**
 * Component: Contract CLI Operations
 * Block-UUID: a6010350-1044-457a-a1ad-a433dd0ac3e6
 * Parent-UUID: 5be3be98-8716-4873-91d7-bf77f2b9ec68
 * Version: 1.0.1
 * Description: CLI commands for file operations: update-file, new-file, and test.
 * Language: Go
 * Created-at: 2026-03-08T04:06:49.821Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0), GLM-4.7 (v1.0.1)
 */


package contract

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
)

// updateFileCmd handles 'gsc contract update-file'
var updateFileCmd = &cobra.Command{
	Use:   "update-file",
	Short: "Update an existing traceable file using a contract",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := contract.UpdateFile(contractUUID, contractAuthcodeExec, contractFile)
		if err != nil {
			if cErr, ok := err.(*contract.ContractError); ok {
				return &cliError{code: cErr.Code, message: cErr.Message}
			}
			return err
		}
		fmt.Println("File updated successfully.")
		return nil
	},
}

// newFileCmd handles 'gsc contract new-file [target-relative-path]'
var newFileCmd = &cobra.Command{
	Use:   "new-file [target-relative-path]",
	Short: "Create a new traceable file using a contract",
	Hidden: true,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		targetPath := args[0]
		err := contract.NewFile(contractUUID, contractAuthcodeExec, targetPath, contractFile)
		if err != nil {
			if cErr, ok := err.(*contract.ContractError); ok {
				return &cliError{code: cErr.Code, message: cErr.Message}
			}
			return err
		}
		fmt.Println("File created successfully.")
		return nil
	},
}

// testContractCmd handles 'gsc contract test [uuid]'
var testContractCmd = &cobra.Command{
	Use:   "test [uuid]",
	Short: "Test a file change against a contract without writing it",
	Hidden: true,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		uuid := ""
		if len(args) > 0 {
			uuid = args[0]
		}

		if uuid == "" {
			foundUUID, err := findContractUUIDByWorkdir()
			if err != nil {
				return err
			}
			uuid = foundUUID
		}

		result, err := contract.TestFile(uuid, contractTestFile, contractTestSanitize)
		if err != nil {
			return err
		}

		fmt.Print(contract.FormatContractTest(result, contractTestFormat))
		return nil
	},
}

func init() {
	// Update-File Flags
	updateFileCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (required)")
	updateFileCmd.Flags().StringVar(&contractFile, "file", "", "Path to the file containing new code (required)")
	updateFileCmd.Flags().StringVar(&contractAuthcodeExec, "authcode", "", "4-digit authorization code (required)")
	updateFileCmd.MarkFlagRequired("uuid")
	updateFileCmd.MarkFlagRequired("file")
	updateFileCmd.MarkFlagRequired("authcode")

	// New-File Flags
	newFileCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (required)")
	newFileCmd.Flags().StringVar(&contractFile, "file", "", "Path to the file containing new code (required)")
	newFileCmd.Flags().StringVar(&contractAuthcodeExec, "authcode", "", "4-digit authorization code (required)")
	newFileCmd.MarkFlagRequired("uuid")
	newFileCmd.MarkFlagRequired("file")
	newFileCmd.MarkFlagRequired("authcode")

	// Test Flags
	testContractCmd.Flags().StringVarP(&contractTestFormat, "format", "f", "human", "Output format (human, json)")
	testContractCmd.Flags().BoolVar(&contractTestSanitize, "sanitize", false, "Sanitize output (e.g., relative paths)")
	testContractCmd.Flags().StringVar(&contractTestFile, "file", "", "Path to the file containing new code to test (required)")
	testContractCmd.MarkFlagRequired("file")

	// Register Subcommands
	contractCmd.AddCommand(updateFileCmd)
	contractCmd.AddCommand(newFileCmd)
	contractCmd.AddCommand(testContractCmd)
}
