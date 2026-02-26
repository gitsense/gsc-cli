/*
 * Component: Contract CLI Commands
 * Block-UUID: ec36e852-512f-4568-8aec-ad6f75f24b06
 * Parent-UUID: d03dcaac-b5c2-4ce0-841a-512371a27dd8
 * Version: 1.2.0
 * Description: Added 'status' command to display the active contract for the current working directory, providing a quick way to check contract validity and details.
 * Language: Go
 * Created-at: 2026-02-26T04:50:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0)
 */


package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/output"
)

var (
	// Create flags
	contractCode        string
	contractDescription string

	// List flags
	contractStatus string
	contractSort   string
	contractOrder  string
	contractFormat string

	// Renew flags
	contractRenewHours int

	// Update/New file flags
	contractUUID string
	contractFile string
)

// contractCmd represents the base command for contract management
var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Manage traceability contracts between CLI and Chat",
	Long: `Contracts establish a formal link between a local working directory and a 
GitSense Chat session, enabling secure and traceable code updates.`,
}

// createContractCmd handles 'gsc contract create'
var createContractCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new traceability contract for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve workdir to current directory
		workdir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Call manager
		meta, err := contract.CreateContract(contractCode, contractDescription, workdir)
		if err != nil {
			return err
		}

		fmt.Printf("Contract created successfully.\n")
		fmt.Printf("UUID: %s\n", meta.UUID)
		fmt.Printf("Expires: %s\n", meta.ExpiresAt.Format(time.RFC3339))
		return nil
	},
}

// statusContractCmd handles 'gsc contract status'
var statusContractCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the contract for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve workdir to current directory
		workdir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Call manager
		meta, err := contract.GetContractByWorkdir(workdir)
		if err != nil {
			// Handle "no contract" gracefully (exit 0)
			if strings.Contains(err.Error(), "no active contract") {
				fmt.Println("No active contract found for this repository.")
				fmt.Println("To create a new contract, run:")
				fmt.Println("  gsc contract create --code <6-digit-code> --description \"Purpose of contract\"")
				return nil
			}
			// Other errors (like multiple contracts) should fail
			return err
		}

		// Map to Display Format
		display := output.ContractDisplay{
			UUID:        meta.UUID,
			Description: meta.Description,
			Workdir:     meta.Workdir,
			Status:      string(meta.Status),
			ExpiresAt:   meta.ExpiresAt.Format(time.RFC3339),
		}

		// Output
		fmt.Print(output.FormatContractStatus(display))
		return nil
	},
}

// listContractCmd handles 'gsc contract list'
var listContractCmd = &cobra.Command{
	Use:   "list",
	Short: "List all traceability contracts",
	RunE: func(cmd *cobra.Command, args []string) error {
		contracts, err := contract.ListContracts()
		if err != nil {
			return err
		}

		// Filter by status
		filtered := filterContracts(contracts, contractStatus)

		// Sort
		sortContracts(filtered, contractSort, contractOrder)

		// Map to Display Format
		displayContracts := make([]output.ContractDisplay, len(filtered))
		for i, c := range filtered {
			displayContracts[i] = output.ContractDisplay{
				UUID:        c.UUID,
				Description: c.Description,
				Workdir:     c.Workdir,
				Status:      string(c.Status),
				ExpiresAt:   c.ExpiresAt.Format(time.RFC3339),
			}
		}

		// Output
		if contractFormat == "json" {
			output.FormatJSON(displayContracts)
		} else {
			fmt.Print(output.FormatContractList(displayContracts))
		}
		return nil
	},
}

// cancelContractCmd handles 'gsc contract cancel [uuid]'
var cancelContractCmd = &cobra.Command{
	Use:   "cancel [uuid]",
	Short: "Cancel an active traceability contract",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := ""
		if len(args) > 0 {
			uuid = args[0]
		}

		// Smart Default: Find UUID by workdir if not provided
		if uuid == "" {
			foundUUID, err := findContractUUIDByWorkdir()
			if err != nil {
				return err
			}
			uuid = foundUUID
		}

		return contract.CancelContract(uuid)
	},
}

// renewContractCmd handles 'gsc contract renew [uuid]'
var renewContractCmd = &cobra.Command{
	Use:   "renew [uuid]",
	Short: "Extend the expiration time of a contract",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uuid := ""
		if len(args) > 0 {
			uuid = args[0]
		}

		// Smart Default: Find UUID by workdir if not provided
		if uuid == "" {
			foundUUID, err := findContractUUIDByWorkdir()
			if err != nil {
				return err
			}
			uuid = foundUUID
		}

		return contract.RenewContract(uuid, contractRenewHours)
	},
}

// updateFileCmd handles 'gsc contract update-file'
var updateFileCmd = &cobra.Command{
	Use:   "update-file",
	Short: "Update an existing traceable file using a contract",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := contract.UpdateFile(contractUUID, contractFile)
		if err != nil {
			// Handle ContractError for specific exit codes
			if cErr, ok := err.(*contract.ContractError); ok {
				return &cliError{code: cErr.Code, message: cErr.Message}
			}
			return err
		}
		fmt.Println("File updated successfully.")
		return nil
	},
}

