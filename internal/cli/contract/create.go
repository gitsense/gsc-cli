/*
 * Component: Contract CLI Create
 * Block-UUID: ea53c1b5-0db6-4644-a4b3-3c529bf8bda0
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command for creating new traceability contracts with interactive workspace configuration.
 * Language: Go
 * Created-at: 2026-03-08T00:21:39.790Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0)
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

	// Register Subcommand
	contractCmd.AddCommand(createContractCmd)
}
