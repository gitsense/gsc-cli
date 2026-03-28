/**
 * Component: Contract CLI Root
 * Block-UUID: 8e2cec45-408c-4f7b-b39e-3a113a3e6ce5
 * Parent-UUID: 82e60ec5-41ea-45d8-8dbb-6612c74dfc1f
 * Version: 1.10.0
 * Description: Integrated IsInContainer check into PersistentPreRunE to prevent recursive proxy loops for contract commands.
 * Language: Go
 * Created-at: 2026-03-28T16:18:28.019Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0)
 */


package contract

import (
	typescontract "github.com/gitsense/gsc-cli/internal/types/contract"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
	docker_internal "github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"encoding/json"
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

// Add Chat flags
var (
	contractAddUUID string
	contractAddForce bool
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
		// 1. Smart Proxy Interceptor
		// If a Docker context is active and the command is proxyable, redirect to the container.
		
		// 0. Handle Logging Flags (Inherited from rootCmd)
		// We must duplicate this logic here because defining a PersistentPreRunE on a child command
		// overrides the parent's PersistentPreRunE in Cobra.
		quiet, _ := cmd.Flags().GetBool("quiet")
		if quiet {
			logger.SetLogLevel(logger.LevelError)
			logger.Debug("Log level set to ERROR (quiet mode)")
		} else {
			verbose, _ := cmd.Flags().GetCount("verbose")
			logger.Debug("Verbose count detected", "count", verbose)
			switch verbose {
			case 0:
				logger.SetLogLevel(logger.LevelWarning)
			case 1:
				logger.SetLogLevel(logger.LevelInfo)
			default:
				logger.SetLogLevel(logger.LevelDebug)
				logger.Debug("Log level set to DEBUG")
			}
		}

		// This is duplicated here because contractCmd defines its own PersistentPreRunE,
		// which overrides the one in the root command.
		// Check if we are already inside a container to prevent recursive loops.
		if !docker_internal.IsInContainer() && docker_internal.IsProxyableCommand(cmd) && docker_internal.HasContext() {
			proxied, err := docker_internal.ProxyCommand(cmd, args)
			if err != nil {
				// If it's an exit error from the container, exit with that code
				if exitErr, ok := err.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				return err
			}
			if proxied {
				// If the command was successfully proxied, we exit the host process.
				// The exit code was already handled by ProxyCommand if it returned an error.
				// If it returned nil error and proxied=true, we assume success (0).
				os.Exit(0)
			}
		}

		// 2. Enforce GSC_HOME requirement
		// This ensures that the web app's data directory is used for contracts and dumps
		if _, err := settings.GetGSCHome(false); err != nil {
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
			return contracts[i].CreatedAt.After(contracts[j].CreatedAt)
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
		logger.Debug("Error getting CWD", "error", err)
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	logger.Debug("Resolving contract by workdir", "abs_cwd", absCwd)

	contracts, err := contract.ListContracts()
	if err != nil {
		return "", err
	}

	var matches []string
	var expiredMatches []string
	for _, c := range contracts {
		logger.Debug("Inspecting contract", "uuid", c.UUID, "status", c.Status, "workdirs_count", len(c.Workdirs))

		if c.Status == typescontract.ContractActive && len(c.Workdirs) > 0 && c.Workdirs[0].Path == absCwd {
			primaryPath := c.Workdirs[0].Path
			isMatch := primaryPath == absCwd
			logger.Debug("Path comparison result", "uuid", c.UUID, "primary_path", primaryPath, "target_cwd", absCwd, "is_match", isMatch)

			matches = append(matches, c.UUID)
		} else if c.Status == typescontract.ContractExpired && len(c.Workdirs) > 0 && c.Workdirs[0].Path == absCwd {
			// Track expired contracts that match the path for a better error message
			expiredMatches = append(expiredMatches, c.UUID)
		}
	}

	if len(matches) == 0 {
		if len(expiredMatches) > 0 {
			return "", fmt.Errorf("no active contract found. Found expired contract(s): %s. Run 'gsc contract renew <uuid>' to reactivate.", strings.Join(expiredMatches, ", "))
		}
		logger.Debug("No contracts found matching CWD", "abs_cwd", absCwd)
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

// ==========================================
// Helper: Contract UUID Resolution
// ==========================================
// This helper is shared by chats and messages commands.

// resolveContractUUID determines the contract UUID based on priority:
// 1. Explicit --uuid flag
// 2. GSC_CONTRACT_UUID environment variable
// 3. Discovery via .gsc-contract.json
// 4. Workdir lookup (legacy)
func resolveContractUUID(explicitUUID string) (string, error) {
	if explicitUUID != "" {
		return explicitUUID, nil
	}

	// Check Environment Variable
	if envUUID := os.Getenv("GSC_CONTRACT_UUID"); envUUID != "" {
		return envUUID, nil
	}

	// Check Discovery (Marker File)
	if homeDir, err := contract.DiscoverContractHome(); err == nil {
		// Read the marker file to get the UUID
		markerPath := filepath.Join(homeDir, ".gsc-contract.json")
		data, err := os.ReadFile(markerPath)
		if err == nil {
			var marker map[string]string
			if err := json.Unmarshal(data, &marker); err == nil {
				if uuid, ok := marker["contract_uuid"]; ok {
					return uuid, nil
				}
			}
		}
	}

	// Fallback to Workdir Lookup
	return findContractUUIDByWorkdir()
}
