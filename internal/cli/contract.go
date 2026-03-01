/**
 * Component: Contract CLI Commands
 * Block-UUID: 7fc394a7-b39b-4799-b4eb-17dffc83a14e
 * Parent-UUID: 592432f2-0c8f-4c91-9e96-e2bb0a65d422
 * Version: 1.9.8
 * Description: Updated execContractCmd to correctly identify the last message in a chat using a recursive SQL query, ensuring output is appended to the end of the conversation regardless of message deletions or reordering.
 * Language: Go
 * Created-at: 2026-03-01T16:24:23.841Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), Gemini 3 Flash (v1.9.0), GLM-4.7 (v1.9.1), GLM-4.7 (v1.9.2), Gemini 3 Flash (v1.9.3), Gemini 3 Flash (v1.9.4), Gemini 3 Flash (v1.9.5), GLM-4.7 (v1.9.6), GLM-4.7 (v1.9.7), GLM-4.7 (v1.9.8)
 */


package cli

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"database/sql"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge/formatters"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/exec"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	// Create flags
	contractCode        string
	contractDescription string
	contractAuthcode    string
	contractWhitelistFile string
	contractNoWhitelist   bool
	contractExecTimeout   int

	// List flags
	contractStatus string
	contractSort   string
	contractOrder  string
	contractFormat string
	contractListAll bool

	// Renew flags
	contractRenewHours int

	// Update/New file flags
	contractUUID         string
	contractFile         string
	contractAuthcodeExec string

	// Info flags
	contractInfoFormat   string
	contractInfoSanitize bool

	// Test flags
	contractTestFormat   string
	contractTestSanitize bool
	contractTestFile     string

	// Exec flags
	contractExecUUID     string
	contractExecAuthcode string
	contractExecCmd      string
	contractExecChat     bool
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
		cmd.SilenceUsage = true
		// Resolve workdir to current directory
		workdir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Generate random 4-digit authcode if not provided
		if contractAuthcode == "" {
			n, err := rand.Int(rand.Reader, big.NewInt(10000))
			if err != nil {
				return fmt.Errorf("failed to generate random authcode: %w", err)
			}
			contractAuthcode = fmt.Sprintf("%04d", n.Int64())
		}

		// Process Whitelist File if provided
		var whitelist []string
		if contractWhitelistFile != "" {
			file, err := os.Open(contractWhitelistFile)
			if err != nil {
				return fmt.Errorf("failed to open whitelist file: %w", err)
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line != "" && !strings.HasPrefix(line, "#") {
					whitelist = append(whitelist, line)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read whitelist file: %w", err)
			}
		}

		// Call manager with new security parameters
		meta, err := contract.CreateContract(
			contractCode, 
			contractDescription, 
			contractAuthcode, 
			workdir,
			whitelist,
			contractNoWhitelist,
			contractExecTimeout,
		)
		if err != nil {
			return err
		}

		fmt.Printf("Contract created successfully.\n")
		fmt.Printf("UUID: %s\n", meta.UUID)
		fmt.Printf("Authcode: %s\n", contractAuthcode)
		fmt.Printf("Expires: %s\n", meta.ExpiresAt.Format(time.RFC3339))
		return nil
	},
}

// statusContractCmd handles 'gsc contract status'
var statusContractCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the contract for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
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
				fmt.Println("")
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
		cmd.SilenceUsage = true
		// If --all is set, override the status filter
		if contractListAll {
			contractStatus = "all"
		}
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
		cmd.SilenceUsage = true
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
		cmd.SilenceUsage = true
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
		cmd.SilenceUsage = true
		err := contract.UpdateFile(contractUUID, contractAuthcodeExec, contractFile)
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
		cmd.SilenceUsage = true
		targetPath := args[0]
		err := contract.NewFile(contractUUID, contractAuthcodeExec, targetPath, contractFile)
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

		// Smart Default: Find UUID by workdir if not provided
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

