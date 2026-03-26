/**
 * Component: Contract CLI Lifecycle
 * Block-UUID: 57f701d0-afb3-404d-b423-d8e6919cd77a
 * Parent-UUID: b4a0c085-3ecd-4d20-81b7-ed8630c9519b
 * Version: 1.0.3
 * Description: CLI commands for managing contract lifecycle: status, list, cancel, renew, complete, delete, and info.
 * Language: Go
 * Created-at: 2026-03-26T16:14:05.132Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package contract

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/output"
)

// statusContractCmd handles 'gsc contract status'
var statusContractCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the contract for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		uuid, err := findContractUUIDByWorkdir()
		if err != nil {
			// findContractUUIDByWorkdir returns an error if no contract is found
			// We want to handle this gracefully for the status command
			if strings.Contains(err.Error(), "no active contract") {
				fmt.Println("No active contract found for this repository.")
				fmt.Println("")
				fmt.Println("To create a new contract, run:")
				fmt.Println("  gsc contract create --code <6-digit-code> --description \"Purpose of contract\"")
				return nil
			}
			return err
		}

		meta, err := contract.GetContract(uuid)
		if err != nil {
			return err
		}

		display := output.ContractDisplay{
			UUID:        meta.UUID,
			Workdir: func() string {
				if len(meta.Workdirs) > 0 { return meta.Workdirs[0].Path }
				return ""
			}(),
			Description: meta.Description,
			Status:      string(meta.Status),
			ExpiresAt:   meta.ExpiresAt.Format("2006-01-02 15:04:05"),
		}

		fmt.Print(output.FormatContractStatus(display))
		return nil
	},
}

// listContractCmd handles 'gsc contract list'
var listContractCmd = &cobra.Command{
	Use:   "list",
	Short: "List all traceability contracts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if contractListAll {
			contractStatus = "all"
		}
		contracts, err := contract.ListContracts()
		if err != nil {
			return err
		}

		filtered := filterContracts(contracts, contractStatus)
		sortContracts(filtered, contractSort, contractOrder)

		displayContracts := make([]output.ContractDisplay, len(filtered))
		for i, c := range filtered {
			displayContracts[i] = output.ContractDisplay{
				UUID: c.UUID,
				Workdir: func() string {
					if len(c.Workdirs) > 0 { return c.Workdirs[0].Path }
					return ""
				}(),
				Description: c.Description,
				Status:      string(c.Status),
				ExpiresAt:   c.ExpiresAt.Format("2006-01-02 15:04:05"),
			}
		}

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

		return contract.CancelContract(uuid)
	},
}

// renewContractCmd handles 'gsc contract renew [uuid]'
var renewContractCmd = &cobra.Command{
	Use:   "renew [uuid]",
	Short: "Extend the expiration time of a contract",
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

		return contract.RenewContract(uuid, contractRenewHours)
	},
}

// completeContractCmd handles 'gsc contract complete [uuid]'
var completeContractCmd = &cobra.Command{
	Use:   "complete [uuid]",
	Short: "Mark an active traceability contract as finished/done",
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

		return contract.CompleteContract(uuid)
	},
}

// deleteContractCmd handles 'gsc contract delete [uuid]'
var deleteContractCmd = &cobra.Command{
	Use:   "delete [uuid]",
	Short: "Delete a traceability contract",
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

		return contract.DeleteContract(uuid)
	},
}

// infoContractCmd handles 'gsc contract info [uuid]'
var infoContractCmd = &cobra.Command{
	Use:   "info [uuid]",
	Short: "Display detailed information about a contract",
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

		info, err := contract.GetContractInfo(uuid, contractInfoSanitize)
		if err != nil {
			return err
		}

		fmt.Print(contract.FormatContractInfo(info, contractInfoFormat))
		return nil
	},
}

func init() {
	// List Flags
	listContractCmd.Flags().StringVar(&contractStatus, "status", "active", "Comma-separated list of statuses (active, expired, cancelled, done, all)")
	listContractCmd.Flags().StringVar(&contractSort, "sort", "expires", "Sort field (expires, created, description)")
	listContractCmd.Flags().StringVar(&contractOrder, "order", "asc", "Sort order (asc, desc)")
	listContractCmd.Flags().StringVarP(&contractFormat, "format", "f", "human", "Output format (human, json)")
	listContractCmd.Flags().BoolVar(&contractListAll, "all", false, "List all contracts regardless of status (overrides --status)")

	// Renew Flags
	renewContractCmd.Flags().IntVar(&contractRenewHours, "hours", 24, "Number of hours to extend the expiration")

	// Info Flags
	infoContractCmd.Flags().StringVarP(&contractInfoFormat, "format", "f", "human", "Output format (human, json)")
	infoContractCmd.Flags().BoolVar(&contractInfoSanitize, "sanitize", false, "Sanitize output (e.g., relative paths)")

	// Register Subcommands
	contractCmd.AddCommand(statusContractCmd)
	contractCmd.AddCommand(listContractCmd)
	contractCmd.AddCommand(cancelContractCmd)
	contractCmd.AddCommand(deleteContractCmd)
	contractCmd.AddCommand(renewContractCmd)
	contractCmd.AddCommand(completeContractCmd)
	contractCmd.AddCommand(infoContractCmd)
}
