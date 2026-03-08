/*
 * Component: Contract CLI Root
 * Block-UUID: 6d6a4ff5-e27e-42ca-adc9-5988bb4d08f4
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Root command definition and flag initialization for the contract CLI package.
 * Language: Go
 * Created-at: 2026-03-08T00:21:11.145Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// ==========================================
// Global Flags
// ==========================================

// Create flags
var (
	contractCode        string
	contractDescription string
	contractAuthcode    string
	contractWhitelistFile string
	contractNoWhitelist   bool
	contractExecTimeout   int
	contractPreferredEditor   string
	contractPreferredTerminal string
	contractPreferredReview   string
)

// List flags
var (
	contractStatus string
	contractSort   string
	contractOrder  string
	contractFormat string
	contractListAll bool
)

// Renew flags
var (
	contractRenewHours int
)

// Update/New file flags
var (
	contractUUID         string
	contractFile         string
	contractAuthcodeExec string
)

// Info flags
var (
	contractInfoFormat   string
	contractInfoSanitize bool
)

// Test flags
var (
	contractTestFormat   string
	contractTestSanitize bool
	contractTestFile     string
)

// Exec flags
var (
	contractExecUUID     string
	contractExecAuthcode string
	contractExecCmd      string
	contractExecChat     bool
)

// Launch flags
var (
	contractLaunchAlias           string
	contractLaunchBlockUUID        string
	contractLaunchParentUUID       string
	contractLaunchAction           string
	contractLaunchAppOverride      string
	contractLaunchCmd              string
	contractLaunchList             bool
	contractLaunchFormat           string
	contractLaunchHash             string
	contractLaunchPosition         int
	contractLaunchActiveChatID     int64
)

// Dump flags (Shared)
var (
	contractDumpUUID   string
	contractDumpOutput string
	contractDumpIncludeSystem bool
	contractDumpDebugPatch bool
	contractDumpRaw    bool
	contractDumpFormat    string
	contractDumpAuthcode string
)

// Dump flags (Merged specific)
var (
	contractDumpSort   string
)

// Dump flags (Mapped specific)
var (
	contractDumpMessageID int64
	contractDumpValidate  bool
)

// ==========================================
// Root Command
// ==========================================

// contractCmd represents the base command for contract management
var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Manage traceability contracts between CLI and Chat",
	Long: `Contracts establish a formal link between a local working directory and a 
GitSense Chat session, enabling secure and traceable code updates.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Enforce GSC_HOME requirement
		// This ensures that the web app's data directory is used for contracts and dumps
		if _, err := settings.GetGSCHome(true); err != nil {
			cmd.SilenceUsage = true
			return err
		}
		return nil
	},
}

// ==========================================
// Registration
// ==========================================

// RegisterContractCommand adds the contract command to the root CLI
func RegisterContractCommand(root *cobra.Command) {
	root.AddCommand(contractCmd)
}

// ==========================================
// Initialization
// ==========================================

func init() {
	// ==========================================
	// Create Flags
	// ==========================================
	// Note: These flags are defined here but used in create.go
	// We attach them to the createContractCmd in create.go's init()
	
	// ==========================================
	// List Flags
	// ==========================================
	// Note: These flags are defined here but used in lifecycle.go

	// ==========================================
	// Renew Flags
	// ==========================================
	// Note: These flags are defined here but used in lifecycle.go

	// ==========================================
	// Update-File Flags
	// ==========================================
	// Note: These flags are defined here but used in ops.go

	// ==========================================
	// New-File Flags
	// ==========================================
	// Note: These flags are defined here but used in ops.go

	// ==========================================
	// Info Flags
	// ==========================================
	// Note: These flags are defined here but used in lifecycle.go

	// ==========================================
	// Test Flags
	// ==========================================
	// Note: These flags are defined here but used in ops.go

	// ==========================================
	// Exec Flags
	// ==========================================
	// Note: These flags are defined here but used in exec.go

	// ==========================================
	// Launch Flags
	// ==========================================
	// Note: These flags are defined here but used in exec.go

	// ==========================================
	// Dump Flags (Refactored)
	// ==========================================
	// Note: These flags are defined here but used in dump.go

	// ==========================================
	// Register Subcommands
	// ==========================================
	// The following calls will be added as we create the respective files:
	// contractCmd.AddCommand(createContractCmd)
	// contractCmd.AddCommand(statusContractCmd)
	// contractCmd.AddCommand(listContractCmd)
	// contractCmd.AddCommand(cancelContractCmd)
	// contractCmd.AddCommand(deleteContractCmd)
	// contractCmd.AddCommand(renewContractCmd)
	// contractCmd.AddCommand(completeContractCmd)
	// contractCmd.AddCommand(updateFileCmd)
	// contractCmd.AddCommand(newFileCmd)
	// contractCmd.AddCommand(infoContractCmd)
	// contractCmd.AddCommand(testContractCmd)
	// contractCmd.AddCommand(execContractCmd)
	// contractCmd.AddCommand(launchContractCmd)
	// contractCmd.AddCommand(dumpContractCmd)
}

// ==========================================
// Helper Functions (Moved from original file)
// ==========================================

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

// verifyDumpAuthcode checks the authcode if provided.
// If the authcode is empty, it bypasses the check (local user).
func verifyDumpAuthcode(uuid string, authcode string) error {
	if authcode == "" {
		return nil
	}
	meta, err := contract.GetContract(uuid)
	if err != nil {
		return err
	}
	if meta.Authcode != authcode {
		return &cliError{code: contract.ExitInvalidAuthcode, message: "Invalid authorization code"}
	}
	return nil
}

// cliError wraps an error with a specific exit code for Cobra
type cliError struct {
	code    int
	message string
}

func (e *cliError) Error() string {
	return e.message
}

// getMapKeys returns a sorted slice of keys from a map
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// getSortedKeys returns a sorted slice of keys from a map (alias for getMapKeys for clarity in wizard)
func getSortedKeys(m map[string]string) []string {
	return getMapKeys(m)
}