// testContractCmd handles 'gsc contract test [uuid]'
var testContractCmd = &cobra.Command{
	Use:   "test [uuid]",
	Short: "Test a file change against a contract without writing it",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
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

		result, err := contract.TestFile(uuid, contractTestFile, contractTestSanitize)
		if err != nil {
			return err
		}

		fmt.Print(contract.FormatContractTest(result, contractTestFormat))
		return nil
	},
}

// execContractCmd handles 'gsc contract exec'
var execContractCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a command within a contract's security context",
	Long: `Executes a command in the contract's working directory, enforcing 
whitelists and timeouts. Results can be enriched and sent to chat.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		logger.Debug("execContractCmd started", "uuid", contractExecUUID, "cmd", contractExecCmd)

		// 1. Load Contract
		meta, err := contract.GetContract(contractExecUUID)
		if err != nil {
			return fmt.Errorf("failed to load contract: %w", err)
		}

		// 2. Validate Auth Code
		if meta.Authcode != contractExecAuthcode {
			return fmt.Errorf("invalid authorization code")
		}

		// 3. Validate Command against Whitelist
		fields := strings.Fields(contractExecCmd)
		if len(fields) == 0 {
			return fmt.Errorf("no command provided")
		}
		binary := fields[0]

		if !meta.NoWhitelist {
			allowed := false
			for _, w := range meta.Whitelist {
				if w == binary {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("command '%s' is not whitelisted for this contract", binary)
			}
		}

		// 4. Execute Command
		// Pass meta.Workdir to ensure the command runs in the contract's directory
		executor := exec.NewExecutor(contractExecCmd, exec.ExecFlags{
			TimeoutSeconds: meta.ExecTimeout,
		}, meta.Workdir)

		logger.Debug("Calling executor.Run()")
		result, err := executor.Run()
		if err != nil {
			logger.Error("executor.Run() failed", "error", err)
			return err
		}
		logger.Debug("executor.Run() completed", "exitCode", result.ExitCode)

		// 5. Resolve and Apply Formatter
		finalOutput := result.Output
		formatter := formatters.ResolveFormatter(binary)
		if formatter != nil {
			logger.Debug("Applying formatter", "binary", binary)
			// Pre-process to capture context (like file path for cat)
			formatter.PreProcess(fields[1:])
			// Post-process the output
			enriched, err := formatter.PostProcess(finalOutput)
			if err == nil {
				finalOutput = enriched
			}
		}

		// 6. Handle Chat Insertion
		if contractExecChat {
			logger.Debug("Handling chat insertion")
			gscHome, _ := settings.GetGSCHome(false)
			sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
			if err != nil {
				return fmt.Errorf("failed to open chat database: %w", err)
			}
			defer sqliteDB.Close()

			// Find the last message in the chat to ensure correct ordering
			lastMessageID, err := db.GetLastMessageID(sqliteDB, meta.ChatID)
			if err != nil {
				return fmt.Errorf("failed to find last message: %w", err)
			}

			// Format the final Markdown message
			markdown := output.FormatBridgeMarkdown(contractExecCmd, result.Duration, "N/A", "text", finalOutput, result.ExitCode)

			msg := &db.Message{
				Type:       "gsc-cli-output",
				Visibility: "human-public",
				ChatID:     meta.ChatID,
				ParentID:   lastMessageID, // Reply to the last message
				Level:      2,                      // Contract is level 1
				Role:       "assistant",
				RealModel:  sql.NullString{String: settings.RealModelNotes, Valid: true},
				Temperature: sql.NullFloat64{Float64: 0, Valid: true},
				Message:    sql.NullString{String: markdown, Valid: true},
			}

			msgID, err := db.InsertMessage(sqliteDB, msg)
			if err != nil {
				return fmt.Errorf("failed to insert message into chat: %w", err)
			}
			fmt.Printf("[BRIDGE] Output added to chat. Message ID: %d\n", msgID)
		}

		// 7. Propagate Exit Code
		if result.ExitCode != 0 {
			return &cliError{code: result.ExitCode, message: fmt.Sprintf("command failed with exit code %d", result.ExitCode)}
		}

		return nil
	},
}

func init() {
	// Create Flags
	createContractCmd.Flags().StringVar(&contractCode, "code", "", "6-digit handshake code from chat (required)")
	createContractCmd.Flags().StringVar(&contractDescription, "description", "", "Description of the contract's purpose (required)")
	createContractCmd.Flags().StringVar(&contractAuthcode, "authcode", "", "4-digit authorization code (optional, random if not set)")
	createContractCmd.Flags().StringVar(&contractWhitelistFile, "whitelist", "", "Path to a file containing a list of allowed commands (optional)")
	createContractCmd.Flags().BoolVar(&contractNoWhitelist, "no-whitelist", false, "Disable whitelist checks (unrestricted mode)")
	createContractCmd.Flags().IntVar(&contractExecTimeout, "exec-timeout", 60, "Execution timeout in seconds (default 60)")
	createContractCmd.MarkFlagRequired("code")
	createContractCmd.MarkFlagRequired("description")

	// List Flags
	listContractCmd.Flags().StringVar(&contractStatus, "status", "active", "Comma-separated list of statuses (active, expired, cancelled, all)")
	listContractCmd.Flags().StringVar(&contractSort, "sort", "expires", "Sort field (expires, created, description)")
	listContractCmd.Flags().StringVar(&contractOrder, "order", "asc", "Sort order (asc, desc)")
	listContractCmd.Flags().StringVarP(&contractFormat, "format", "f", "human", "Output format (human, json)")
	listContractCmd.Flags().BoolVar(&contractListAll, "all", false, "List all contracts regardless of status (overrides --status)")

	// Renew Flags
	renewContractCmd.Flags().IntVar(&contractRenewHours, "hours", 24, "Number of hours to extend the expiration")

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

	// Info Flags
	infoContractCmd.Flags().StringVarP(&contractInfoFormat, "format", "f", "human", "Output format (human, json)")
	infoContractCmd.Flags().BoolVar(&contractInfoSanitize, "sanitize", false, "Sanitize output (e.g., relative paths)")

	// Test Flags
	testContractCmd.Flags().StringVarP(&contractTestFormat, "format", "f", "human", "Output format (human, json)")
	testContractCmd.Flags().BoolVar(&contractTestSanitize, "sanitize", false, "Sanitize output (e.g., relative paths)")
	testContractCmd.Flags().StringVar(&contractTestFile, "file", "", "Path to the file containing new code to test (required)")
	testContractCmd.MarkFlagRequired("file")

	// Exec Flags
	execContractCmd.Flags().StringVar(&contractExecUUID, "uuid", "", "Contract UUID (required)")
	execContractCmd.Flags().StringVar(&contractExecAuthcode, "authcode", "", "4-digit authorization code (required)")
	execContractCmd.Flags().StringVar(&contractExecCmd, "cmd", "", "Command to execute (required)")
	execContractCmd.Flags().BoolVar(&contractExecChat, "chat", false, "Add output to chat")
	execContractCmd.MarkFlagRequired("uuid")
	execContractCmd.MarkFlagRequired("authcode")
	execContractCmd.MarkFlagRequired("cmd")

	// Add subcommands to base contract command
	contractCmd.AddCommand(createContractCmd)
	contractCmd.AddCommand(statusContractCmd)
	contractCmd.AddCommand(listContractCmd)
	contractCmd.AddCommand(cancelContractCmd)
	contractCmd.AddCommand(renewContractCmd)
	contractCmd.AddCommand(updateFileCmd)
	contractCmd.AddCommand(newFileCmd)
	contractCmd.AddCommand(infoContractCmd)
	contractCmd.AddCommand(testContractCmd)
	contractCmd.AddCommand(execContractCmd)
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
