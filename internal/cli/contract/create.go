/**
 * Component: Contract CLI Create
 * Block-UUID: 94ce3270-81a9-4963-82cf-1f49c1999c2f
 * Parent-UUID: ea53c1b5-0db6-4644-a4b3-3c529bf8bda0
 * Version: 1.1.0
 * Description: CLI command for creating new traceability contracts with interactive workspace configuration.
 * Language: Go
 * Created-at: 2026-03-10T04:35:57.944Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0), GLM-4.7 (v1.1.0)
 */


package contract

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

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
		// If the terminal preference is missing, prompt the user.
		// This ensures the "Review & Save" feature is configured.
		if contractPreferredTerminal == "" {
			fmt.Println("\n--- Workspace Configuration ---")
			fmt.Println("Let's configure your preferred terminal for the AI review workflow.")
			
			// 1. Prepare Options (Dynamic based on OS)
			terminalOptions := getSortedKeys(settings.DefaultTerminalTemplates)

			// 2. Prompt for Terminal
			if contractPreferredTerminal == "" {
				prompt := &survey.Select{
					Message: "Choose your preferred terminal:",
					Options: terminalOptions,
				}
				if err := survey.AskOne(prompt, &contractPreferredTerminal); err != nil {
					return fmt.Errorf("prompt failed: %w", err)
				}
			}
			
			fmt.Println("-----------------------------\n")
		}

		// Validate Preferred Terminal
		if contractPreferredTerminal != "" {
			if _, exists := settings.DefaultTerminalTemplates[contractPreferredTerminal]; !exists {
				return fmt.Errorf("unsupported terminal '%s'. Available terminals: %s", 
					contractPreferredTerminal, 
					strings.Join(getMapKeys(settings.DefaultTerminalTemplates), ", "))
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
			contractPreferredTerminal,
			"", // PreferredEditor (removed)
			"", // PreferredReview (removed)
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

func init() {
	// Create Flags
	createContractCmd.Flags().StringVar(&contractCode, "code", "", "6-digit handshake code from chat (required)")
	createContractCmd.Flags().StringVar(&contractDescription, "description", "", "Description of the contract's purpose (required)")
	createContractCmd.Flags().StringVar(&contractAuthcode, "authcode", "", "4-digit authorization code (optional, random if not set)")
	createContractCmd.Flags().StringVar(&contractWhitelistFile, "whitelist", "", "Path to a file containing a list of allowed commands (optional)")
	createContractCmd.Flags().BoolVar(&contractNoWhitelist, "no-whitelist", false, "Disable whitelist checks (unrestricted mode)")
	createContractCmd.Flags().IntVar(&contractExecTimeout, "exec-timeout", 60, "Execution timeout in seconds (default 60)")
	createContractCmd.Flags().StringVar(&contractPreferredTerminal, "terminal", "", fmt.Sprintf("Preferred terminal for project access (Available: %s)", strings.Join(getMapKeys(settings.DefaultTerminalTemplates), ", ")))
	createContractCmd.MarkFlagRequired("code")
	createContractCmd.MarkFlagRequired("description")

	// Register Subcommand
	contractCmd.AddCommand(createContractCmd)
}