// newFileCmd handles 'gsc contract new-file'
var newFileCmd = &cobra.Command{
	Use:   "new-file [target-relative-path]",
	Short: "Create a new traceable file using a contract",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetPath := args[0]
		err := contract.NewFile(contractUUID, targetPath, contractFile)
		if err != nil {
			// Handle ContractError for specific exit codes
			if cErr, ok := err.(*contract.ContractError); ok {
				return &cliError{code: cErr.Code, message: cErr.Message}
			}
			return err
		}
		fmt.Println("File created successfully.")
		return nil
	},
}

func init() {
	// Create Flags
	createContractCmd.Flags().StringVar(&contractCode, "code", "", "6-digit handshake code from chat (required)")
	createContractCmd.Flags().StringVar(&contractDescription, "description", "", "Description of the contract's purpose (required)")
	createContractCmd.MarkFlagRequired("code")
	createContractCmd.MarkFlagRequired("description")

	// List Flags
	listContractCmd.Flags().StringVar(&contractStatus, "status", "active", "Comma-separated list of statuses (active, expired, cancelled, all)")
	listContractCmd.Flags().StringVar(&contractSort, "sort", "expires", "Sort field (expires, created, description)")
	listContractCmd.Flags().StringVar(&contractOrder, "order", "asc", "Sort order (asc, desc)")
	listContractCmd.Flags().StringVarP(&contractFormat, "format", "f", "human", "Output format (human, json)")

	// Renew Flags
	renewContractCmd.Flags().IntVar(&contractRenewHours, "hours", 24, "Number of hours to extend the expiration")

	// Update-File Flags
	updateFileCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (required)")
	updateFileCmd.Flags().StringVar(&contractFile, "file", "", "Path to the file containing new code (required)")
	updateFileCmd.MarkFlagRequired("uuid")
	updateFileCmd.MarkFlagRequired("file")

	// New-File Flags
	newFileCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (required)")
	newFileCmd.Flags().StringVar(&contractFile, "file", "", "Path to the file containing new code (required)")
	newFileCmd.MarkFlagRequired("uuid")
	newFileCmd.MarkFlagRequired("file")

	// Add subcommands to base contract command
	contractCmd.AddCommand(createContractCmd)
	contractCmd.AddCommand(statusContractCmd)
	contractCmd.AddCommand(listContractCmd)
	contractCmd.AddCommand(cancelContractCmd)
	contractCmd.AddCommand(renewContractCmd)
	contractCmd.AddCommand(updateFileCmd)
	contractCmd.AddCommand(newFileCmd)
}

// RegisterContractCommand adds the contract command to the root CLI
func RegisterContractCommand(root *cobra.Command) {
	root.AddCommand(contractCmd)
}

// Helper: filterContracts filters the list based on the status string
func filterContracts(contracts []contract.ContractMetadata, statusStr string) []contract.ContractMetadata {
	if statusStr == "" || statusStr == "all" {
		return contracts
	}

	parts := strings.Split(statusStr, ",")
	var filtered []contract.ContractMetadata

	for _, c := range contracts {
		for _, part := range parts {
			s := strings.TrimSpace(part)
			if s == "all" {
				return contracts
			}
			if string(c.Status) == s {
				filtered = append(filtered, c)
				break
			}
		}
	}
	return filtered
}

// Helper: sortContracts sorts the list based on field and order
func sortContracts(contracts []contract.ContractMetadata, field, order string) {
	less := func(i, j int) bool {
		switch field {
		case "created":
			if order == "desc" {
				return contracts[i].CreatedAt.After(contracts[j].CreatedAt)
			}
			return contracts[i].CreatedAt.Before(contracts[j].CreatedAt)
		case "description":
			if order == "desc" {
				return contracts[i].Description > contracts[j].Description
			}
			return contracts[i].Description < contracts[j].Description
		default: // expires
			if order == "desc" {
				return contracts[i].ExpiresAt.After(contracts[j].ExpiresAt)
			}
			return contracts[i].ExpiresAt.Before(contracts[j].ExpiresAt)
		}
	}
	sort.Slice(contracts, less)
}

// Helper: findContractUUIDByWorkdir finds the active contract for the current directory
func findContractUUIDByWorkdir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Resolve to absolute path
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	contracts, err := contract.ListContracts()
	if err != nil {
		return "", err
	}

	var matches []string
	for _, c := range contracts {
		if c.Status == contract.ContractActive && c.Workdir == absCwd {
			matches = append(matches, c.UUID)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no active contracts found in this directory")
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple active contracts found in this directory. Please specify a UUID")
	}

	return matches[0], nil
}

// cliError wraps an error with a specific exit code for Cobra
type cliError struct {
	code    int
	message string
}

func (e *cliError) Error() string {
	return e.message
}
