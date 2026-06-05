/**
 * Component: Contract CLI Create
 * Block-UUID: e05ffaec-bf52-439b-bd13-feedf8c8d2f6
 * Parent-UUID: 8c781331-aa47-4f7b-aa16-8cfb366f8762
 * Version: 1.2.0
 * Description: Auto-select default terminal based on platform instead of prompting user interactively.
 * Language: Go
 * Created-at: 2026-03-10T04:35:57.944Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.29.1), Gemini 3 Flash (v1.30.0), GLM-4.7 (v1.31.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.1.1), GLM-4.7 (v1.1.2), GLM-4.7 (v1.2.0)
 */


package contract

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// getDefaultTerminalForPlatform returns the most commonly available terminal for the current platform.
func getDefaultTerminalForPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "terminal.app"
	case "linux":
		return "gnome-terminal"
	case "windows":
		return "wt"
	default:
		// Fallback for unknown platforms
		return "bash"
	}
}

// createContractCmd handles 'gsc app contract create'
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
		// Terminal Selection
		// ==========================================
		// If the terminal preference is missing, auto-select the default for the platform.
		// This ensures the "Review & Save" feature is configured without requiring user interaction.
		if contractPreferredTerminal == "" {
			contractPreferredTerminal = getDefaultTerminalForPlatform()
			fmt.Printf("Using default terminal: %s\n", contractPreferredTerminal)
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
			"",                        // PreferredEditor (empty)
			contractPreferredTerminal, // PreferredTerminal (correct position)
			"",                        // PreferredReview (empty)
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
