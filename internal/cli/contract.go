/*
 * Component: Contract CLI Commands
 * Block-UUID: 68390be0-8fd0-4624-bddd-cb16fffeb047
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the CLI command structure for managing traceability contracts, including creation, listing, cancellation, renewal, and file updates/creation.
 * Language: Go
 * Created-at: 2026-02-26T04:50:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0)
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
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/logger"
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
		meta, err := manifest.CreateContract(contractCode, contractDescription, workdir)
		if err != nil {
			return err
		}

		fmt.Printf("Contract created successfully.\n")
		fmt.Printf("UUID: %s\n", meta.UUID)
		fmt.Printf("Expires: %s\n", meta.ExpiresAt.Format(time.RFC3339))
		return nil
	},
}

// listContractCmd handles 'gsc contract list'
var listContractCmd = &cobra.Command{
	Use:   "list",
	Short: "List all traceability contracts",
	RunE: func(cmd *cobra.Command, args []string) error {
		contracts, err := manifest.ListContracts()
		if err != nil {
			return err
		}

		// Filter by status
		filtered := filterContracts(contracts, contractStatus)

		// Sort
		sortContracts(filtered, contractSort, contractOrder)

		// Output
		if contractFormat == "json" {
			output.FormatJSON(filtered)
		} else {
			fmt.Print(output.FormatContractList(filtered))
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

		return manifest.CancelContract(uuid)
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

		return manifest.RenewContract(uuid, contractRenewHours)
	},
}

// updateFileCmd handles 'gsc contract update-file'
var updateFileCmd = &cobra.Command{
	Use:   "update-file",
	Short: "Update an existing traceable file using a contract",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := manifest.UpdateFile(contractUUID, contractFile)
		if err != nil {
			// Handle ContractError for specific exit codes
			if cErr, ok := err.(*manifest.ContractError); ok {
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
		err := manifest.NewFile(contractUUID, targetPath, contractFile)
		if err != nil {
			// Handle ContractError for specific exit codes
			if cErr, ok := err.(*manifest.ContractError); ok {
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
func filterContracts(contracts []manifest.ContractMetadata, statusStr string) []manifest.ContractMetadata {
	if statusStr == "" || statusStr == "all" {
		return contracts
	}

	parts := strings.Split(statusStr, ",")
	var filtered []manifest.ContractMetadata

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
func sortContracts(contracts []manifest.ContractMetadata, field, order string) {
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

	contracts, err := manifest.ListContracts()
	if err != nil {
		return "", err
	}

	var matches []string
	for _, c := range contracts {
		if c.Status == manifest.ContractActive && c.Workdir == absCwd {
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
