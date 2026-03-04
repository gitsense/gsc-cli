/**
 * Component: Contract CLI Commands
 * Block-UUID: 07aff45c-e140-408c-8516-fe823c4b1c2e
 * Parent-UUID: 8f9c8d2e-3f4a-4b5c-9d6e-0f1a2b3c4d5e
 * Version: 1.26.0
 * Description: Refactored dump command to use subcommands (tree, merged, mapped) for better discoverability and scoped flags. Removed --type flag in favor of positional subcommands.
 * Language: Go
 * Created-at: 2026-03-04T04:40:27.188Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.23.2), Gemini 3 Flash (v1.24.0), Gemini 3 Flash (v1.25.0), GLM-4.7 (v1.26.0)
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
	"github.com/AlecAivazis/survey/v2"
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
	contractPreferredEditor   string
	contractPreferredTerminal string
	contractPreferredReview   string

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

	// Launch flags
	contractLaunchAlias           string
	contractLaunchBlockUUID        string
	contractLaunchParentUUID       string
	contractLaunchAction           string
	contractLaunchAppOverride      string
	contractLaunchCmd              string
	contractLaunchList             bool

	// Dump flags (Shared)
	contractDumpUUID   string
	contractDumpOutput string
	contractDumpIncludeSystem bool
	contractDumpDebugPatch bool
	contractDumpRaw    bool
	contractDumpFormat    string

	// Dump flags (Merged specific)
	contractDumpSort   string

	// Dump flags (Mapped specific)
	contractDumpMessageID int64
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
		workdir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		if contractAuthcode == "" {
			n, err := rand.Int(rand.Reader, big.NewInt(10000))
			if err != nil {
				return fmt.Errorf("failed to generate random authcode: %w", err)
			}
			contractAuthcode = fmt.Sprintf("%04d", n.Int64())
		}

		// ==========================================
		// Interactive Setup Wizard
		// ==========================================
		// If any of the workspace preferences are missing, prompt the user.
		// This ensures the "Review & Save" feature is configured.
		if contractPreferredEditor == "" || contractPreferredTerminal == "" || contractPreferredReview == "" {
			fmt.Println("\n--- Workspace Configuration ---")
			fmt.Println("Let's configure your preferred tools for the AI review workflow.")
			
			// 1. Prepare Options (Dynamic based on OS)
			editorOptions := getSortedKeys(settings.DefaultEditorTemplates)
			terminalOptions := getSortedKeys(settings.DefaultTerminalTemplates)

			// 2. Prompt for Editor
			if contractPreferredEditor == "" {
				prompt := &survey.Select{
					Message: "Choose your preferred code editor:",
					Options: editorOptions,
				}
				if err := survey.AskOne(prompt, &contractPreferredEditor); err != nil {
					return fmt.Errorf("prompt failed: %w", err)
				}
			}

			// 3. Prompt for Terminal
			if contractPreferredTerminal == "" {
				prompt := &survey.Select{
					Message: "Choose your preferred terminal:",
					Options: terminalOptions,
				}
				if err := survey.AskOne(prompt, &contractPreferredTerminal); err != nil {
					return fmt.Errorf("prompt failed: %w", err)
				}
			}

			// 4. Prompt for Review Tool
			// Smart Default: Use the selected Editor if it's a valid review tool
			if contractPreferredReview == "" {
				defaultReview := contractPreferredEditor
				// Ensure the default is actually in the list of valid editors
				if _, exists := settings.DefaultEditorTemplates[defaultReview]; !exists {
					defaultReview = "" // Fallback to no default if invalid
				}

				prompt := &survey.Select{
					Message: "Choose your preferred tool for reviewing AI code:",
					Options: editorOptions, // Review tools are usually editors
					Default: defaultReview,
				}
				if err := survey.AskOne(prompt, &contractPreferredReview); err != nil {
					return fmt.Errorf("prompt failed: %w", err)
				}
			}
			
			fmt.Println("-----------------------------\n")
		}

		// Validate Preferred Editor
		if contractPreferredEditor != "" {
			if _, exists := settings.DefaultEditorTemplates[contractPreferredEditor]; !exists {
				return fmt.Errorf("unsupported editor '%s'. Available editors: %s", 
					contractPreferredEditor, 
					strings.Join(getMapKeys(settings.DefaultEditorTemplates), ", "))
			}
		}

		// Validate Preferred Terminal
		if contractPreferredTerminal != "" {
			if _, exists := settings.DefaultTerminalTemplates[contractPreferredTerminal]; !exists {
				return fmt.Errorf("unsupported terminal '%s'. Available terminals: %s", 
					contractPreferredTerminal, 
					strings.Join(getMapKeys(settings.DefaultTerminalTemplates), ", "))
			}
		}

		// Validate Preferred Review
		if contractPreferredReview != "" {
			if _, exists := settings.DefaultEditorTemplates[contractPreferredReview]; !exists {
				return fmt.Errorf("unsupported review tool '%s'. Available tools: %s", 
					contractPreferredReview, 
					strings.Join(getMapKeys(settings.DefaultEditorTemplates), ", "))
			}
		}

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

		// Call manager with new security and workspace parameters
		meta, err := contract.CreateContract(
			contractCode, 
			contractDescription, 
			contractAuthcode, 
			workdir,
			whitelist,
			contractNoWhitelist,
			contractExecTimeout,
			contractPreferredEditor,
			contractPreferredTerminal,
			contractPreferredReview,
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
		workdir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		meta, err := contract.GetContractByWorkdir(workdir)
		if err != nil {
			if strings.Contains(err.Error(), "no active contract") {
				fmt.Println("No active contract found for this repository.")
				fmt.Println("")
				fmt.Println("To create a new contract, run:")
				fmt.Println("  gsc contract create --code <6-digit-code> --description \"Purpose of contract\"")
				return nil
			}
			return err
		}

		display := output.ContractDisplay{
			UUID:        meta.UUID,
			Description: meta.Description,
			Workdir:     meta.Workdir,
			Status:      string(meta.Status),
			ExpiresAt:   meta.ExpiresAt.Format(time.RFC3339),
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
				UUID:        c.UUID,
				Description: c.Description,
				Workdir:     c.Workdir,
				Status:      string(c.Status),
				ExpiresAt:   c.ExpiresAt.Format(time.RFC3339),
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

// updateFileCmd handles 'gsc contract update-file'
var updateFileCmd = &cobra.Command{
	Use:   "update-file",
	Short: "Update an existing traceable file using a contract",
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
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		logger.Debug("execContractCmd started", "uuid", contractExecUUID, "cmd", contractExecCmd)

		meta, err := contract.GetContract(contractExecUUID)
		if err != nil {
			return fmt.Errorf("failed to load contract: %w", err)
		}

		if meta.Authcode != contractExecAuthcode {
			return fmt.Errorf("invalid authorization code")
		}

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

		executor := exec.NewExecutor(contractExecCmd, exec.ExecFlags{
			TimeoutSeconds: meta.ExecTimeout,
		}, meta.Workdir)

		result, err := executor.Run()
		if err != nil {
			return err
		}

		finalOutput := result.Output
		formatter := formatters.ResolveFormatter(binary)
		if formatter != nil {
			formatter.PreProcess(fields[1:])
			enriched, err := formatter.PostProcess(finalOutput)
			if err == nil {
				finalOutput = enriched
			}
		}

		if contractExecChat {
			gscHome, _ := settings.GetGSCHome(false)
			sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
			if err != nil {
				return fmt.Errorf("failed to open chat database: %w", err)
			}
			defer sqliteDB.Close()

			lastMessageID, err := db.GetLastMessageID(sqliteDB, meta.ChatID)
			if err != nil {
				return fmt.Errorf("failed to find last message: %w", err)
			}

			markdown := output.FormatBridgeMarkdown(contractExecCmd, result.Duration, "N/A", "text", finalOutput, result.ExitCode)

			msg := &db.Message{
				Type:       "gsc-cli-output",
				Visibility: "public",
				ChatID:     meta.ChatID,
				ParentID:   lastMessageID,
				Level:      2,
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

		if result.ExitCode != 0 {
			return &cliError{code: result.ExitCode, message: fmt.Sprintf("command failed with exit code %d", result.ExitCode)}
		}

		return nil
	},
}

// launchContractCmd handles 'gsc contract launch'
var launchContractCmd = &cobra.Command{
	Use:   "launch [alias]",
	Short: "Launch workspace tools (terminal, editor, review)",
	Long: `Processes requests from the Web UI to perform context-aware actions 
like launching a terminal in the project root or staging AI code for review 
in a proper editor.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// Handle --list flag
		if contractLaunchList {
			caps := contract.GetLaunchCapabilities()
			output.FormatJSON(caps)
			return nil
		}

		// Validate required flags if not listing
		if contractUUID == "" {
			return fmt.Errorf("--uuid is required")
		}
		if contractAuthcodeExec == "" {
			return fmt.Errorf("--authcode is required")
		}
		if contractLaunchAlias == "" {
			return fmt.Errorf("--alias is required")
		}

		req := contract.LaunchRequest{
			ContractUUID: contractUUID,
			Authcode:     contractAuthcodeExec,
			Alias:        contractLaunchAlias,
			BlockUUID:    contractLaunchBlockUUID,
			ParentUUID:   contractLaunchParentUUID,
			Action:       contractLaunchAction,
			AppOverride:  contractLaunchAppOverride,
			Cmd:          contractLaunchCmd,
		}

		result, err := contract.HandleLaunch(req)
		if err != nil {
			// Return a JSON error response so the Node.js service can parse it
			errorResult := contract.LaunchResult{
				Success: false,
				Message: err.Error(),
				Alias:   req.Alias,
			}
			output.FormatJSON(errorResult)
			return err
		}
		
		output.FormatJSON(result)
		return nil
	},
}

// ==========================================
// DUMP COMMANDS (Refactored to Subcommands)
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

		// 2. Resolve Output Directory
		outputDir := contractDumpOutput
		if outputDir == "" {
			gscHome, _ := settings.GetGSCHome(false)
			outputDir = filepath.Join(gscHome, settings.DumpsRelPath, uuid)
		}

		// 3. Select Strategy
		writer := &contract.TreeWriter{}

		// 4. Execute
		logger.Info("Generating conversational dump...", "type", "tree", "output", outputDir, "trim", !contractDumpRaw)
		
		_, err = contract.ExecuteDump(uuid, writer, outputDir, contractDumpIncludeSystem, !contractDumpRaw, "tree", "", contractDumpDebugPatch, 0)
		
		// 5. Handle Output Format
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

		// 2. Resolve Output Directory
		outputDir := contractDumpOutput
		if outputDir == "" {
			gscHome, _ := settings.GetGSCHome(false)
			outputDir = filepath.Join(gscHome, settings.DumpsRelPath, uuid)
		}

		// 3. Select Strategy
		writer := &contract.MergedWriter{}

		// 4. Execute
		logger.Info("Generating merged dump...", "type", "merged", "sort", contractDumpSort, "output", outputDir, "trim", !contractDumpRaw)
		
		_, err = contract.ExecuteDump(uuid, writer, outputDir, contractDumpIncludeSystem, !contractDumpRaw, "merged", contractDumpSort, contractDumpDebugPatch, 0)
		
		// 5. Handle Output Format
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

		// 2. Resolve Output Directory
		outputDir := contractDumpOutput
		if outputDir == "" {
			gscHome, _ := settings.GetGSCHome(false)
			outputDir = filepath.Join(gscHome, settings.DumpsRelPath, uuid)
		}

		// 3. Select Strategy
		writer := &contract.MappedWriter{}

		// 4. Execute
		logger.Info("Generating mapped dump...", "type", "mapped", "output", outputDir, "trim", !contractDumpRaw)
		
		result, err := contract.ExecuteDump(uuid, writer, outputDir, contractDumpIncludeSystem, !contractDumpRaw, "mapped", "", contractDumpDebugPatch, contractDumpMessageID)
		
		// 5. Handle Output Format
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
		
		// Human Mode
		if err != nil {
			return err
		}
		
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

// resolveDumpUUID is a helper to resolve the contract UUID from flag or workdir.
func resolveDumpUUID(uuid string) (string, error) {
	if uuid != "" {
		return uuid, nil
	}
	return findContractUUIDByWorkdir()
}

func init() {
	// Create Flags
	createContractCmd.Flags().StringVar(&contractCode, "code", "", "6-digit handshake code from chat (required)")
	createContractCmd.Flags().StringVar(&contractDescription, "description", "", "Description of the contract's purpose (required)")
	createContractCmd.Flags().StringVar(&contractAuthcode, "authcode", "", "4-digit authorization code (optional, random if not set)")
	createContractCmd.Flags().StringVar(&contractWhitelistFile, "whitelist", "", "Path to a file containing a list of allowed commands (optional)")
	createContractCmd.Flags().BoolVar(&contractNoWhitelist, "no-whitelist", false, "Disable whitelist checks (unrestricted mode)")
	createContractCmd.Flags().IntVar(&contractExecTimeout, "exec-timeout", 60, "Execution timeout in seconds (default 60)")
	createContractCmd.Flags().StringVar(&contractPreferredEditor, "editor", "", fmt.Sprintf("Preferred editor for code review (Available: %s)", strings.Join(getMapKeys(settings.DefaultEditorTemplates), ", ")))
	createContractCmd.Flags().StringVar(&contractPreferredTerminal, "terminal", "", fmt.Sprintf("Preferred terminal for project access (Available: %s)", strings.Join(getMapKeys(settings.DefaultTerminalTemplates), ", ")))
	createContractCmd.Flags().StringVar(&contractPreferredReview, "review", "", fmt.Sprintf("Preferred tool for code review (Available: %s)", strings.Join(getMapKeys(settings.DefaultEditorTemplates), ", ")))
	createContractCmd.MarkFlagRequired("code")
	createContractCmd.MarkFlagRequired("description")

	// List Flags
	listContractCmd.Flags().StringVar(&contractStatus, "status", "active", "Comma-separated list of statuses (active, expired, cancelled, done, all)")
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

	// Launch Flags
	launchContractCmd.Flags().StringVar(&contractUUID, "uuid", "", "Contract UUID (required)")
	launchContractCmd.Flags().StringVar(&contractAuthcodeExec, "authcode", "", "4-digit authorization code (required)")
	launchContractCmd.Flags().StringVar(&contractLaunchAlias, "alias", "", "Action alias: review, terminal, editor, exec (required)")
	launchContractCmd.Flags().StringVar(&contractLaunchBlockUUID, "block-uuid", "", "UUID of the AI code block")
	launchContractCmd.Flags().StringVar(&contractLaunchParentUUID, "parent-uuid", "", "UUID of the parent code block")
	launchContractCmd.Flags().StringVar(&contractLaunchAction, "action", "source", "Review action: source or patch")
	launchContractCmd.Flags().StringVar(&contractLaunchAppOverride, "app-override", "", "Override contract app (e.g., zed, iterm2)")
	launchContractCmd.Flags().StringVar(&contractLaunchCmd, "cmd", "", "Raw command for exec alias")
	launchContractCmd.Flags().BoolVar(&contractLaunchList, "list", false, "List available aliases, apps, and commands")

	// ==========================================
	// Dump Flags (Refactored)
	// ==========================================
	
	// Parent Flags (Shared by all dump types)
	dumpContractCmd.PersistentFlags().StringVar(&contractDumpUUID, "uuid", "", "Contract UUID (optional if in workdir)")
	dumpContractCmd.PersistentFlags().StringVarP(&contractDumpOutput, "output", "o", "", "Output directory (default: ~/.gitsense/dumps/<uuid>)")
	dumpContractCmd.PersistentFlags().BoolVar(&contractDumpIncludeSystem, "include-system", false, "Include the system message in the dump (default: false)")
	dumpContractCmd.PersistentFlags().BoolVar(&contractDumpDebugPatch, "debug-patch", false, "Enable patch debugging (persists source and diff artifacts on failure)")
	dumpContractCmd.PersistentFlags().BoolVar(&contractDumpRaw, "raw", false, "Disable smart trimming (preserve exact LLM output)")
	dumpContractCmd.PersistentFlags().StringVarP(&contractDumpFormat, "format", "f", "human", "Output format: human or json (default: human)")

	// Merged Specific Flags
	dumpMergedCmd.Flags().StringVar(&contractDumpSort, "sort", "recency", "Sort mode for merged type: recency, popularity, chronological")

	// Mapped Specific Flags
	dumpMappedCmd.Flags().Int64Var(&contractDumpMessageID, "message-id", 0, "Filter dump to a specific message ID")

	// Register Subcommands
	dumpContractCmd.AddCommand(dumpTreeCmd)
	dumpContractCmd.AddCommand(dumpMergedCmd)
	dumpContractCmd.AddCommand(dumpMappedCmd)

	// Add subcommands to base contract command
	contractCmd.AddCommand(createContractCmd)
	contractCmd.AddCommand(statusContractCmd)
	contractCmd.AddCommand(listContractCmd)
	contractCmd.AddCommand(cancelContractCmd)
	contractCmd.AddCommand(deleteContractCmd)
	contractCmd.AddCommand(renewContractCmd)
	contractCmd.AddCommand(completeContractCmd)
	contractCmd.AddCommand(updateFileCmd)
	contractCmd.AddCommand(newFileCmd)
	contractCmd.AddCommand(infoContractCmd)
	contractCmd.AddCommand(testContractCmd)
	contractCmd.AddCommand(execContractCmd)
	contractCmd.AddCommand(launchContractCmd)
	contractCmd.AddCommand(dumpContractCmd)
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
