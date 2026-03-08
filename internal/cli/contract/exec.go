/*
 * Component: Contract CLI Execution
 * Block-UUID: 91152f9e-1154-4ca4-9686-b126479986c4
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI commands for executing commands within a contract context and launching workspace tools.
 * Language: Go
 * Created-at: 2026-03-08T00:23:01.234Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0)
 */


package contract

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/bridge/formatters"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/exec"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/internal/output"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

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
		}, meta.Workdir, nil)

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
			Hash:         contractLaunchHash,
			Position:     contractLaunchPosition,
			ActiveChatID: contractLaunchActiveChatID,
		}

		result, err := contract.HandleLaunch(req)
		if err != nil {
			// Return a JSON error response so the Node.js service can parse it
			errorResult := contract.LaunchResult{
				Success: false,
				Message: err.Error(),
				Alias:   req.Alias,
			}
			if contractLaunchFormat == "json" {
				output.FormatJSON(errorResult)
			}
			return err
		}
		
		if contractLaunchFormat == "json" {
			output.FormatJSON(result)
		}
		return nil
	},
}

func init() {
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
	launchContractCmd.Flags().StringVarP(&contractLaunchFormat, "format", "f", "human", "Output format (human, json)")
	launchContractCmd.Flags().StringVar(&contractLaunchHash, "hash", "", "Message hash for shadow workspace resolution")
	launchContractCmd.Flags().IntVar(&contractLaunchPosition, "position", -1, "Code block position for directory resolution")
	launchContractCmd.Flags().Int64Var(&contractLaunchActiveChatID, "active-chat-id", 0, "Active chat ID for environment context")

	// Register Subcommands
	contractCmd.AddCommand(execContractCmd)
	contractCmd.AddCommand(launchContractCmd)
}
